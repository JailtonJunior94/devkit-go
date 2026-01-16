# observability

Production-grade observability toolkit with OpenTelemetry integration for distributed tracing, metrics, and structured logging.

## Introduction

### Problem It Solves

Modern distributed systems require comprehensive observability to:
- Track requests across multiple services (distributed tracing)
- Monitor system health and performance (metrics)
- Debug issues with structured, contextual logging
- Correlate logs, traces, and metrics for faster troubleshooting

Without a unified observability approach, debugging production issues becomes nearly impossible as complexity grows.

### When to Use

✅ **Use this package when:**
- Building production services that need observability
- Integrating with OpenTelemetry-compatible backends (Jaeger, Tempo, Prometheus, Grafana)
- Need vendor-neutral instrumentation
- Want automatic trace context propagation across logs
- Require consistent observability across multiple services

❌ **Don't use when:**
- Building simple CLI tools or scripts (use standard library logging)
- No observability backend available (use `noop` provider instead)
- Performance overhead cannot be tolerated (use `noop` provider)

---

## Architecture

### Core Design

The package follows the **Facade Pattern** with three pillars:

```
Observability (Facade)
    ├── Tracer    (Distributed tracing)
    ├── Logger    (Structured logging with trace context)
    └── Metrics   (Application metrics)
```

### Provider Implementations

| Provider | Use Case | Overhead |
|----------|----------|----------|
| **otel** | Production with OpenTelemetry backend | Normal |
| **fake** | Unit/integration testing | Low |
| **noop** | Production without observability | Zero |

### Key Architectural Decisions

1. **Unified Interface**: Single `Observability` facade provides access to all features
2. **Context-First**: All operations require `context.Context` for trace propagation
3. **Immutable Configuration**: Providers are configured at creation, not runtime
4. **Thread-Safe**: All implementations are safe for concurrent use
5. **Field-Based**: Structured data uses strongly-typed `Field` instead of `map[string]any`

### Dependencies

- Core package is interface-only (no external dependencies)
- `otel` provider requires OpenTelemetry SDKs
- `fake` and `noop` have no external dependencies

---

## API Reference

### Core Interface

```go
type Observability interface {
    Tracer() Tracer
    Logger() Logger
    Metrics() Metrics
}
```

### Tracer Interface

```go
type Tracer interface {
    Start(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, Span)
    SpanFromContext(ctx context.Context) Span
    ContextWithSpan(ctx context.Context, span Span) context.Context
}

type Span interface {
    End()
    SetAttributes(fields ...Field)
    SetStatus(code StatusCode, description string)
    RecordError(err error, fields ...Field)
    AddEvent(name string, fields ...Field)
    Context() SpanContext
}
```

**Span Options:**
- `WithSpanKind(kind SpanKind)` - Set span kind (Internal, Server, Client, Producer, Consumer)
- `WithAttributes(fields ...Field)` - Set initial attributes

### Logger Interface

```go
type Logger interface {
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    With(fields ...Field) Logger
}
```

**Log Levels:** Debug, Info, Warn, Error
**Log Formats:** Text, JSON

### Metrics Interface

```go
type Metrics interface {
    Counter(name, description, unit string) Counter
    Histogram(name, description, unit string) Histogram
    UpDownCounter(name, description, unit string) UpDownCounter
    Gauge(name, description, unit string, callback GaugeCallback) error
}
```

**Metric Types:**
- **Counter**: Monotonically increasing (requests, errors)
- **Histogram**: Value distribution (latencies, sizes)
- **UpDownCounter**: Can increase/decrease (active connections, queue size)
- **Gauge**: Current value snapshot (CPU usage, memory)

### Field Constructors

```go
String(key, value string) Field
Int(key string, value int) Field
Int64(key string, value int64) Field
Float64(key string, value float64) Field
Bool(key string, value bool) Field
Error(err error) Field
Any(key string, value any) Field
```

---

## Examples

### Basic Usage with OpenTelemetry

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
        ServiceName:    "order-service",
        ServiceVersion: "1.2.0",
        Environment:    "production",
        OTLPEndpoint:   "otel-collector:4317",
        OTLPProtocol:   otel.ProtocolGRPC,
    }

    provider, err := otel.NewProvider(ctx, config)
    if err != nil {
        panic(err)
    }
    defer provider.Shutdown(ctx)

    // Use observability
    processOrder(ctx, provider)
}

