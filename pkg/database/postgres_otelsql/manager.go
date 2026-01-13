package postgres_otelsql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// DBManager manages a PostgreSQL connection pool instrumented with OpenTelemetry.
//
// Architecture Decisions:
//  1. Single Responsibility: This manager ONLY handles connection lifecycle.
//     Observability (tracing, metrics) is configured externally via otel global providers.
//  2. Initialized Once: Create ONE instance at application startup and share it.
//     Creating multiple instances defeats connection pooling and exhausts database resources.
//  3. Thread-Safe: All operations are safe for concurrent use.
//  4. Context-Aware: All operations respect context cancellation and timeouts.
//  5. Graceful Shutdown: Connections are properly closed without losing in-flight queries.
//
// Anti-Patterns to AVOID:
//  - Creating a new manager per HTTP request (destroys pooling)
//  - Calling Close() and then using DB() (returns nil, causes panics)
//  - Using context.Background() in handlers (breaks tracing propagation)
//  - Setting MaxOpenConns > PostgreSQL max_connections (causes connection errors)
type DBManager struct {
	db     *sql.DB
	config *Config
	mu     sync.RWMutex // Protects closed flag during shutdown
	closed bool
}

// NewDBManager creates and initializes a new DBManager.
// This function MUST be called ONCE during application bootstrap.
//
// Initialization Steps:
//  1. Validates configuration
//  2. Opens instrumented connection with otelsql
//  3. Configures connection pool
//  4. Verifies connectivity with Ping
//  5. Returns ready-to-use manager
//
// Example Bootstrap (main.go):
//
//	func main() {
//	    ctx := context.Background()
//
//	    // Initialize OpenTelemetry SDK first (required for instrumentation)
//	    otelProvider, err := otel.NewProvider(ctx, otelConfig)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer otelProvider.Shutdown(ctx)
//
//	    // Create DBManager ONCE
//	    cfg := postgres_otelsql.DefaultConfig(
//	        "postgres://user:pass@localhost:5432/mydb",
//	        "my-service",
//	    )
//	    dbManager, err := postgres_otelsql.NewDBManager(ctx, cfg)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer dbManager.Shutdown(ctx)
//
//	    // Inject dbManager.DB() into repositories
//	    repo := repository.NewUserRepository(dbManager.DB())
//	}
func NewDBManager(ctx context.Context, config *Config) (*DBManager, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Register otelsql wrapper with pgx driver
	// This automatically instruments all queries with:
	// - Distributed tracing (spans for each query)
	// - Metrics (connection pool stats, query duration)
	// - Context propagation (trace_id flows through queries)
	driverName, err := otelsql.Register(
		"pgx",
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			// DisableErrSkip: false means we DON'T create spans for sql.ErrNoRows
			// This reduces noise - missing rows is not an error condition
			DisableErrSkip: false,
		}),
		otelsql.WithSQLCommenter(config.EnableQueryLogging),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register otelsql driver: %w", err)
	}

	// Open connection using the instrumented driver
	db, err := sql.Open(driverName, config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	manager := &DBManager{
		db:     db,
		config: config,
		closed: false,
	}

	// Configure connection pool
	manager.configurePool()

	// Verify connectivity immediately (fail-fast)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		// If ping fails, close the connection to prevent resource leak
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Start metrics recording if enabled
	if config.EnableMetrics {
		if _, err := otelsql.RegisterDBStatsMetrics(db); err != nil {
			// Non-fatal: we can continue without metrics
			// But we should log this in production
			fmt.Printf("WARNING: Failed to register otelsql metrics: %v\n", err)
		}
	}

	return manager, nil
}

