package pgxpool_manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// PgxPoolManager manages a pgxpool connection pool instrumented with OpenTelemetry.
//
// Key Differences from database/sql:
//  1. Native PostgreSQL driver (pgx) with better performance
//  2. Built-in connection pooling with health checks
//  3. Better error messages and type safety
//  4. Supports PostgreSQL-specific features (LISTEN/NOTIFY, COPY, etc.)
//  5. Lower memory footprint
//
// Architecture Principles:
//  1. Single Instance: Create ONCE at application startup, share globally
//  2. Thread-Safe: All operations safe for concurrent use
//  3. Context-Aware: All operations respect context cancellation
//  4. Instrumented: Automatic tracing and metrics without code changes
//  5. Graceful Shutdown: Closes connections cleanly without losing data
//
// Critical Anti-Patterns:
//  - Creating pool per request (defeats pooling, exhausts connections)
//  - Using context.Background() in handlers (breaks tracing)
//  - Calling Close() then using Pool() (returns nil, causes panic)
//  - Setting MaxConns > PostgreSQL max_connections (connection errors)
type PgxPoolManager struct {
	pool   *pgxpool.Pool
	config *Config
	mu     sync.RWMutex // Protects closed flag during shutdown
	closed bool
	tracer trace.Tracer
}

// NewPgxPoolManager creates and initializes a new PgxPoolManager.
// This function MUST be called ONCE during application bootstrap.
//
// Initialization Steps:
//  1. Validates configuration
//  2. Configures pgxpool with observability hooks
//  3. Creates connection pool
//  4. Verifies connectivity with Ping
//  5. Returns ready-to-use manager
//
// Example Bootstrap:
//
//	func main() {
//	    ctx := context.Background()
//
//	    // Initialize OpenTelemetry FIRST
//	    otelProvider, _ := otel.NewProvider(ctx, otelConfig)
//	    defer otelProvider.Shutdown(ctx)
//
//	    // Create PgxPoolManager ONCE
//	    cfg := pgxpool_manager.DefaultConfig(
//	        "postgres://user:pass@localhost:5432/mydb",
//	        "my-service",
//	    )
//	    poolManager, err := pgxpool_manager.NewPgxPoolManager(ctx, cfg)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer poolManager.Shutdown(ctx)
//
//	    // Inject poolManager.Pool() into repositories
//	    repo := repository.NewUserRepository(poolManager.Pool())
//	}
func NewPgxPoolManager(ctx context.Context, config *Config) (*PgxPoolManager, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Parse DSN into pgxpool config
	poolConfig, err := pgxpool.ParseConfig(config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	// Apply pool settings
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckPeriod

	// Get tracer from global provider
	tracer := otel.Tracer(config.ServiceName)

	manager := &PgxPoolManager{
		config: config,
		tracer: tracer,
		closed: false,
	}

	// Configure observability hooks BEFORE creating pool
	if config.EnableTracing {
		poolConfig.ConnConfig.Tracer = &otelTracer{
			tracer:      tracer,
			serviceName: config.ServiceName,
		}
	}

	if config.EnableQueryLogging {
		poolConfig.ConnConfig.Tracer = &queryLogger{
			next:   poolConfig.ConnConfig.Tracer,
			logger: config.Logger,
		}
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	manager.pool = pool

	// Verify connectivity immediately (fail-fast)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		// Close pool to prevent resource leak
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
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

	if config.MaxConns <= 0 {
		return fmt.Errorf("MaxConns must be > 0, got %d", config.MaxConns)
	}

	if config.MinConns < 0 {
		return fmt.Errorf("MinConns cannot be negative, got %d", config.MinConns)
	}

	if config.MinConns > config.MaxConns {
		return fmt.Errorf("MinConns (%d) cannot exceed MaxConns (%d)",
			config.MinConns, config.MaxConns)
	}

	if config.MaxConnLifetime < time.Minute {
		return fmt.Errorf("MaxConnLifetime too short (minimum 1 minute), got %v",
			config.MaxConnLifetime)
	}

	if config.MaxConnIdleTime < 30*time.Second {
		return fmt.Errorf("MaxConnIdleTime too short (minimum 30 seconds), got %v",
			config.MaxConnIdleTime)
	}

	if config.HealthCheckPeriod < 10*time.Second {
		return fmt.Errorf("HealthCheckPeriod too short (minimum 10 seconds), got %v",
			config.HealthCheckPeriod)
	}

	return nil
}

// Pool returns the underlying *pgxpool.Pool for use in repositories.
//
// CRITICAL: The returned pool is thread-safe and should be shared across
// the entire application. Do NOT create a new pool per request.
//
// Usage in Repositories:
//
//	type UserRepository struct {
//	    pool *pgxpool.Pool
//	}
//
//	func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
//	    return &UserRepository{pool: pool}
//	}
//
//	func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
//	    query := `SELECT id, name, email FROM users WHERE id = $1`
//
//	    // ALWAYS pass ctx from caller for trace propagation
//	    var user User
//	    err := r.pool.QueryRow(ctx, query, id).Scan(&user.ID, &user.Name, &user.Email)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    return &user, nil
//	}
//
// Returns nil if the manager has been shut down.
func (m *PgxPoolManager) Pool() *pgxpool.Pool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil
	}

	return m.pool
}

