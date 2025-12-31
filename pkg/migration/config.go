package migration

import (
	"fmt"
	"strings"
	"time"
)

// Config holds the configuration for database migrations.
type Config struct {
	// Driver specifies the database driver (postgres, cockroachdb).
	Driver Driver

	// DSN is the database connection string (Data Source Name).
	DSN string

	// Source specifies the location of migration files.
	// Currently supports file:// URLs (e.g., "file://migrations").
	// Future support planned for: s3://, gcs://, github://, etc.
	Source string

	// Logger is the structured logger for migration operations.
	Logger Logger

	// Timeout is the maximum duration for the entire migration operation.
	// This includes connecting to the database and running all migrations.
	Timeout time.Duration

	// LockTimeout is the maximum time to wait for acquiring the migration lock.
	// Set to 0 to wait indefinitely.
	// For CockroachDB, this is particularly important due to distributed transactions.
	LockTimeout time.Duration

	// StatementTimeout is the maximum duration for a single SQL statement.
	// Set to 0 to use database defaults.
	StatementTimeout time.Duration

	// MultiStatementEnabled allows running multiple SQL statements in a single migration file.
	// This is useful for complex migrations but requires careful transaction handling.
	MultiStatementEnabled bool

	// MultiStatementMaxSize limits the size of multi-statement migration files.
	// This prevents memory issues with very large migrations.
	MultiStatementMaxSize int

	// DatabaseName is an optional name used in logging for better observability.
	// If empty, it will be extracted from the DSN.
	DatabaseName string

	// PreferSimpleProtocol forces the use of simple query protocol.
	// Useful for certain database configurations or debugging.
	PreferSimpleProtocol bool
}

// DefaultConfig returns a Config with sensible defaults for production use.
func DefaultConfig() Config {
	return Config{
		Driver:                DriverPostgres,
		Logger:                NewNoopLogger(),
		Timeout:               5 * time.Minute,
		LockTimeout:           30 * time.Second,
		StatementTimeout:      0, // Use database default
		MultiStatementEnabled: true,
		MultiStatementMaxSize: 10 * 1024 * 1024, // 10MB
		PreferSimpleProtocol:  false,
	}
}

// Validate checks if the configuration is valid and returns detailed error messages.
func (c Config) Validate() error {
	if !c.Driver.IsValid() {
		return fmt.Errorf("%w: %s (supported: postgres, cockroachdb)", ErrInvalidDriver, c.Driver)
	}

	if strings.TrimSpace(c.DSN) == "" {
		return fmt.Errorf("%w: DSN cannot be empty", ErrMissingDSN)
	}

	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("%w: source cannot be empty", ErrMissingSource)
	}

	if !strings.HasPrefix(c.Source, "file://") {
		return fmt.Errorf("invalid migration source: must start with file:// (got: %s)", c.Source)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("%w: got %v", ErrInvalidTimeout, c.Timeout)
	}

	if c.LockTimeout < 0 {
		return fmt.Errorf("%w: got %v", ErrInvalidLockTimeout, c.LockTimeout)
	}

	if c.StatementTimeout < 0 {
		return fmt.Errorf("statement timeout must be non-negative: got %v", c.StatementTimeout)
	}

	if c.MultiStatementEnabled && c.MultiStatementMaxSize <= 0 {
		return fmt.Errorf("multi-statement max size must be positive when multi-statement is enabled: got %d", c.MultiStatementMaxSize)
	}

	if c.Logger == nil {
		return fmt.Errorf("logger cannot be nil (use NewNoopLogger() if logging is not needed)")
	}

	return nil
}

// extractDatabaseNameFromDSN attempts to extract the database name from the DSN.
// Returns empty string if extraction fails.
func extractDatabaseNameFromDSN(dsn string) string {
	parts := strings.Split(dsn, "/")
	if len(parts) == 0 {
		return ""
	}

	dbPart := parts[len(parts)-1]

	if idx := strings.Index(dbPart, "?"); idx != -1 {
		dbPart = dbPart[:idx]
	}

	return strings.TrimSpace(dbPart)
}

// GetDatabaseName returns the configured database name or extracts it from DSN.
func (c Config) GetDatabaseName() string {
	if c.DatabaseName != "" {
		return c.DatabaseName
	}
	return extractDatabaseNameFromDSN(c.DSN)
}
