package migration

import "time"

// Option is a functional option for configuring the Migrator.
type Option func(*Config)

// WithDriver sets the database driver.
func WithDriver(driver Driver) Option {
	return func(c *Config) {
		c.Driver = driver
	}
}

// WithDSN sets the database connection string (Data Source Name).
func WithDSN(dsn string) Option {
	return func(c *Config) {
		c.DSN = dsn
	}
}

// WithSource sets the migration source location.
// Currently supports file:// URLs (e.g., "file://migrations" or "file:///absolute/path/to/migrations").
func WithSource(source string) Option {
	return func(c *Config) {
		c.Source = source
	}
}

// WithLogger sets the structured logger for migration operations.
// If not set, a no-op logger will be used.
func WithLogger(logger Logger) Option {
	return func(c *Config) {
		if logger != nil {
			c.Logger = logger
		}
	}
}

// WithTimeout sets the maximum duration for the entire migration operation.
// This includes connecting to the database and running all migrations.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		if timeout > 0 {
			c.Timeout = timeout
		}
	}
}

// WithLockTimeout sets the maximum time to wait for acquiring the migration lock.
// Set to 0 to wait indefinitely.
// This is particularly important for CockroachDB due to its distributed nature.
func WithLockTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		if timeout >= 0 {
			c.LockTimeout = timeout
		}
	}
}

// WithStatementTimeout sets the maximum duration for a single SQL statement.
// Set to 0 to use database defaults.
func WithStatementTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		if timeout >= 0 {
			c.StatementTimeout = timeout
		}
	}
}

// WithMultiStatement enables or disables multi-statement support.
// When enabled, migration files can contain multiple SQL statements.
func WithMultiStatement(enabled bool) Option {
	return func(c *Config) {
		c.MultiStatementEnabled = enabled
	}
}

// WithMultiStatementMaxSize sets the maximum size for multi-statement migration files.
// This prevents memory issues with very large migrations.
func WithMultiStatementMaxSize(size int) Option {
	return func(c *Config) {
		if size > 0 {
			c.MultiStatementMaxSize = size
		}
	}
}

// WithDatabaseName sets an optional database name for logging and observability.
// If not set, it will be extracted from the DSN.
func WithDatabaseName(name string) Option {
	return func(c *Config) {
		c.DatabaseName = name
	}
}

// WithPreferSimpleProtocol forces the use of simple query protocol.
// This can be useful for certain database configurations or debugging.
func WithPreferSimpleProtocol(prefer bool) Option {
	return func(c *Config) {
		c.PreferSimpleProtocol = prefer
	}
}

// WithConfig applies an entire Config struct.
// This is useful when you have a pre-built configuration.
func WithConfig(cfg Config) Option {
	return func(c *Config) {
		*c = cfg
	}
}
