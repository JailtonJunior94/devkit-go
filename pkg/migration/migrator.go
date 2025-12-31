package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrator manages database migrations using golang-migrate.
// It provides a safe, resilient, and intuitive API for running migrations
// in applications, CLI tools, init containers, and Kubernetes jobs.
type Migrator struct {
	config         Config
	migrate        *migrate.Migrate
	driverStrategy DriverStrategy
	closeOnce      sync.Once
	closed         bool
	closedMu       sync.RWMutex
	logger         Logger
	databaseName   string
}

// New creates a new Migrator instance with the given options.
// It validates the configuration and establishes a connection to the migration system.
//
// Example:
//
//	migrator, err := migration.New(
//	    migration.WithDriver(migration.DriverPostgres),
//	    migration.WithDSN("postgres://user:pass@localhost:5432/mydb?sslmode=disable"),
//	    migration.WithSource("file://migrations"),
//	    migration.WithLogger(logger),
//	)
//	if err != nil {
//	    return err
//	}
//	defer migrator.Close()
func New(opts ...Option) (*Migrator, error) {
	config := DefaultConfig()

	for _, opt := range opts {
		opt(&config)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid migration configuration: %w", err)
	}

	ctx := context.Background()
	logger := config.Logger
	databaseName := config.GetDatabaseName()

	logger.Info(ctx, "initializing migrator",
		String("driver", config.Driver.String()),
		String("database", databaseName),
		String("source", config.Source),
	)

	strategy, err := GetDriverStrategy(config.Driver)
	if err != nil {
		logger.Error(ctx, "failed to get driver strategy", Error(err))
		return nil, err
	}

	if err := strategy.Validate(config); err != nil {
		logger.Error(ctx, "driver-specific validation failed", Error(err))
		return nil, fmt.Errorf("driver validation failed: %w", err)
	}

	params := DatabaseParams{
		LockTimeout:           config.LockTimeout,
		StatementTimeout:      config.StatementTimeout,
		MultiStatementEnabled: config.MultiStatementEnabled,
		MultiStatementMaxSize: config.MultiStatementMaxSize,
		PreferSimpleProtocol:  config.PreferSimpleProtocol,
	}

	databaseURL, err := strategy.BuildDatabaseURL(config.DSN, params)
	if err != nil {
		logger.Error(ctx, "failed to build database URL", Error(err))
		return nil, fmt.Errorf("failed to build database URL: %w", err)
	}

	m, err := migrate.New(config.Source, databaseURL)
	if err != nil {
		logger.Error(ctx, "failed to initialize migrate instance",
			Error(err),
			String("source", config.Source),
		)
		return nil, fmt.Errorf("failed to initialize migrate instance: %w", err)
	}

	migrator := &Migrator{
		config:         config,
		migrate:        m,
		driverStrategy: strategy,
		closed:         false,
		logger:         logger,
		databaseName:   databaseName,
	}

	logger.Info(ctx, "migrator initialized successfully",
		String("database", databaseName),
	)

	return migrator, nil
}

// Up runs all available migrations in ascending order.
// Returns ErrNoChanges if the database is already up to date.
// This is safe to run multiple times - it's idempotent.
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.checkClosed(); err != nil {
		return err
	}

	m.logger.Info(ctx, "starting migration UP",
		String("database", m.databaseName),
	)

	start := time.Now()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- m.migrate.Up()
	}()

	select {
	case <-ctxWithTimeout.Done():
		m.logger.Error(ctx, "migration UP timed out",
			String("timeout", m.config.Timeout.String()),
			String("database", m.databaseName),
		)
		return fmt.Errorf("migration UP timed out after %v: %w", m.config.Timeout, ctxWithTimeout.Err())

	case err := <-errChan:
		duration := time.Since(start)

		if err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				m.logger.Info(ctx, "no migrations to apply - database is up to date",
					String("database", m.databaseName),
					String("duration", duration.String()),
				)
				return nil
			}

			if strings.Contains(err.Error(), "dirty") {
				m.logger.Error(ctx, "database is in dirty state - manual intervention required",
					Error(err),
					String("database", m.databaseName),
					String("duration", duration.String()),
				)
				return ErrDirtyDatabase
			}

			version, dirty, vErr := m.migrate.Version()
			m.logger.Error(ctx, "migration UP failed",
				Error(err),
				String("database", m.databaseName),
				Uint("version", version),
				Bool("dirty", dirty),
				String("duration", duration.String()),
			)

			if vErr == nil {
				return NewMigrationError("up", m.config.Driver, version, err)
			}
			return NewMigrationError("up", m.config.Driver, 0, err)
		}

		version, dirty, _ := m.migrate.Version()
		m.logger.Info(ctx, "migration UP completed successfully",
			String("database", m.databaseName),
			Uint("current_version", version),
			Bool("dirty", dirty),
			String("duration", duration.String()),
		)

		return nil
	}
}

