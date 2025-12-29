package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Database defines the public interface for PostgreSQL database operations.
type Database interface {
	Connect(ctx context.Context) error
	DB() *sql.DB
	HealthCheck(ctx context.Context) error
	Close() error
}

// database is the private implementation of the Database interface.
type database struct {
	db     *sql.DB
	config *config
}

// New creates a new PostgreSQL database instance with the provided options.
func New(opts ...Option) Database {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &database{
		config: cfg,
	}
}

// Connect establishes a connection to the PostgreSQL database with retry logic.
func (d *database) Connect(ctx context.Context) error {
	if d.db != nil {
		return ErrAlreadyConnected
	}

	dsn := d.buildDSN()

	var db *sql.DB
	var err error

	for attempt := 0; attempt <= d.config.maxRetries; attempt++ {
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			if attempt < d.config.maxRetries {
				time.Sleep(d.config.retryInterval)
				continue
			}
			return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
		}

		if err = db.PingContext(ctx); err != nil {
			_ = db.Close()
			if attempt < d.config.maxRetries {
				time.Sleep(d.config.retryInterval)
				continue
			}
			return fmt.Errorf("%w: %v", ErrPingFailed, err)
		}

		break
	}

	d.configurePool(db)
	d.db = db

	return nil
}

// DB returns the underlying *sql.DB instance.
func (d *database) DB() *sql.DB {
	return d.db
}

// HealthCheck verifies the database connection is alive.
func (d *database) HealthCheck(ctx context.Context) error {
	if d.db == nil {
		return ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(ctx, d.config.pingTimeout)
	defer cancel()

	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrHealthCheckFailed, err)
	}

	return nil
}

// Close gracefully closes the database connection.
func (d *database) Close() error {
	if d.db == nil {
		return ErrNotConnected
	}

	if err := d.db.Close(); err != nil {
		return fmt.Errorf("%w: %v", ErrCloseFailed, err)
	}

	d.db = nil
	return nil
}

// buildDSN constructs the PostgreSQL connection string.
// If a DSN was provided via WithDSN, it takes precedence over individual parameters.
func (d *database) buildDSN() string {
	if d.config.dsn != "" {
		return d.config.dsn
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		d.config.host,
		d.config.port,
		d.config.user,
		d.config.password,
		d.config.database,
		d.config.sslMode,
		int(d.config.connectTimeout.Seconds()),
	)
}

// configurePool sets up the connection pool parameters.
func (d *database) configurePool(db *sql.DB) {
	db.SetMaxOpenConns(d.config.maxOpenConns)
	db.SetMaxIdleConns(d.config.maxIdleConns)
	db.SetConnMaxLifetime(d.config.connMaxLifetime)
	db.SetConnMaxIdleTime(d.config.connMaxIdleTime)
}
