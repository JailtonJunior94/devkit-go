package pgxpool_manager

import "time"

// LogFunc is a function type for logging SQL queries and warnings.
// Implementations should handle formatting via fmt.Sprintf(format, args...).
type LogFunc func(format string, args ...any)

// Config holds the configuration for pgxpool with OpenTelemetry instrumentation.
type Config struct {
	// DSN is the PostgreSQL connection string.
	// Format: postgres://user:password@host:port/database?sslmode=disable
	// Example: "postgres://myuser:mypass@localhost:5432/mydb?sslmode=require"
	DSN string

	// ServiceName identifies this service in traces and metrics.
	// Used as the tracer name and metric prefix.
	// Example: "payment-api", "order-service"
	ServiceName string

	// === Pool Configuration ===

	// MaxConns is the maximum size of the connection pool.
	// This limits total connections (active + idle).
	// Recommended: 25-100 depending on workload.
	// Rule of thumb: (expected concurrent requests * average query time) / target response time
	// Default: 25
	MaxConns int32

	// MinConns is the minimum number of connections maintained in the pool.
	// These connections stay open even during idle periods.
	// Recommended: 5-10 for most applications.
	// Trade-off: Higher values = lower latency but more memory/connections
	// Default: 5
	MinConns int32

	// MaxConnLifetime is the maximum duration a connection can be reused.
	// Forces connection rotation to prevent:
	// - Memory leaks in long-lived connections
	// - Stale connections after network changes
	// - PostgreSQL session state accumulation
	// Recommended: 5-15 minutes depending on network stability.
	// Default: 10 minutes
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum time a connection can remain idle.
	// Idle connections exceeding this duration are closed.
	// Recommended: 2-5 minutes for variable traffic patterns.
	// Default: 3 minutes
	MaxConnIdleTime time.Duration

	// HealthCheckPeriod is the interval between automatic connection health checks.
	// pgx periodically pings idle connections to ensure they're alive.
	// Recommended: 30 seconds - 2 minutes.
	// Too frequent = unnecessary overhead, too infrequent = stale connections linger
	// Default: 1 minute
	HealthCheckPeriod time.Duration

	// === Observability Configuration ===

	// EnableTracing enables automatic distributed tracing for all queries.
	// Each query becomes a span with attributes:
	// - db.system: "postgresql"
	// - db.statement: SQL query
	// - db.operation: SELECT, INSERT, UPDATE, DELETE
	// - db.connection_string: sanitized DSN (no password)
	// Default: true
	EnableTracing bool

	// EnableMetrics enables automatic metrics collection:
	// - Pool size (acquired, idle, max)
	// - Query duration
	// - Connection errors
	// Default: true (recommended for production)
	EnableMetrics bool

	// EnableQueryLogging enables SQL query logging with execution time.
	// CRITICAL: Only enable in development. In production, this causes:
	// - Massive log volume
	// - Potential PII/sensitive data exposure
	// - Degraded performance
	// Default: false
	EnableQueryLogging bool

	// Logger is an optional function for capturing SQL query logs.
	// Only used when EnableQueryLogging=true.
	// If nil (default), query logs are silenced.
	//
	// Example - Log to stdout:
	//   cfg.Logger = func(format string, args ...any) {
	//       log.Printf("[DB-QUERY] " + format, args...)
	//   }
	//
	// Example - Structured logging with slog:
	//   cfg.Logger = func(format string, args ...any) {
	//       slog.Debug(fmt.Sprintf(format, args...))
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
		MaxConns:            25,
		MinConns:            5,
		MaxConnLifetime:     10 * time.Minute,
		MaxConnIdleTime:     3 * time.Minute,
		HealthCheckPeriod:   1 * time.Minute,
		EnableTracing:       true,
		EnableMetrics:       true,
		EnableQueryLogging:  false, // NEVER enable in production
		Logger:              nil,   // Silent by default
	}
}
