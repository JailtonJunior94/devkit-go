# PostgreSQL Package Architecture

## Overview

This document describes the architectural decisions, design patterns, and implementation details of the PostgreSQL database package.

## Design Principles

### 1. SOLID Principles

#### Single Responsibility Principle (SRP)
- **database.go**: Connection management and lifecycle
- **config.go**: Configuration defaults and structures
- **options.go**: Functional options for configuration
- **errors.go**: Error definitions

Each file has a single, well-defined responsibility.

#### Open/Closed Principle (OCP)
The package is open for extension through functional options but closed for modification. New configuration options can be added without changing existing code.

```go
// Adding a new option doesn't require modifying the database struct
func WithNewFeature(value string) Option {
    return func(c *config) {
        c.newFeature = value
    }
}
```

#### Liskov Substitution Principle (LSP)
The `Database` interface can be substituted with any implementation without breaking client code. This enables easy mocking for tests.

#### Interface Segregation Principle (ISP)
The `Database` interface is minimal and focused, containing only essential methods:
- `Connect()`
- `DB()`
- `HealthCheck()`
- `Close()`

#### Dependency Inversion Principle (DIP)
Clients depend on the `Database` interface (abstraction), not the concrete `database` implementation.

### 2. DRY (Don't Repeat Yourself)

Configuration logic is centralized in the `config` struct with default values defined once in constants. The functional options pattern eliminates repetitive configuration code.

### 3. Clean Code

- **Meaningful Names**: `Connect()`, `HealthCheck()`, `WithMaxOpenConns()`
- **Small Functions**: Each function does one thing well
- **No Magic Numbers**: All defaults are named constants
- **Error Handling**: Explicit error handling with typed errors

## Functional Options Pattern

### Why Functional Options?

1. **Backward Compatibility**: Adding new options doesn't break existing code
2. **Clarity**: Each option is self-documenting
3. **Flexibility**: Options can be applied in any order
4. **Defaults**: Sensible defaults without requiring configuration
5. **Validation**: Each option can validate its input

### Implementation

```go
type Option func(*config)

func New(opts ...Option) Database {
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    return &database{config: cfg}
}
```

### Benefits Over Alternatives

**vs. Config Struct**:
```go
// Config struct approach (avoided)
type Config struct {
    Host string
    Port int
    // ... many fields
}

// Requires all fields or complex initialization
db := New(Config{Host: "localhost", Port: 5432, ...})
```

**vs. Builder Pattern**:
```go
// Builder pattern (more verbose)
db := NewBuilder().
    Host("localhost").
    Port(5432).
    Build()
```

**Functional Options** (chosen):
```go
// Clean, flexible, backward compatible
db := New(
    WithHost("localhost"),
    WithPort(5432),
)
```

## Connection Management

### Connection Lifecycle

1. **Creation**: `New()` creates instance with configuration
2. **Connection**: `Connect()` establishes database connection
3. **Usage**: `DB()` provides access to underlying `*sql.DB`
4. **Health**: `HealthCheck()` verifies connection health
5. **Cleanup**: `Close()` gracefully closes connection

### Retry Logic

The package implements exponential backoff for connection retries:

```go
for attempt := 0; attempt <= maxRetries; attempt++ {
    // Try to connect
    if success {
        break
    }
    time.Sleep(retryInterval)
}
```

**Benefits**:
- Handles transient network issues
- Prevents connection storms
- Configurable retry count and interval

### Connection Pool

Leverages `database/sql` built-in connection pooling:

- **MaxOpenConns**: Maximum number of open connections (default: 25)
- **MaxIdleConns**: Maximum idle connections in pool (default: 5)
- **ConnMaxLifetime**: Maximum connection lifetime (default: 5m)
- **ConnMaxIdleTime**: Maximum idle time before closing (default: 10m)

**Why These Defaults?**

- 25 open connections: Suitable for most applications
- 5 idle connections: Balance between resource usage and latency
- 5 minute lifetime: Prevents stale connections
- 10 minute idle: Closes unused connections

## Error Handling

### Typed Errors

All errors are predefined as package-level variables:

```go
var (
    ErrAlreadyConnected = errors.New("database is already connected")
    ErrNotConnected     = errors.New("database is not connected")
    // ...
)
```

**Benefits**:
- Clients can check specific errors with `errors.Is()`
- Documentation through error names
- Compile-time safety
- No string comparison needed

### Error Wrapping

Internal errors are wrapped with context:

```go
return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
```

**Benefits**:
- Preserves error chain for `errors.Is()` and `errors.As()`
- Adds context without losing original error
- Better debugging and logging

## Thread Safety

The implementation is thread-safe because:

1. `*sql.DB` from `database/sql` is thread-safe
2. Configuration is immutable after creation
3. No shared mutable state

Multiple goroutines can safely use the same `Database` instance.

## Testing Strategy

### Unit Tests

- Test configuration options
- Test DSN building
- Test error conditions
- Test state validation

### Example Tests

- Demonstrate usage patterns
- Serve as documentation
- Validated by Go test framework

