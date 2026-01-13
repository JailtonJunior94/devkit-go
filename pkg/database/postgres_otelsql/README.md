# PostgreSQL DBManager with otelsql

Production-ready database connection manager using `database/sql` instrumented with `otelsql` for automatic OpenTelemetry observability.

## Features

- **Automatic Tracing**: Every SQL query creates a span with full context propagation
- **Connection Pool Metrics**: Real-time metrics on pool usage, wait times, and connection lifecycle
- **Clean Architecture**: Zero OpenTelemetry imports in domain layer
- **Production-Safe Defaults**: Optimized pool settings and security validations
- **Graceful Shutdown**: Properly closes connections without losing in-flight queries
- **Thread-Safe**: All operations safe for concurrent use

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ HTTP Request (with trace context)                            │
└───────────────────────┬─────────────────────────────────────┘
                        │ context.Context (contains trace_id)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ Handler Layer                                                │
│ - Extracts context from request                              │
│ - Passes context down to service                             │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ Service Layer (Domain Logic)                                 │
│ - NO database or OpenTelemetry imports                       │
│ - Passes context to repository                               │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ Repository Layer                                             │
│ - Uses database/sql with context                             │
│ - QueryRowContext(ctx, ...)                                  │
│ - ExecContext(ctx, ...)                                      │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ otelsql (automatic instrumentation)                          │
│ - Intercepts every query                                     │
│ - Creates child span automatically                           │
│ - Records metrics (duration, errors, pool stats)             │
│ - Adds span attributes:                                      │
│   * db.system: "postgresql"                                  │
│   * db.statement: "SELECT * FROM users WHERE id = $1"        │
│   * db.operation: "SELECT"                                   │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│ PostgreSQL                                                   │
└─────────────────────────────────────────────────────────────┘
```

## Installation

```bash
go get github.com/XSAM/otelsql
go get github.com/jackc/pgx/v5/stdlib
go get go.opentelemetry.io/otel
```

## Quick Start

### 1. Initialize OpenTelemetry (REQUIRED FIRST)

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

// Initialize OTel SDK before creating DBManager
otelProvider, err := otel.NewProvider(ctx, &otel.Config{
    ServiceName:    "my-service",
    OTLPEndpoint:   "localhost:4317",
    // ... other config
})
defer otelProvider.Shutdown(ctx)
```

### 2. Create DBManager (ONCE at startup)

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres_otelsql"
)

config := postgres_otelsql.DefaultConfig(
    "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
    "my-service",
)

dbManager, err := postgres_otelsql.NewDBManager(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer dbManager.Shutdown(ctx)
```

### 3. Use in Repositories

```go
type UserRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}

