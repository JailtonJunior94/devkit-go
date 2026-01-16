# devkit-go

A comprehensive, production-ready Go toolkit for building robust backend applications with observability, messaging, database management, and HTTP services.

## Overview

`devkit-go` is a battle-tested collection of Go packages designed to accelerate backend development by providing well-architected, reusable components that follow Go best practices and idioms. Built with observability-first principles, it seamlessly integrates OpenTelemetry for distributed tracing, metrics, and structured logging.

**Project Status:** Active Development
**Go Version:** 1.25+
**License:** MIT (check repository for details)

---

## Key Features

✅ **Observability** - Full OpenTelemetry integration (traces, metrics, logs) with noop and fake providers for testing
✅ **Database** - PostgreSQL support with connection pooling, Unit of Work pattern, and instrumentation
✅ **Messaging** - Kafka and RabbitMQ publishers/consumers with automatic retries, DLQ, and tracing
✅ **HTTP** - Chi and Fiber-based servers with graceful shutdown, middleware support, and observability
✅ **Migrations** - Safe database migrations with multiple driver support (Postgres, MySQL, CockroachDB)
✅ **Events** - In-process event dispatcher for domain events and pub/sub patterns
✅ **Value Objects** - DDD-compliant VOs for Money, Percentage, UUID, ULID, and nullable types
✅ **Type-safe** - LINQ-style operations on slices with generics
✅ **Testing** - Fake providers and test utilities for comprehensive testing

---

## Installation

```bash
go get github.com/JailtonJunior94/devkit-go
```

## Quick Start

### Observability with OpenTelemetry

```go
package main

import (
    "context"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

func main() {
    ctx := context.Background()

    // Initialize OpenTelemetry provider
    config := &otel.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        OTLPEndpoint:   "localhost:4317",
        OTLPProtocol:   otel.ProtocolGRPC,
    }

    provider, err := otel.NewProvider(ctx, config)
    if err != nil {
        panic(err)
    }
    defer provider.Shutdown(ctx)

    // Use tracer, logger, and metrics
    tracer := provider.Tracer()
    logger := provider.Logger()
    metrics := provider.Metrics()

    ctx, span := tracer.Start(ctx, "main.operation")
    defer span.End()

    logger.Info(ctx, "application started", observability.String("version", "1.0.0"))

    counter := metrics.Counter("requests.total", "Total requests", "1")
    counter.Increment(ctx)
}
```

### HTTP Server with Chi

```go
package main

import (
    "context"
    "net/http"
    "github.com/JailtonJunior94/devkit-go/pkg/httpserver"
)

func main() {
    server := httpserver.New(
        httpserver.WithPort("8080"),
    )

    server.RegisterRoute(httpserver.Route{
        Path:   "/health",
        Method: http.MethodGet,
        Handler: func(w http.ResponseWriter, r *http.Request) error {
            w.WriteHeader(http.StatusOK)
            return nil
        },
    })

    shutdown := server.Run()
    defer shutdown(context.Background())

    // Block until shutdown
    select {}
}
```

### Database with Unit of Work

```go
package main

import (
    "context"
    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
    "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
)

func main() {
    ctx := context.Background()

    // Initialize database
    dbManager, err := postgres.NewDatabaseManager(ctx, &postgres.Config{
        DSN: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
    })
    if err != nil {
        panic(err)
    }
    defer dbManager.Close()

    // Use Unit of Work for transactions
    uowInstance := uow.NewUnitOfWork(dbManager.DB())

    err = uowInstance.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
        // All operations here run in a transaction
        // Automatic rollback on error, commit on success
        _, err := tx.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
        return err
    })
}
```

### Value Objects

```go
package main

import (
    "github.com/JailtonJunior94/devkit-go/pkg/vos"
)

func main() {
    // Money with currency
    price, _ := vos.NewMoney(1050, vos.CurrencyBRL) // 10.50 BRL
    discount, _ := vos.NewMoney(200, vos.CurrencyBRL) // 2.00 BRL
    total, _ := price.Subtract(discount) // 8.50 BRL

    // Percentage calculations
    taxRate, _ := vos.NewPercentageFromFloat(10.0) // 10%
    tax, _ := taxRate.Apply(total) // 0.85 BRL

    // UUID and ULID
    id, _ := vos.NewUUID()
    ulid, _ := vos.NewULID()

    // Nullable types
    name := vos.NewNullableString("John Doe")
    age := vos.NewNullableInt(30)
}
```

---

## Package Structure

```
pkg/
├── database/               # Database management and patterns
│   ├── pgxpool_manager/   # PostgreSQL connection pooling
│   ├── postgres/          # PostgreSQL database manager
│   ├── postgres_otelsql/  # PostgreSQL with OTel instrumentation
│   └── uow/               # Unit of Work pattern for transactions
├── encrypt/               # Password hashing (bcrypt)
├── entity/                # Base entity for DDD entities
├── events/                # In-process event dispatcher
├── http_server/           # HTTP server implementations
│   ├── chi_server/        # Chi router integration
│   └── server_fiber/      # Fiber framework integration
├── httpclient/            # HTTP client with retry and instrumentation
├── httpserver/            # Production-ready HTTP server (Chi-based)
├── linq/                  # LINQ-style operations for slices
├── logger/                # Logger interface and Zap implementation
├── messaging/             # Message brokers
│   ├── kafka/             # Kafka producer/consumer
│   └── rabbitmq/          # RabbitMQ publisher/consumer
├── migration/             # Database migration tool
├── observability/         # OpenTelemetry integration
│   ├── fake/              # Fake provider for testing
│   ├── noop/              # No-op provider (zero overhead)
│   └── otel/              # OpenTelemetry implementation
├── responses/             # HTTP response helpers
└── vos/                   # Value Objects (Money, UUID, etc.)
```