func processOrder(ctx context.Context, obs observability.Observability) {
    logger := obs.Logger()
    tracer := obs.Tracer()
    metrics := obs.Metrics()

    // Start a span
    ctx, span := tracer.Start(ctx, "process_order")
    defer span.End()

    span.SetAttributes(
        observability.String("order_id", "12345"),
        observability.Int("items", 3),
    )

    // Log with trace context (automatically includes trace_id and span_id)
    logger.Info(ctx, "processing order",
        observability.String("order_id", "12345"),
    )

    // Record metrics
    counter := metrics.Counter("orders.processed", "Total orders processed", "1")
    counter.Increment(ctx, observability.String("status", "success"))

    latency := metrics.Histogram("order.latency", "Order processing time", "ms")
    latency.Record(ctx, 123.45)
}
```

### Nested Spans for Distributed Tracing

```go
func processOrder(ctx context.Context, obs observability.Observability) error {
    tracer := obs.Tracer()
    logger := obs.Logger()

    // Parent span
    ctx, span := tracer.Start(ctx, "process_order")
    defer span.End()

    // Child span 1: Validate
    if err := validateOrder(ctx, tracer, logger); err != nil {
        span.RecordError(err)
        span.SetStatus(observability.StatusCodeError, "validation failed")
        return err
    }

    // Child span 2: Save to DB
    if err := saveOrder(ctx, tracer); err != nil {
        span.RecordError(err)
        span.SetStatus(observability.StatusCodeError, "save failed")
        return err
    }

    span.SetStatus(observability.StatusCodeOK, "")
    return nil
}

func validateOrder(ctx context.Context, tracer observability.Tracer, logger observability.Logger) error {
    ctx, span := tracer.Start(ctx, "validate_order")
    defer span.End()

    logger.Debug(ctx, "validating order")

    // Validation logic...
    span.AddEvent("validation_complete")
    return nil
}

func saveOrder(ctx context.Context, tracer observability.Tracer) error {
    ctx, span := tracer.Start(ctx, "save_order",
        observability.WithSpanKind(observability.SpanKindClient),
    )
    defer span.End()

    // Database call...
    return nil
}
```

### Logger with Permanent Fields

```go
func NewUserService(obs observability.Observability) *UserService {
    // Create logger with permanent fields
    logger := obs.Logger().With(
        observability.String("service", "user-service"),
        observability.String("component", "user-handler"),
    )

    return &UserService{
        logger: logger,
        tracer: obs.Tracer(),
    }
}

func (s *UserService) CreateUser(ctx context.Context, name string) error {
    // All logs will include service and component fields
    s.logger.Info(ctx, "creating user", observability.String("name", name))

    // Child logger with additional fields
    reqLogger := s.logger.With(observability.String("request_id", "abc123"))
    reqLogger.Debug(ctx, "validating user data")

    return nil
}
```

### Metrics: All Types

```go
func setupMetrics(metrics observability.Metrics) {
    // Counter: Monotonically increasing
    requestCounter := metrics.Counter(
        "http.requests.total",
        "Total HTTP requests",
        "1",
    )

    // Histogram: Value distribution
    latencyHistogram := metrics.Histogram(
        "http.request.duration",
        "HTTP request latency",
        "ms",
    )

    // UpDownCounter: Can increase/decrease
    activeConnections := metrics.UpDownCounter(
        "connections.active",
        "Active connections",
        "1",
    )

    // Gauge: Current value (asynchronous)
    _ = metrics.Gauge(
        "memory.usage",
        "Current memory usage",
        "bytes",
        func(ctx context.Context) float64 {
            // Return current memory usage
            return getCurrentMemoryUsage()
        },
    )

    // Use metrics
    ctx := context.Background()
    requestCounter.Increment(ctx, observability.String("method", "GET"))
    latencyHistogram.Record(ctx, 45.2)
    activeConnections.Add(ctx, 1)  // Connection opened
    activeConnections.Add(ctx, -1) // Connection closed
}
```

### Testing with Fake Provider

```go
func TestUserService(t *testing.T) {
    // Use fake provider for testing
    provider := fake.NewProvider()

    service := NewUserService(provider)
    err := service.CreateUser(context.Background(), "John Doe")

    require.NoError(t, err)

    // Assert on captured logs
    fakeLogger := provider.Logger().(*fake.FakeLogger)
    entries := fakeLogger.GetEntries()

    assert.Len(t, entries, 1)
    assert.Equal(t, "creating user", entries[0].Message)
    assert.Equal(t, observability.LogLevelInfo, entries[0].Level)

    // Assert on captured spans
    fakeTracer := provider.Tracer().(*fake.FakeTracer)
    spans := fakeTracer.GetSpans()

    assert.Len(t, spans, 1)
    assert.Equal(t, "create_user", spans[0].Name)
}
```

### No-Op Provider (Zero Overhead)

```go
func main() {
    var provider observability.Observability

    if enableObservability {
        provider, _ = otel.NewProvider(ctx, config)
    } else {
        // Zero overhead when observability is disabled
        provider = noop.NewProvider()
    }

    // Code works identically, but noop has zero cost
    logger := provider.Logger()
    logger.Info(ctx, "message") // No-op, zero allocation
}
```

---

## Best Practices

### 1. Always Pass Context

```go
// ✅ Good: Context enables trace propagation
logger.Info(ctx, "user created", observability.String("user_id", id))