// CRITICAL: Always pass context from the caller
func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    query := `SELECT id, name, email FROM users WHERE id = $1`

    // otelsql automatically creates a span for this query
    row := r.db.QueryRowContext(ctx, query, id)

    var user User
    if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
        return nil, err
    }

    return &user, nil
}
```

## Configuration

### Production Settings

```go
config := &postgres_otelsql.Config{
    DSN:                 "postgres://...",
    ServiceName:         "order-service",
    MaxOpenConns:        50,        // Adjust based on load
    MaxIdleConns:        25,        // 50% of MaxOpenConns
    ConnMaxLifetime:     10 * time.Minute,
    ConnMaxIdleTime:     3 * time.Minute,
    EnableMetrics:       true,
    EnableTracing:       true,
    EnableQueryLogging:  false,     // NEVER true in production
}
```

### Development Settings

```go
config := postgres_otelsql.DefaultConfig(dsn, "dev-service")
config.EnableQueryLogging = true  // OK for local development
```

## Metrics Collected

When `EnableMetrics: true`, otelsql automatically exports:

| Metric | Type | Description |
|--------|------|-------------|
| `db.client.connections.usage` | Gauge | Current open connections |
| `db.client.connections.max` | Gauge | Maximum allowed connections |
| `db.client.connections.idle` | Gauge | Idle connections in pool |
| `db.client.connections.wait_time` | Histogram | Time waiting for connection |
| `db.client.operation.duration` | Histogram | Query execution time |

## Tracing

Every SQL query automatically creates a span with attributes:

```
Span: "SELECT users"
├─ db.system: "postgresql"
├─ db.operation: "SELECT"
├─ db.statement: "SELECT id, name, email FROM users WHERE id = $1"
├─ db.rows_affected: 1
└─ trace_id: "4bf92f3577b34da6a3ce929d0e0e4736"
```

## Common Patterns

### Health Checks

```go
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := h.dbManager.Ping(ctx); err != nil {
        http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

### Graceful Shutdown

```go
func main() {
    // ... setup ...

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Shutdown database connections
    if err := dbManager.Shutdown(shutdownCtx); err != nil {
        log.Printf("Database shutdown error: %v", err)
    }

    // Flush telemetry
    if err := otelProvider.Shutdown(shutdownCtx); err != nil {
        log.Printf("OTel shutdown error: %v", err)
    }
}
```

### Custom Metrics

```go
stats := dbManager.Stats()

// Export to Prometheus
dbConnectionsOpen.Set(float64(stats.OpenConnections))
dbConnectionsInUse.Set(float64(stats.InUse))
dbConnectionsIdle.Set(float64(stats.Idle))
dbWaitCount.Set(float64(stats.WaitCount))
dbWaitDuration.Set(stats.WaitDuration.Seconds())
```

## Anti-Patterns

### ❌ Creating DBManager per request

```go
// WRONG: Creates new connection pool for every request
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    dbManager, _ := postgres_otelsql.NewDBManager(r.Context(), config) // ❌
    defer dbManager.Shutdown(r.Context())
    // ...
}
```

✅ **Correct**: Create ONCE at startup

```go
// main.go
dbManager, _ := postgres_otelsql.NewDBManager(ctx, config)
defer dbManager.Shutdown(ctx)

// Inject into handlers/services
handler := NewHandler(dbManager.DB())
```

### ❌ Using context.Background() in handlers

```go
// WRONG: Breaks trace propagation
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRowContext(context.Background(), query, id) // ❌
    // ...
}
```

✅ **Correct**: Always pass context from caller

```go
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRowContext(ctx, query, id) // ✅
    // ...
}
```

### ❌ Enabling query logging in production

```go
// WRONG: High I/O, potential PII leaks
config.EnableQueryLogging = true // ❌ in production
```

✅ **Correct**: Only enable in development

```go
config.EnableQueryLogging = (os.Getenv("ENV") == "development")
```

## Testing

```go
func TestUserRepository(t *testing.T) {
    // Use sqlmock for unit tests
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    repo := NewUserRepository(db)

    mock.ExpectQuery("SELECT (.+) FROM users").
        WithArgs("usr_123").
        WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
            AddRow("usr_123", "John"))

    user, err := repo.FindByID(context.Background(), "usr_123")
    require.NoError(t, err)
    assert.Equal(t, "usr_123", user.ID)
}
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: DB_DSN
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: dsn
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "otel-collector:4317"

        # Readiness probe using DBManager.Ping()
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10

        # Liveness probe (application health, not DB)
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
```

## Production Checklist

- [ ] OpenTelemetry SDK initialized before DBManager
- [ ] DBManager created ONCE at startup
- [ ] `EnableQueryLogging: false` in production
- [ ] MaxOpenConns ≤ PostgreSQL `max_connections`
- [ ] Context propagated from HTTP handlers to repositories
- [ ] Graceful shutdown implemented (dbManager.Shutdown + otelProvider.Shutdown)
- [ ] Health checks use dbManager.Ping()
- [ ] No `context.Background()` in request handlers
- [ ] Connection pool metrics monitored
- [ ] DSN stored in secrets, not committed to git

## Troubleshooting

### Issue: No traces appearing

**Cause**: OpenTelemetry SDK not initialized before DBManager

**Solution**:
```go
// Initialize OTel FIRST
otelProvider, _ := otel.NewProvider(ctx, config)
defer otelProvider.Shutdown(ctx)

// THEN create DBManager
dbManager, _ := postgres_otelsql.NewDBManager(ctx, dbConfig)
```

### Issue: Connection pool exhaustion

**Symptoms**: Errors like "pq: sorry, too many clients already"

**Solution**: Reduce `MaxOpenConns` to be less than PostgreSQL `max_connections`

```sql
-- Check PostgreSQL max_connections
SHOW max_connections;

-- Check current connections
SELECT count(*) FROM pg_stat_activity;
```

### Issue: High wait times in metrics

**Symptoms**: `db.client.connections.wait_time` histogram showing high values

**Solution**: Increase `MaxOpenConns` or optimize slow queries

```go
config.MaxOpenConns = 50 // Increase pool size
```

## See Also

- [otelsql Documentation](https://github.com/XSAM/otelsql)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
- [database/sql Best Practices](https://www.alexedwards.net/blog/configuring-sqldb)