// Down rolls back all migrations.
// WARNING: This will remove all schema changes. Use with extreme caution.
// Returns ErrNoChanges if there are no migrations to roll back.
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.checkClosed(); err != nil {
		return err
	}

	m.logger.Warn(ctx, "starting migration DOWN - this will roll back all migrations",
		String("database", m.databaseName),
	)

	start := time.Now()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- m.migrate.Down()
	}()

	select {
	case <-ctxWithTimeout.Done():
		m.logger.Error(ctx, "migration DOWN timed out",
			String("timeout", m.config.Timeout.String()),
			String("database", m.databaseName),
		)
		return fmt.Errorf("migration DOWN timed out after %v: %w", m.config.Timeout, ctxWithTimeout.Err())

	case err := <-errChan:
		duration := time.Since(start)

		if err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				m.logger.Info(ctx, "no migrations to roll back",
					String("database", m.databaseName),
					String("duration", duration.String()),
				)
				return nil
			}

			if strings.Contains(err.Error(), "dirty") {
				m.logger.Error(ctx, "database is in dirty state - manual intervention required",
					Error(err),
					String("database", m.databaseName),
					String("duration", duration.String()),
				)
				return ErrDirtyDatabase
			}

			version, dirty, vErr := m.migrate.Version()
			m.logger.Error(ctx, "migration DOWN failed",
				Error(err),
				String("database", m.databaseName),
				Uint("version", version),
				Bool("dirty", dirty),
				String("duration", duration.String()),
			)

			if vErr == nil {
				return NewMigrationError("down", m.config.Driver, version, err)
			}
			return NewMigrationError("down", m.config.Driver, 0, err)
		}

		m.logger.Info(ctx, "migration DOWN completed successfully",
			String("database", m.databaseName),
			String("duration", duration.String()),
		)

		return nil
	}
}

// Steps runs a specific number of migrations.
// Positive n migrates up, negative n migrates down.
// Returns ErrNoChanges if no migrations were applied.
func (m *Migrator) Steps(ctx context.Context, n int) error {
	if err := m.checkClosed(); err != nil {
		return err
	}

	direction := "up"
	if n < 0 {
		direction = "down"
	}

	m.logger.Info(ctx, "starting migration steps",
		String("database", m.databaseName),
		Int("steps", n),
		String("direction", direction),
	)

	start := time.Now()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- m.migrate.Steps(n)
	}()

	select {
	case <-ctxWithTimeout.Done():
		m.logger.Error(ctx, "migration steps timed out",
			String("timeout", m.config.Timeout.String()),
			String("database", m.databaseName),
			Int("steps", n),
		)
		return fmt.Errorf("migration steps timed out after %v: %w", m.config.Timeout, ctxWithTimeout.Err())

	case err := <-errChan:
		duration := time.Since(start)

		if err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				m.logger.Info(ctx, "no migrations to apply",
					String("database", m.databaseName),
					Int("steps", n),
					String("duration", duration.String()),
				)
				return nil
			}

			if strings.Contains(err.Error(), "dirty") {
				m.logger.Error(ctx, "database is in dirty state - manual intervention required",
					Error(err),
					String("database", m.databaseName),
					String("duration", duration.String()),
				)
				return ErrDirtyDatabase
			}

			version, dirty, vErr := m.migrate.Version()
			m.logger.Error(ctx, "migration steps failed",
				Error(err),
				String("database", m.databaseName),
				Int("steps", n),
				Uint("version", version),
				Bool("dirty", dirty),
				String("duration", duration.String()),
			)

			if vErr == nil {
				return NewMigrationError("steps", m.config.Driver, version, err)
			}
			return NewMigrationError("steps", m.config.Driver, 0, err)
		}

		version, dirty, _ := m.migrate.Version()
		m.logger.Info(ctx, "migration steps completed successfully",
			String("database", m.databaseName),
			Int("steps", n),
			Uint("current_version", version),
			Bool("dirty", dirty),
			String("duration", duration.String()),
		)

		return nil
	}
}

// Version returns the current migration version and dirty state.
// Returns (0, false, nil) if no migrations have been applied yet.
func (m *Migrator) Version(ctx context.Context) (version uint, dirty bool, err error) {
	if err := m.checkClosed(); err != nil {
		return 0, false, err
	}

	version, dirty, err = m.migrate.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			m.logger.Info(ctx, "no migrations applied yet",
				String("database", m.databaseName),
			)
			return 0, false, nil
		}

		m.logger.Error(ctx, "failed to get migration version",
			Error(err),
			String("database", m.databaseName),
		)
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	m.logger.Debug(ctx, "current migration version",
		String("database", m.databaseName),
		Uint("version", version),
		Bool("dirty", dirty),
	)

	return version, dirty, nil
}

// Close releases all resources held by the Migrator.
// It's safe to call multiple times.
// After calling Close, the Migrator cannot be used anymore.
func (m *Migrator) Close() error {
	var closeErr error

	m.closeOnce.Do(func() {
		ctx := context.Background()

		m.logger.Info(ctx, "closing migrator",
			String("database", m.databaseName),
		)

		m.closedMu.Lock()
		m.closed = true
		m.closedMu.Unlock()

		if m.migrate != nil {
			srcErr, dbErr := m.migrate.Close()
			if srcErr != nil || dbErr != nil {
				closeErr = errors.Join(srcErr, dbErr)
				m.logger.Error(ctx, "error closing migrator resources",
					Error(closeErr),
					String("database", m.databaseName),
				)
			} else {
				m.logger.Info(ctx, "migrator closed successfully",
					String("database", m.databaseName),
				)
			}
		}
	})

	return closeErr
}

// checkClosed returns an error if the migrator has been closed.
func (m *Migrator) checkClosed() error {
	m.closedMu.RLock()
	defer m.closedMu.RUnlock()

	if m.closed {
		return ErrAlreadyClosed
	}
	return nil
}
