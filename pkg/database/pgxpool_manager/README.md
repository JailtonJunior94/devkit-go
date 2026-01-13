## DBManager B: pgxpool + Fiber + OpenTelemetry

**Production-grade PostgreSQL connection pool manager** using `pgxpool` with automatic distributed tracing for Fiber HTTP applications.

## Features

- **Native PostgreSQL Driver**: pgx offers better performance than database/sql
- **Automatic Distributed Tracing**: HTTP → Service → Database with full context propagation
- **Fiber Integration**: First-class support for GoFiber with `otelfiber` middleware
- **Built-in Health Checks**: Connection pool health monitoring
- **Clean Architecture**: Domain layer remains pure, no framework dependencies
- **Production-Ready**: Sensible defaults, security validations, graceful shutdown
- **Zero Allocation Instrumentation**: Minimal overhead tracing

## Distributed Tracing Flow

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. HTTP Request with Trace Headers                               │
│    Traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-...          │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 2. otelfiber Middleware (AUTOMATIC)                              │
│    - Extracts trace context from HTTP headers                    │
│    - Creates root span: "GET /api/v1/users/:id"                  │
│    - Injects span into fiber.Ctx                                 │
│    - Span Attributes:                                            │
│      * http.method: GET                                          │
│      * http.url: /api/v1/users/123                               │
│      * http.status_code: 200                                     │
└────────────────────────────┬─────────────────────────────────────┘
                             │ c.UserContext() → contains span
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. HTTP Handler                                                  │
│    func (h *UserHandler) GetUser(c *fiber.Ctx) error {           │
│        ctx := c.UserContext() // ← Gets context with span        │
│        user, err := h.service.GetUserByID(ctx, id)               │
│    }                                                             │
└────────────────────────────┬─────────────────────────────────────┘
                             │ passes ctx down
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 4. Service Layer (Domain Logic)                                  │
│    func (s *UserService) GetUserByID(ctx context.Context, ...) { │
│        return s.repo.FindByID(ctx, id) // ← propagates ctx       │
│    }                                                             │
└────────────────────────────┬─────────────────────────────────────┘
                             │ passes ctx down
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 5. Repository Layer                                              │
│    func (r *UserRepo) FindByID(ctx context.Context, ...) {       │
│        r.pool.QueryRow(ctx, query, id) // ← pgx gets ctx         │
│    }                                                             │
└────────────────────────────┬─────────────────────────────────────┘
                             │ pgxpool_manager hooks into queries
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 6. otelTracer Hook (AUTOMATIC)                                   │
│    - TraceQueryStart() creates child span: "pgx.query"           │
│    - Span Attributes:                                            │
│      * db.system: "postgresql"                                   │
│      * db.statement: "SELECT * FROM users WHERE id = $1"         │
│      * db.operation: "SELECT"                                    │
│      * db.rows_affected: 1                                       │
│    - TraceQueryEnd() finalizes span with duration                │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ 7. PostgreSQL Database                                           │
└──────────────────────────────────────────────────────────────────┘

Result: Single trace with 2 spans sharing the same trace_id:
  ├─ Span 1 (root): "GET /api/v1/users/123" [http.method, http.url]
  └─ Span 2 (child): "pgx.query" [db.system, db.statement, db.operation]
```

## Installation

```bash
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/gofiber/fiber/v2
go get go.opentelemetry.io/contrib/instrumentation/github.com/gofiber/fiber/otelfiber/v2
go get go.opentelemetry.io/otel
```

## Quick Start

### Complete Example (see examples/fiber_complete/main.go)

The complete example demonstrates:
- Clean Architecture layers (Domain, Application, Infrastructure, Presentation)
- Full distributed tracing: HTTP → Service → Repository → Database
- Context propagation using `c.UserContext()`
- Health checks with connection pool stats
- Graceful shutdown

### Minimal Setup

```go
package main

import (
    "github.com/JailtonJunior94/devkit-go/pkg/database/pgxpool_manager"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
    "github.com/gofiber/fiber/v2"
    "go.opentelemetry.io/contrib/instrumentation/github.com/gofiber/fiber/otelfiber/v2"
)

