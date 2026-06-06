package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	migratelib "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"

	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/sqlserver"

	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Migrator interface {
	Up(ctx context.Context) error
	Down(ctx context.Context, steps int) error
	Force(ctx context.Context, version int) error
	Version(ctx context.Context) (version uint, dirty bool, err error)
}

type migrator struct {
	open func() (*migratelib.Migrate, error)
	opts options
	obs  observability.Observability
	drv  database.Driver
}

func New(mgr manager.Manager, src Source, opts ...Option) (Migrator, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	if o.dsn == "" {
		return nil, fmt.Errorf("%w: WithDSN is required", database.ErrInvalidConfig)
	}

	dbURL := normalizeDSN(o.dsn)

	open := func() (*migratelib.Migrate, error) {
		srcDrv, srcURL, err := resolveSource(src)
		if err != nil {
			return nil, err
		}

		var m *migratelib.Migrate
		if srcDrv != nil {
			m, err = migratelib.NewWithSourceInstance("iofs", srcDrv, dbURL)
		} else {
			m, err = migratelib.New(srcURL, dbURL)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
		}
		if o.timeout > 0 {
			m.LockTimeout = o.timeout
		}
		return m, nil
	}

	probe, err := open()
	if err != nil {
		return nil, err
	}
	_, _ = probe.Close()

	return &migrator{
		open: open,
		opts: o,
		obs:  o.observability,
		drv:  mgr.Driver(),
	}, nil
}

func normalizeDSN(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "postgres://"):
		return "pgx5" + dsn[len("postgres"):]
	case strings.HasPrefix(dsn, "postgresql://"):
		return "pgx5" + dsn[len("postgresql"):]
	default:
		return dsn
	}
}

func (m *migrator) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if m.opts.timeout > 0 {
		return context.WithTimeout(ctx, m.opts.timeout)
	}
	return ctx, func() {}
}

func (m *migrator) Up(ctx context.Context) error {
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()

	spanName := fmt.Sprintf("db.%s.migration.up", m.drv)
	_, span := m.obs.Tracer().Start(ctx, spanName)
	defer span.End()

	if err := m.run(ctx, func(mm *migratelib.Migrate) error { return mm.Up() }); err != nil {
		mapped := mapError(err)
		if !errors.Is(mapped, ErrNoChange) {
			span.RecordError(mapped)
			span.SetStatus(observability.StatusCodeError, mapped.Error())
		}
		return mapped
	}
	return nil
}

func (m *migrator) Down(ctx context.Context, steps int) error {
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()

	spanName := fmt.Sprintf("db.%s.migration.down", m.drv)
	_, span := m.obs.Tracer().Start(ctx, spanName)
	defer span.End()

	if err := m.run(ctx, func(mm *migratelib.Migrate) error { return mm.Steps(-steps) }); err != nil {
		mapped := mapError(err)
		span.RecordError(mapped)
		span.SetStatus(observability.StatusCodeError, mapped.Error())
		return mapped
	}
	return nil
}

func (m *migrator) Force(ctx context.Context, version int) error {
	ctx, cancel := m.withTimeout(ctx)
	defer cancel()

	spanName := fmt.Sprintf("db.%s.migration.force", m.drv)
	_, span := m.obs.Tracer().Start(ctx, spanName)
	defer span.End()

	if err := m.run(ctx, func(mm *migratelib.Migrate) error { return mm.Force(version) }); err != nil {
		mapped := mapError(err)
		span.RecordError(mapped)
		span.SetStatus(observability.StatusCodeError, mapped.Error())
		return mapped
	}
	return nil
}

func (m *migrator) Version(_ context.Context) (uint, bool, error) {
	mm, err := m.open()
	if err != nil {
		return 0, false, err
	}
	defer func() {
		_, _ = mm.Close()
	}()

	version, dirty, err := mm.Version()
	if err != nil {
		return 0, false, mapError(err)
	}
	return version, dirty, nil
}

func (m *migrator) run(ctx context.Context, fn func(*migratelib.Migrate) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, err)
	}

	mm, err := m.open()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- fn(mm)
	}()

	select {
	case err := <-done:
		_, _ = mm.Close()
		return err
	case <-ctx.Done():
		select {
		case mm.GracefulStop <- true:
		default:
		}
		go func() {
			_, _ = mm.Close()
		}()
		return fmt.Errorf("%w: %w", database.ErrMigrationFailed, ctx.Err())
	}
}