// ❌ Bad: Missing context breaks trace correlation
logger.Info(context.Background(), "user created")
```

### 2. Close Spans with defer

```go
// ✅ Good: Span always ends, even on panic
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// ❌ Bad: Span may not end if function panics
ctx, span := tracer.Start(ctx, "operation")
// ... work ...
span.End()
```

### 3. Set Span Status on Errors

```go
// ✅ Good: Mark span as error and record details
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

if err := doWork(); err != nil {
    span.RecordError(err)
    span.SetStatus(observability.StatusCodeError, "operation failed")
    return err
}

span.SetStatus(observability.StatusCodeOK, "")
```

### 4. Use Structured Fields, Not String Concatenation

```go
// ✅ Good: Structured, queryable
logger.Info(ctx, "user logged in",
    observability.String("user_id", userID),
    observability.String("ip", ip),
)

// ❌ Bad: Unstructured, hard to query
logger.Info(ctx, fmt.Sprintf("user %s logged in from %s", userID, ip))
```

### 5. Reuse Metric Instruments

```go
// ✅ Good: Create once, reuse many times
type Service struct {
    requestCounter observability.Counter
}

func NewService(metrics observability.Metrics) *Service {
    return &Service{
        requestCounter: metrics.Counter("requests", "Total requests", "1"),
    }
}

func (s *Service) HandleRequest(ctx context.Context) {
    s.requestCounter.Increment(ctx)
}

// ❌ Bad: Creating metric on every call (inefficient)
func (s *Service) HandleRequest(ctx context.Context, metrics observability.Metrics) {
    metrics.Counter("requests", "Total requests", "1").Increment(ctx)
}
```

---

## Caveats and Limitations

### Context Propagation

**Caveat:** Trace context is stored in `context.Context`. If you don't pass context correctly, traces will be disconnected.

```go
// ❌ Bad: Creates new root span (loses parent context)
go func() {
    ctx, span := tracer.Start(context.Background(), "async-work")
    defer span.End()
}()

// ✅ Good: Preserves parent span
go func(ctx context.Context) {
    ctx, span := tracer.Start(ctx, "async-work")
    defer span.End()
}(ctx)
```

### Logger Field Immutability

**Caveat:** `With()` creates a new logger. The original logger is unchanged.

```go
logger := obs.Logger()
logger.With(observability.String("key", "value"))

// ❌ logger still doesn't have "key" field
logger.Info(ctx, "message")

// ✅ Use the returned logger
logger = logger.With(observability.String("key", "value"))
logger.Info(ctx, "message")
```

### Span Lifetime

**Limitation:** Spans must be ended. Forgotten spans can cause memory leaks in long-running processes.

```go
// ✅ Always use defer
ctx, span := tracer.Start(ctx, "operation")
defer span.End()
```

### Performance

**Consideration:** OpenTelemetry has overhead (~microseconds per span). For ultra-high-throughput systems:
- Use sampling (only trace 1-10% of requests)
- Use `noop` provider in performance-critical paths
- Avoid creating spans in tight loops

### Configuration Immutability

**Limitation:** Once a provider is created, its configuration cannot be changed. To reconfigure:

```go
// Must recreate provider
provider.Shutdown(ctx)
provider, _ = otel.NewProvider(ctx, newConfig)
```

---

## Configuration Reference (OTel Provider)

```go
type Config struct {
    ServiceName    string       // Required: Service identifier
    ServiceVersion string       // Recommended: Semantic version
    Environment    string       // dev/staging/production
    OTLPEndpoint   string       // Required: Collector endpoint (host:port)
    OTLPProtocol   OTLPProtocol // grpc (default) or http

    // Security
    Insecure  bool        // Allow insecure connections (dev only)
    TLSConfig *tls.Config // Custom TLS config (optional)

    // Tracing
    TraceSampleRate float64 // 0.0-1.0 (default 1.0 = always sample)

    // Logging
    LogLevel  LogLevel  // debug/info/warn/error
    LogFormat LogFormat // text or json

    // Resource Attributes (optional)
    ResourceAttributes map[string]string
}
```

---

## Thread Safety

All providers and their components (Tracer, Logger, Metrics) are **fully thread-safe** and can be shared across goroutines.

---

## Related Packages

- `pkg/observability/otel` - OpenTelemetry implementation
- `pkg/observability/fake` - Testing provider
- `pkg/observability/noop` - Zero-overhead provider
- `pkg/httpserver` - Includes observability middleware
- `pkg/httpclient` - Includes automatic instrumentation
- `pkg/messaging` - Kafka/RabbitMQ with distributed tracing