func main() {
    ctx := context.Background()

    // 1. Initialize OpenTelemetry FIRST
    otelProvider, _ := otel.NewProvider(ctx, &otel.Config{
        ServiceName:  "my-api",
        OTLPEndpoint: "localhost:4317",
    })
    defer otelProvider.Shutdown(ctx)

    // 2. Create PgxPoolManager ONCE
    poolManager, _ := pgxpool_manager.NewPgxPoolManager(ctx,
        pgxpool_manager.DefaultConfig(
            "postgres://user:pass@localhost:5432/mydb",
            "my-api",
        ),
    )
    defer poolManager.Shutdown(ctx)

    // 3. Setup Fiber with otelfiber middleware
    app := fiber.New()

    // CRITICAL: Register otelfiber BEFORE routes
    app.Use(otelfiber.Middleware())

    // 4. Register routes
    app.Get("/users/:id", func(c *fiber.Ctx) error {
        // CRITICAL: Use c.UserContext() for trace propagation
        ctx := c.UserContext()

        var user User
        err := poolManager.Pool().QueryRow(ctx,
            "SELECT id, name FROM users WHERE id = $1",
            c.Params("id"),
        ).Scan(&user.ID, &user.Name)

        if err != nil {
            return c.Status(404).SendString("User not found")
        }

        return c.JSON(user)
    })

    app.Listen(":8080")
}
```

## Configuration

### Production Settings

```go
config := &pgxpool_manager.Config{
    DSN:                 "postgres://...",
    ServiceName:         "payment-api",
    MaxConns:            50,   // Adjust based on load
    MinConns:            10,   // Keep warm connections
    MaxConnLifetime:     15 * time.Minute,
    MaxConnIdleTime:     5 * time.Minute,
    HealthCheckPeriod:   1 * time.Minute,
    EnableTracing:       true,
    EnableMetrics:       true,
    EnableQueryLogging:  false, // NEVER true in production
}
```

### Development Settings

```go
config := pgxpool_manager.DefaultConfig(dsn, "dev-api")
config.EnableQueryLogging = true // See SQL queries in console
config.MaxConns = 10              // Lower pool size for dev
```

## Fiber Integration

### Critical: Context Propagation

```go
// ✅ CORRECT: Use c.UserContext()
app.Get("/users/:id", func(c *fiber.Ctx) error {
    ctx := c.UserContext() // Contains trace span from otelfiber
    user, err := service.GetUser(ctx, c.Params("id"))
    return c.JSON(user)
})

// ❌ WRONG: Using context.Background() breaks tracing
app.Get("/users/:id", func(c *fiber.Ctx) error {
    ctx := context.Background() // ❌ No trace propagation!
    user, err := service.GetUser(ctx, c.Params("id"))
    return c.JSON(user)
})
```

### Health Checks

```go
type HealthHandler struct {
    poolManager *pgxpool_manager.PgxPoolManager
}

// Liveness: Is the app running?
func (h *HealthHandler) Liveness(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{"status": "ok"})
}

// Readiness: Can the app serve traffic?
func (h *HealthHandler) Readiness(c *fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
    defer cancel()

    if err := h.poolManager.Ping(ctx); err != nil {
        return c.Status(503).JSON(fiber.Map{
            "status": "unavailable",
            "error": "database unreachable",
        })
    }

    stats := h.poolManager.Stats()
    return c.JSON(fiber.Map{
        "status": "ok",
        "database": fiber.Map{
            "acquired": stats.AcquiredConns(),
            "idle":     stats.IdleConns(),
            "max":      stats.MaxConns(),
        },
    })
}
```

## Repository Pattern

```go
type UserRepository struct {
    pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
    return &UserRepository{pool: pool}
}

// FindByID demonstrates context propagation and automatic tracing
func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    query := `SELECT id, email, name, created_at FROM users WHERE id = $1`

    var user User
    // CRITICAL: Pass ctx from caller
    // pgxpool_manager automatically creates a span for this query
    err := r.pool.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Email,
        &user.Name,
        &user.CreatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to find user: %w", err)
    }

    return &user, nil
}

// Transaction example
func (r *UserRepository) TransferMoney(ctx context.Context, fromID, toID string, amount int64) error {
    // Begin transaction
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) // Rollback if not committed

    // Debit from account
    _, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
    if err != nil {
        return err
    }

    // Credit to account
    _, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
    if err != nil {
        return err
    }

    // Commit transaction
    return tx.Commit(ctx)
}
```

## Tracing Output

When a request is made to `GET /api/v1/users/123`, the following spans are created:

```
Trace ID: 4bf92f3577b34da6a3ce929d0e0e4736

├─ Span: "GET /api/v1/users/:id" (duration: 45ms)
│  ├─ http.method: GET
│  ├─ http.url: /api/v1/users/123
│  ├─ http.status_code: 200
│  ├─ http.user_agent: curl/7.64.1
│  └─ service.name: user-api
│
└─ Span: "pgx.query" (duration: 12ms) [child of above]
   ├─ db.system: postgresql
   ├─ db.statement: SELECT id, email, name, created_at FROM users WHERE id = $1
   ├─ db.operation: SELECT
   ├─ db.rows_affected: 1
   └─ service.name: user-api
```

## Metrics

pgxpool automatically exposes connection pool metrics:

| Metric | Description |
|--------|-------------|
| `AcquiredConns()` | Currently in-use connections |
| `IdleConns()` | Available idle connections |
| `MaxConns()` | Maximum allowed connections |
| `TotalConns()` | Total connections (acquired + idle) |

Access via `poolManager.Stats()`:

```go
stats := poolManager.Stats()
fmt.Printf("Pool Usage: %d/%d (Idle: %d)\n",
    stats.AcquiredConns(),
    stats.MaxConns(),
    stats.IdleConns(),
)
```

## Anti-Patterns

### ❌ Creating Pool Per Request

```go
// WRONG: Defeats pooling, exhausts connections
func HandleRequest(c *fiber.Ctx) error {
    pool, _ := pgxpool_manager.NewPgxPoolManager(c.UserContext(), config) // ❌
    defer pool.Shutdown(c.UserContext())
    // ...
}
```

✅ **Correct**: Create ONCE at startup

```go
// main.go
poolManager, _ := pgxpool_manager.NewPgxPoolManager(ctx, config)
defer poolManager.Shutdown(ctx)