---

## Core Packages

### 🔭 Observability (`pkg/observability`)

Unified observability with OpenTelemetry support for traces, metrics, and structured logging. Includes providers for production (OTel), testing (fake), and no-op (zero overhead).

**[→ Full Documentation](pkg/observability/README.md)**

### 💾 Database (`pkg/database`)

PostgreSQL integration with connection pooling, Unit of Work pattern, and OpenTelemetry instrumentation. Supports transactional operations and repository patterns.

**[→ Full Documentation](pkg/database/README.md)**

### 📨 Messaging (`pkg/messaging`)

Production-ready Kafka and RabbitMQ integrations with automatic retries, dead-letter queues, distributed tracing, and graceful shutdown.

**[→ Full Documentation](pkg/messaging/README.md)**

### 🌐 HTTP Server (`pkg/httpserver`)

Chi-based HTTP server with graceful shutdown, middleware support, error handling, and built-in observability hooks.

**[→ Full Documentation](pkg/httpserver/README.md)**

### 🔄 Migrations (`pkg/migration`)

Safe database migration management supporting Postgres, MySQL, and CockroachDB. Built on golang-migrate with enhanced error handling and logging.

**[→ Full Documentation](pkg/migration/README.md)**

### 💰 Value Objects (`pkg/vos`)

Domain-Driven Design value objects for Money (with currency), Percentage, UUID, ULID, and nullable types. Immutable, thread-safe, with precision guarantees.

**[→ Full Documentation](pkg/vos/README.md)**

### 🎯 Events (`pkg/events`)

Thread-safe, in-process event dispatcher for domain events and pub/sub patterns within your application.

**[→ Full Documentation](pkg/events/README.md)**

### 🔐 Encrypt (`pkg/encrypt`)

Secure password hashing using bcrypt with configurable cost factors and timing-attack resistance.

**[→ Full Documentation](pkg/encrypt/README.md)**

---

## Best Practices

### 1. Observability First

Always initialize observability at application startup:

```go
provider, _ := otel.NewProvider(ctx, config)
defer provider.Shutdown(ctx)

// Pass to dependencies
logger := provider.Logger()
tracer := provider.Tracer()
```

### 2. Use Unit of Work for Transactions

Never manage transactions manually. Use the UoW pattern:

```go
err := uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
    // Transactional operations
    return repo.Save(ctx, tx, entity)
})
```

### 3. Structured Logging with Context

Always pass context and use structured fields:

```go
logger.Info(ctx, "user created",
    observability.String("user_id", id),
    observability.Int("age", 30),
)
```

### 4. Value Objects for Domain Logic

Use VOs instead of primitives for domain concepts:

```go
// Bad
price := 1050 // What unit? Cents? Dollars?

// Good
price, _ := vos.NewMoney(1050, vos.CurrencyBRL) // Clear: 10.50 BRL
```

### 5. Graceful Shutdown

Always handle graceful shutdown properly:

```go
shutdown := server.Run()
defer shutdown(context.Background())

// Or with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
shutdown(ctx)
```

---

## Testing

### Using Fake Providers

```go
func TestService(t *testing.T) {
    provider := fake.NewProvider()
    logger := provider.Logger()
    tracer := provider.Tracer()

    service := NewService(logger, tracer)
    service.DoSomething()

    // Assert on captured logs
    entries := provider.Logger().(*fake.FakeLogger).GetEntries()
    assert.Len(t, entries, 1)
    assert.Equal(t, "operation completed", entries[0].Message)
}
```

### Testcontainers Support

The project includes testcontainers integration for integration tests (see tests in `pkg/database`, `pkg/messaging`).

---

## Requirements

- **Go:** 1.25 or higher
- **PostgreSQL:** 12+ (for database packages)
- **Kafka:** 2.8+ (for messaging/kafka)
- **RabbitMQ:** 3.9+ (for messaging/rabbitmq)

---

## Contributing

Contributions are welcome! Please ensure:

1. Code follows Go idioms and best practices
2. All tests pass (`go test ./...`)
3. New features include tests and documentation
4. Changes maintain backward compatibility

---

## Architecture Decisions

### Why OpenTelemetry?

OpenTelemetry is the industry standard for observability, providing vendor-neutral instrumentation with support for all major observability platforms.

### Why Unit of Work?

The UoW pattern simplifies transaction management, ensures consistency, and prevents common pitfalls like forgetting to commit or rollback.

### Why Value Objects?

VOs enforce domain invariants, prevent primitive obsession, and make business logic explicit and type-safe.

---

## License

MIT License - see repository for full details.

---

## Support

For issues, questions, or contributions, visit the [GitHub repository](https://github.com/JailtonJunior94/devkit-go).