### Integration Tests (Future)

Could add integration tests with testcontainers:

```go
func TestIntegration(t *testing.T) {
    // Start PostgreSQL container
    // Run real database operations
    // Verify behavior
}
```

## Driver Choice: pgx vs lib/pq

### Why pgx?

1. **Active Maintenance**: pgx is actively maintained
2. **Performance**: Better performance than lib/pq
3. **Features**: Native support for PostgreSQL types
4. **Standards**: Implements database/sql interface
5. **Community**: Large community and ecosystem

### Using stdlib Driver

```go
import _ "github.com/jackc/pgx/v5/stdlib"
```

Uses pgx through the standard `database/sql` interface, providing:
- Portability
- Standard patterns
- Easy migration
- Familiar API

## Security Considerations

### Connection String

DSN is built programmatically to prevent injection:

```go
dsn := fmt.Sprintf(
    "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
    host, port, user, password, database, sslMode,
)
```

### SSL Support

Configurable SSL mode:
- `disable`: No SSL (development only)
- `prefer`: Try SSL, fall back to non-SSL
- `require`: Require SSL
- `verify-ca`: Verify server certificate
- `verify-full`: Full verification

### Credentials

- Passwords handled securely
- No credential logging
- Supports environment-based configuration

## Extensibility

### Adding Features

New features can be added without breaking changes:

1. Add configuration field to `config` struct
2. Add constant default value
3. Create functional option
4. Update `defaultConfig()`

Example:

```go
// 1. Add to config
type config struct {
    // ... existing fields
    schema string
}

// 2. Add default
const defaultSchema = "public"

// 3. Create option
func WithSchema(schema string) Option {
    return func(c *config) {
        if schema != "" {
            c.schema = schema
        }
    }
}

// 4. Update defaults
func defaultConfig() *config {
    return &config{
        // ... existing defaults
        schema: defaultSchema,
    }
}
```

### Multiple Database Support

The package can be instantiated multiple times for different databases:

```go
primaryDB := postgres.New(
    postgres.WithDatabase("primary"),
)

replicaDB := postgres.New(
    postgres.WithHost("replica.example.com"),
    postgres.WithDatabase("replica"),
)
```

## Performance Considerations

### Connection Pooling

- Reuses connections for better performance
- Reduces overhead of creating new connections
- Configurable based on workload

### Prepared Statements

Clients can use prepared statements through `DB()`:

```go
stmt, err := db.DB().PrepareContext(ctx, "SELECT * FROM users WHERE id = $1")
```

### Query Timeouts

Use context for query timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
rows, err := db.DB().QueryContext(ctx, query)
```

## Observability

### Metrics

Pool statistics available through `Stats()`:

```go
stats := db.DB().Stats()
fmt.Printf("Open: %d, Idle: %d, InUse: %d\n",
    stats.OpenConnections, stats.Idle, stats.InUse)
```

### Health Checks

Built-in health check for monitoring:

```go
if err := db.HealthCheck(ctx); err != nil {
    // Alert monitoring system
}
```

### Logging

Package doesn't implement logging to avoid dependency on specific logging libraries. Clients can log at the application level:

```go
if err := db.Connect(ctx); err != nil {
    logger.Error("database connection failed", "error", err)
}
```

## Future Enhancements

### Possible Additions

1. **Migrations**: Built-in migration support
2. **Read Replicas**: Automatic read/write splitting
3. **Query Builder**: Type-safe query building
4. **Hooks**: Pre/post connection hooks
5. **Observability**: OpenTelemetry integration
6. **Metrics**: Prometheus metrics
7. **Connection Events**: Connection lifecycle callbacks

### Backward Compatibility

All enhancements would maintain backward compatibility through:
- New optional interfaces
- Functional options
- Feature flags

## Comparison with Alternatives

### vs. GORM

**PostgreSQL Package** (Chosen):
- Lightweight and focused
- No ORM overhead
- Full SQL control
- Better performance

**GORM**:
- Full ORM features
- More abstractions
- Heavier dependencies
- Learning curve

### vs. sqlx

**PostgreSQL Package** (Chosen):
- Focused on PostgreSQL
- Simpler API
- Built-in health checks
- Retry logic included

**sqlx**:
- Database agnostic
- Named parameters
- Struct scanning
- More features

### vs. Raw database/sql

**PostgreSQL Package** (Chosen):
- Built on database/sql
- Adds connection management
- Health checks included
- Retry logic built-in
- Configuration management

**Raw database/sql**:
- Minimal
- More boilerplate
- Manual retry logic
- Manual health checks

## Conclusion

This package provides a production-ready PostgreSQL client that follows Go best practices and design principles. It's simple enough for small projects yet robust enough for large-scale applications.

The architecture prioritizes:
- **Simplicity**: Easy to understand and use
- **Reliability**: Robust error handling and retries
- **Performance**: Efficient connection pooling
- **Maintainability**: Clean code and SOLID principles
- **Extensibility**: Easy to extend without breaking changes
- **Testability**: Interface-based design for easy mocking