handler := NewHandler(poolManager.Pool())
```

### ❌ Not Using c.UserContext()

```go
// WRONG: Breaks distributed tracing
func HandleRequest(c *fiber.Ctx) error {
    ctx := context.Background() // ❌ No trace propagation
    user, _ := repo.FindByID(ctx, id)
    return c.JSON(user)
}
```

✅ **Correct**: Always use c.UserContext()

```go
func HandleRequest(c *fiber.Ctx) error {
    ctx := c.UserContext() // ✅ Trace propagation works
    user, _ := repo.FindByID(ctx, id)
    return c.JSON(user)
}
```

### ❌ Registering otelfiber After Routes

```go
// WRONG: Routes won't be traced
app.Get("/users/:id", handler)
app.Use(otelfiber.Middleware()) // ❌ Too late!
```

✅ **Correct**: Register middleware BEFORE routes

```go
app.Use(otelfiber.Middleware()) // ✅ Register first
app.Get("/users/:id", handler)
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-api
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: user-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_DSN
          valueFrom:
            secretKeyRef:
              name: postgres-creds
              key: dsn
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "otel-collector.observability.svc:4317"

        # Readiness probe using health check
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 2
          failureThreshold: 3

        # Liveness probe
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10

        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

## Testing

### Unit Tests with sqlmock

```go
func TestUserRepository(t *testing.T) {
    // For unit tests, use a mock pool or test database
    pool := setupTestPool(t)
    defer pool.Close()

    repo := NewUserRepository(pool)

    user, err := repo.FindByID(context.Background(), "usr_123")
    require.NoError(t, err)
    assert.Equal(t, "usr_123", user.ID)
}
```

### Integration Tests

```go
func TestUserAPI(t *testing.T) {
    // Setup test database
    poolManager, _ := pgxpool_manager.NewPgxPoolManager(context.Background(),
        pgxpool_manager.DefaultConfig(
            "postgres://postgres:postgres@localhost:5432/testdb",
            "test-api",
        ),
    )
    defer poolManager.Shutdown(context.Background())

    // Setup Fiber app
    app := fiber.New()
    app.Use(otelfiber.Middleware())

    repo := NewUserRepository(poolManager.Pool())
    handler := NewUserHandler(repo)
    app.Get("/users/:id", handler.GetUser)

    // Make test request
    req := httptest.NewRequest("GET", "/users/123", nil)
    resp, _ := app.Test(req)

    assert.Equal(t, 200, resp.StatusCode)
}
```

## Production Checklist

- [ ] OpenTelemetry SDK initialized before PgxPoolManager
- [ ] PgxPoolManager created ONCE at startup
- [ ] `otelfiber.Middleware()` registered BEFORE routes
- [ ] All handlers use `c.UserContext()`, never `context.Background()`
- [ ] `EnableQueryLogging: false` in production
- [ ] MaxConns ≤ PostgreSQL `max_connections`
- [ ] Graceful shutdown implemented (Fiber + Pool + OTel)
- [ ] Health checks configured (/health/live, /health/ready)
- [ ] Readiness probe uses poolManager.Ping()
- [ ] Connection pool metrics monitored
- [ ] DSN stored in secrets, not in code

## Troubleshooting

### No traces appearing in Jaeger/Tempo

**Cause**: otelfiber middleware not registered or registered after routes

**Solution**:
```go
// Register middleware FIRST
app.Use(otelfiber.Middleware())

// THEN register routes
app.Get("/users/:id", handler)
```

### Traces show HTTP span but no database span

**Cause**: Not using `c.UserContext()` in handlers

**Solution**:
```go
// Get context with span from Fiber
ctx := c.UserContext()

// Pass to service/repository
user, err := service.GetUser(ctx, id)
```

### Pool exhaustion errors

**Symptoms**: Errors like "cannot acquire connection"

**Check pool stats**:
```go
stats := poolManager.Stats()
fmt.Printf("Acquired: %d, Max: %d\n", stats.AcquiredConns(), stats.MaxConns())
```

**Solutions**:
- Increase MaxConns if hitting limit frequently
- Check for connection leaks (not returning connections)
- Optimize slow queries

### High connection wait times

**Solution**: Increase pool size or reduce MinConns to allow dynamic scaling

```go
config.MaxConns = 50
config.MinConns = 10
```

## See Also

- [pgx Documentation](https://github.com/jackc/pgx)
- [Fiber Documentation](https://docs.gofiber.io/)
- [otelfiber Middleware](https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/instrumentation/github.com/gofiber/fiber/otelfiber)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