// validateConfig validates required configuration fields.
func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.DSN == "" {
		return fmt.Errorf("DSN cannot be empty")
	}

	if config.ServiceName == "" {
		return fmt.Errorf("ServiceName cannot be empty")
	}

	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("MaxOpenConns must be > 0, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns < 0 {
		return fmt.Errorf("MaxIdleConns cannot be negative, got %d", config.MaxIdleConns)
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("MaxIdleConns (%d) cannot exceed MaxOpenConns (%d)",
			config.MaxIdleConns, config.MaxOpenConns)
	}

	if config.ConnMaxLifetime < time.Minute {
		return fmt.Errorf("ConnMaxLifetime too short (minimum 1 minute), got %v", config.ConnMaxLifetime)
	}

	if config.ConnMaxIdleTime < 30*time.Second {
		return fmt.Errorf("ConnMaxIdleTime too short (minimum 30 seconds), got %v", config.ConnMaxIdleTime)
	}

	return nil
}

// configurePool applies connection pool settings.
func (m *DBManager) configurePool() {
	m.db.SetMaxOpenConns(m.config.MaxOpenConns)
	m.db.SetMaxIdleConns(m.config.MaxIdleConns)
	m.db.SetConnMaxLifetime(m.config.ConnMaxLifetime)
	m.db.SetConnMaxIdleTime(m.config.ConnMaxIdleTime)
}

// DB returns the underlying *sql.DB for use in repositories.
//
// CRITICAL: The returned *sql.DB is thread-safe and should be shared across
// the entire application. Do NOT create a new connection per request.
//
// Usage in Repositories:
//
//	type UserRepository struct {
//	    db *sql.DB
//	}
//
//	func NewUserRepository(db *sql.DB) *UserRepository {
//	    return &UserRepository{db: db}
//	}
//
//	func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
//	    // ALWAYS pass the context from the caller
//	    // This ensures trace propagation and respects cancellation
//	    row := r.db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", id)
//	    // ... scan logic
//	}
//
// Returns nil if the manager has been shut down.
func (m *DBManager) DB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil
	}

	return m.db
}

// Ping verifies database connectivity.
// Use this for health checks (e.g., Kubernetes readiness probes).
//
// Example Health Check Handler:
//
//	func (h *HealthHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
//	    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
//	    defer cancel()
//
//	    if err := h.dbManager.Ping(ctx); err != nil {
//	        http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
//	        return
//	    }
//
//	    w.WriteHeader(http.StatusOK)
//	}
func (m *DBManager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("database manager is closed")
	}

	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}

// Shutdown gracefully closes the database connection pool.
// This method is idempotent and safe to call multiple times.
//
// Behavior:
//  - Waits for all active queries to complete (respects context timeout)
//  - Closes all idle connections
//  - Prevents new queries from starting
//
// MUST be called during application shutdown:
//
//	func main() {
//	    // ... setup ...
//	    defer func() {
//	        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	        defer cancel()
//
//	        if err := dbManager.Shutdown(shutdownCtx); err != nil {
//	            log.Printf("Error during database shutdown: %v", err)
//	        }
//	    }()
//
//	    // ... run application ...
//	}
func (m *DBManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Idempotent: if already closed, do nothing
	if m.closed {
		return nil
	}

	// Mark as closed to prevent new operations
	m.closed = true

	// Close the connection pool in a goroutine to respect context timeout
	done := make(chan error, 1)
	go func() {
		done <- m.db.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Context expired, but Close() continues in background
		return fmt.Errorf("shutdown timeout exceeded: %w", ctx.Err())
	}
}

// Stats returns database statistics for monitoring.
// Useful for custom metrics, debugging, or dashboards.
//
// Example Prometheus Metrics:
//
//	func (m *MetricsCollector) Collect() {
//	    stats := m.dbManager.Stats()
//	    dbConnectionsOpen.Set(float64(stats.OpenConnections))
//	    dbConnectionsInUse.Set(float64(stats.InUse))
//	    dbConnectionsIdle.Set(float64(stats.Idle))
//	    dbConnectionWaitCount.Set(float64(stats.WaitCount))
//	    dbConnectionWaitDuration.Set(stats.WaitDuration.Seconds())
//	}
func (m *DBManager) Stats() sql.DBStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return sql.DBStats{}
	}

	return m.db.Stats()
}
