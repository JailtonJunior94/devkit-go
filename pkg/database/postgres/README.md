# PostgreSQL Database Package

Package `postgres` provides a robust, production-ready PostgreSQL database client for Go applications.

## Features

- **Clean Architecture**: Interface-based design following SOLID principles
- **Functional Options Pattern**: Flexible and extensible configuration
- **Connection Pooling**: Configurable connection pool with sensible defaults
- **Health Checks**: Built-in health check support
- **Retry Logic**: Automatic retry on connection failures
- **Thread-Safe**: Safe for concurrent use
- **Type-Safe**: Strongly typed with clear error handling
- **Zero Dependencies**: Uses only standard library and pgx driver

## Installation

```bash
go get github.com/jackc/pgx/v5
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
)

func main() {
    db := postgres.New(
        postgres.WithHost("localhost"),
        postgres.WithPort(5432),
        postgres.WithUser("postgres"),
        postgres.WithPassword("postgres"),
        postgres.WithDatabase("myapp"),
    )

    ctx := context.Background()

    if err := db.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use the database
    sqlDB := db.DB()
    // ... perform queries
}
```

## Configuration Options

All configuration is done through functional options:

### Connection Settings

- `WithDSN(dsn string)` - Complete connection string (takes precedence over individual parameters)
- `WithHost(host string)` - Database host (default: "localhost")
- `WithPort(port int)` - Database port (default: 5432)
- `WithUser(user string)` - Database user (default: "postgres")
- `WithPassword(password string)` - Database password (default: "")
- `WithDatabase(database string)` - Database name (default: "postgres")
- `WithSSLMode(sslMode string)` - SSL mode (default: "disable")

### Connection Pool Settings

- `WithMaxOpenConns(n int)` - Maximum open connections (default: 25)
- `WithMaxIdleConns(n int)` - Maximum idle connections (default: 5)
- `WithConnMaxLifetime(d time.Duration)` - Connection max lifetime (default: 5m)
- `WithConnMaxIdleTime(d time.Duration)` - Connection max idle time (default: 10m)

### Timeout Settings

- `WithConnectTimeout(d time.Duration)` - Connection timeout (default: 10s)
- `WithPingTimeout(d time.Duration)` - Ping timeout (default: 5s)

### Retry Settings

- `WithMaxRetries(n int)` - Maximum retry attempts (default: 3)
- `WithRetryInterval(d time.Duration)` - Interval between retries (default: 2s)

## Examples

### Basic Connection

```go
db := postgres.New(
    postgres.WithHost("localhost"),
    postgres.WithDatabase("myapp"),
    postgres.WithUser("admin"),
    postgres.WithPassword("secret"),
)
```

### Using DSN String

```go
// Using PostgreSQL connection string
db := postgres.New(
    postgres.WithDSN("postgresql://admin:secret@localhost:5432/myapp?sslmode=disable"),
)

// Or from environment variable
db := postgres.New(
    postgres.WithDSN(os.Getenv("DATABASE_URL")),
)

// DSN takes precedence - other options are ignored
db := postgres.New(
    postgres.WithDSN("postgresql://admin:secret@localhost:5432/myapp"),
    postgres.WithHost("ignored"),  // This will be ignored
    postgres.WithPort(9999),       // This will be ignored
)
```

### Production Configuration

```go
db := postgres.New(
    postgres.WithHost("db.production.com"),
    postgres.WithPort(5432),
    postgres.WithUser("app_user"),
    postgres.WithPassword("secure_password"),
    postgres.WithDatabase("production_db"),
    postgres.WithSSLMode("require"),
    postgres.WithMaxOpenConns(100),
    postgres.WithMaxIdleConns(20),
    postgres.WithConnMaxLifetime(30*time.Minute),
    postgres.WithMaxRetries(5),
    postgres.WithRetryInterval(5*time.Second),
)
```

### Health Check

```go
ctx := context.Background()

if err := db.HealthCheck(ctx); err != nil {
    log.Printf("Database health check failed: %v", err)
}
```

### Using with Queries

```go
sqlDB := db.DB()

rows, err := sqlDB.QueryContext(ctx, "SELECT id, name FROM users WHERE active = $1", true)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    // Process row
}
```

## Architecture

### Interface

```go
type Database interface {
    Connect(ctx context.Context) error
    DB() *sql.DB
    HealthCheck(ctx context.Context) error
    Close() error
}
```

### Design Principles

1. **Single Responsibility**: Each component has a single, well-defined purpose
2. **Open/Closed**: Easily extensible through options without modifying core code
3. **Dependency Inversion**: Depends on abstractions (interfaces), not concretions
4. **Interface Segregation**: Clean, minimal interface
5. **DRY**: Configuration logic centralized and reusable

## Error Handling

The package provides typed errors for better error handling:

- `ErrAlreadyConnected` - Connection already established
- `ErrNotConnected` - No active connection
- `ErrConnectionFailed` - Failed to open connection
- `ErrPingFailed` - Failed to ping database
- `ErrHealthCheckFailed` - Health check failed
- `ErrCloseFailed` - Failed to close connection

Example:

```go
if err := db.Connect(ctx); err != nil {
    if errors.Is(err, postgres.ErrAlreadyConnected) {
        // Handle already connected
    }
    // Handle other errors
}
```

## Testing

The package includes comprehensive unit tests. Run tests with:

```bash
go test ./pkg/database/postgres/...
```

## Best Practices

1. **Always use context**: Pass context to all methods that accept it
2. **Defer Close()**: Always defer Close() after successful connection
3. **Health checks**: Implement regular health checks in production
4. **Connection pooling**: Tune pool settings based on your workload
5. **Error handling**: Always check and handle errors appropriately
6. **SSL in production**: Always use SSL in production environments

## Thread Safety

The database instance is safe for concurrent use. You can share a single instance across your application.

## Performance Considerations

- Default pool size (25 open, 5 idle) works well for most applications
- Increase pool size for high-throughput applications
- Tune `ConnMaxLifetime` based on your database server settings
- Use prepared statements for repeated queries
- Consider connection pooling at the application level for microservices

## License

This package is part of the devkit-go project.
