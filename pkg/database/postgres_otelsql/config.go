package postgres_otelsql

import "time"

// LogFunc is a function type for logging warnings and informational messages.
// Implementations should handle formatting via fmt.Sprintf(format, args...).
type LogFunc func(format string, args ...any)

// Config holds the configuration for PostgreSQL with otelsql instrumentation.
// All fields are required unless explicitly marked as optional.
type Config struct {
	// DSN is the PostgreSQL Data Source Name.
	// Format: postgres://user:password@host:port/database?sslmode=disable
	// Example: "postgres://myuser:mypass@localhost:5432/mydb?sslmode=disable"
	DSN string

	// ServiceName identifies this service in traces and metrics.
	// Used as the prefix for all database metrics and span attributes.
	// Example: "order-service", "payment-api"
	ServiceName string

	// Pool Configuration
	// These settings directly impact connection reuse, memory usage, and latency.

	// MaxOpenConns is the maximum number of open connections to the database.
	// Includes both in-use and idle connections.
	// Recommended: 25-50 for most applications.
	// Set based on: (expected concurrent requests) * (average query time) / (desired response time)
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections in the pool.
	// Idle connections are kept alive for fast reuse without handshake overhead.
	// Recommended: 25-50% of MaxOpenConns for variable traffic patterns.
	// Default: 10
	MaxIdleConns int

	// ConnMaxLifetime is the maximum time a connection can be reused.
	// Forces connection rotation to prevent:
	// - Memory leaks in long-lived connections
	// - Stale connections after network changes
	// - Accumulation of PostgreSQL session state
	// Recommended: 5-15 minutes depending on network stability.
	// Default: 5 minutes
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum time a connection can remain idle.
	// Connections idle longer than this are closed to free resources.
	// Recommended: 1-3 minutes for variable traffic.
	// Default: 2 minutes
	ConnMaxIdleTime time.Duration

	// Observability Configuration

	// EnableMetrics enables automatic collection of connection pool metrics:
	// - db.client.connections.usage (gauge)
	// - db.client.connections.max (gauge)
	// - db.client.connections.idle (gauge)
	// - db.client.connections.wait_time (histogram)
	// Default: true
	EnableMetrics bool

	// EnableTracing enables automatic tracing of SQL queries with context propagation.
	// Each query becomes a span with attributes:
	// - db.system: "postgresql"
	// - db.statement: SQL query
	// - db.operation: SELECT, INSERT, UPDATE, DELETE
	// Default: true
	EnableTracing bool

	// EnableQueryLogging logs all SQL queries with execution time.
	// CRITICAL: Only enable in development. In production, this creates:
	// - High disk I/O
	// - Potential PII/sensitive data leaks in logs
	// - Performance degradation
	// Default: false
	EnableQueryLogging bool

	// Logger is an optional function for capturing logs and warnings.
	// If nil (default), logs are silenced (recommended for production).
	//
	// Example - Log to stdout:
	//   cfg.Logger = func(format string, args ...any) {
	//       log.Printf("[DB-WARNING] " + format, args...)
	//   }
	//
	// Example - Structured logging with slog:
	//   cfg.Logger = func(format string, args ...any) {
	//       slog.Warn(fmt.Sprintf(format, args...))
	//   }
	//
	// Default: nil (silent)
	Logger LogFunc
}

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig(dsn, serviceName string) *Config {
	return &Config{
		DSN:                 dsn,
		ServiceName:         serviceName,
		MaxOpenConns:        25,
		MaxIdleConns:        10,
		ConnMaxLifetime:     5 * time.Minute,
		ConnMaxIdleTime:     2 * time.Minute,
		EnableMetrics:       true,
		EnableTracing:       true,
		EnableQueryLogging:  false, // NEVER enable in production
		Logger:              nil,   // Silent by default
	}
}