// Ping verifies database connectivity.
// Use this for health checks (e.g., Kubernetes readiness probes).
//
// Example Fiber Health Check:
//
//	func (h *HealthHandler) Readiness(c *fiber.Ctx) error {
//	    ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
//	    defer cancel()
//
//	    if err := h.poolManager.Ping(ctx); err != nil {
//	        return c.Status(503).JSON(fiber.Map{
//	            "status": "unavailable",
//	            "error": "database unreachable",
//	        })
//	    }
//
//	    return c.JSON(fiber.Map{"status": "ok"})
//	}
func (m *PgxPoolManager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("pool manager is closed")
	}

	if err := m.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}

// Shutdown gracefully closes the connection pool.
// The context is checked BEFORE starting Close(), but Close() itself is blocking.
//
// Behavior:
//  - Checks if context is already expired BEFORE starting Close()
//  - Marks pool as closed to prevent new operations
//  - Executes blocking Close() (may exceed context deadline)
//  - Close() CANNOT be interrupted once started
//  - Idempotent and thread-safe
//
// IMPORTANT:
//  - Close() is blocking by design in pgxpool
//  - If context expires DURING Close(), operation continues until complete
//  - Trade-off: We prefer closing connections completely rather than leaving them orphaned
//
// MUST be called during application shutdown:
//
//	func main() {
//	    // ... setup ...
//
//	    defer func() {
//	        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	        defer cancel()
//
//	        if err := poolManager.Shutdown(shutdownCtx); err != nil {
//	            log.Printf("Error during shutdown: %v", err)
//	        }
//	    }()
//
//	    // ... run application ...
//	}
func (m *PgxPoolManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Idempotent: if already closed, do nothing
	if m.closed {
		return nil
	}

	// Check if context is already expired BEFORE starting Close()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("shutdown aborted (context expired): %w", err)
	}

	// Mark as closed to prevent new operations
	m.closed = true

	// Close() is blocking and does NOT respect ctx.Done() after starting
	// Waits for all active connections to be released gracefully
	m.pool.Close()

	return nil
}

// Stats returns pool statistics for monitoring.
//
// Example Custom Metrics:
//
//	func (m *MetricsCollector) Collect() {
//	    stats := m.poolManager.Stats()
//	    pgxPoolAcquiredConns.Set(float64(stats.AcquiredConns()))
//	    pgxPoolIdleConns.Set(float64(stats.IdleConns()))
//	    pgxPoolMaxConns.Set(float64(stats.MaxConns()))
//	    pgxPoolTotalConns.Set(float64(stats.TotalConns()))
//	}
func (m *PgxPoolManager) Stats() *pgxpool.Stat {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil
	}

	return m.pool.Stat()
}

// otelTracer implements pgx.QueryTracer for OpenTelemetry integration.
type otelTracer struct {
	tracer      trace.Tracer
	serviceName string
}

// TraceQueryStart creates a span when a query begins.
func (t *otelTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			attribute.String("db.statement", data.SQL),
			attribute.String("db.operation", extractOperation(data.SQL)),
		),
	}

	ctx, _ = t.tracer.Start(ctx, "pgx.query", opts...)
	return ctx
}

// TraceQueryEnd finalizes the span when a query completes.
func (t *otelTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
		if data.CommandTag.RowsAffected() > 0 {
			span.SetAttributes(attribute.Int64("db.rows_affected", data.CommandTag.RowsAffected()))
		}
	}

	span.End()
}

// queryLogger logs SQL queries (for development only).
type queryLogger struct {
	next   pgx.QueryTracer
	logger LogFunc
}

func (q *queryLogger) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if q.logger != nil {
		q.logger("[SQL] %s [ARGS] %v", data.SQL, data.Args)
	}

	if q.next != nil {
		return q.next.TraceQueryStart(ctx, conn, data)
	}

	return ctx
}

func (q *queryLogger) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if q.logger != nil {
		if data.Err != nil {
			q.logger("[SQL ERROR] %v", data.Err)
		} else {
			q.logger("[SQL SUCCESS] [ROWS] %d", data.CommandTag.RowsAffected())
		}
	}

	if q.next != nil {
		q.next.TraceQueryEnd(ctx, conn, data)
	}
}

// extractOperation extracts the SQL operation type (SELECT, INSERT, etc.).
// Handles queries with leading whitespace and case-insensitive matching.
func extractOperation(sql string) string {
	// Trim whitespace (espaços, tabs, newlines)
	trimmed := strings.TrimSpace(sql)
	if len(trimmed) == 0 {
		return "UNKNOWN"
	}

	// Pega primeira palavra (até espaço ou fim da string)
	firstWord, _, _ := strings.Cut(trimmed, " ")

	// Normaliza para uppercase
	operation := strings.ToUpper(firstWord)

	// Classifica operações conhecidas
	switch operation {
	case "SELECT":
		return "SELECT"
	case "INSERT":
		return "INSERT"
	case "UPDATE":
		return "UPDATE"
	case "DELETE":
		return "DELETE"
	case "CREATE", "DROP", "ALTER", "TRUNCATE":
		return "DDL"
	case "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT":
		return "TRANSACTION"
	case "WITH":
		// CTEs (Common Table Expressions) are almost always SELECT queries.
		// Parsing nested parentheses correctly is complex and not worth it for tracing labels.
		return "SELECT"
	default:
		return "OTHER"
	}
}

// Ensure interfaces are implemented correctly.
var _ pgx.QueryTracer = (*otelTracer)(nil)
var _ pgx.QueryTracer = (*queryLogger)(nil)
