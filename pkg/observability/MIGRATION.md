# Migration Guide: o11y → observability

This guide helps you migrate from the old `o11y` package to the new `observability` package.

## Overview of Changes

The observability package has been completely redesigned with the following improvements:

- ✅ **Clean Architecture**: Fully decoupled with dependency inversion
- ✅ **Facade Pattern**: Single interface for all observability needs
- ✅ **Multiple Implementations**: `noop`, `fake`, and `otel` providers
- ✅ **Enhanced Security**: TLS configuration, sensitive data redaction, input validation
- ✅ **Better Testing**: Fake implementation with full assertion capabilities

## Breaking Changes

### 1. Package Import Path

```go
// BEFORE
import "github.com/JailtonJunior94/devkit-go/pkg/o11y"

// AFTER
import (
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)
```

### 2. Initialization

#### Before (old o11y)

```go
// Separate initialization for each component
logger, logShutdown, err := o11y.NewLogger(ctx, tracer, endpoint, serviceName, resource)
if err != nil {
    log.Fatal(err)
}
defer logShutdown(ctx)

metrics, metricsShutdown, err := o11y.NewMetrics(ctx, endpoint, serviceName, resource)
if err != nil {
    log.Fatal(err)
}
defer metricsShutdown(ctx)

tracer, tracerShutdown, err := o11y.NewTracer(ctx, endpoint, serviceName, resource)
if err != nil {
    log.Fatal(err)
}
defer tracerShutdown(ctx)
```

#### After (new observability)

```go
// Single provider initialization
config := &otel.Config{
    ServiceName:     "my-service",
    ServiceVersion:  "1.0.0",
    Environment:     "production",
    OTLPEndpoint:    "otel-collector:4317",
    OTLPProtocol:    otel.ProtocolGRPC,

    // Security configuration (NEW!)
    Insecure:        false, // Use TLS by default
    TLSConfig:       nil,   // Uses system default CAs

    TraceSampleRate: 1.0,
    LogLevel:        observability.LogLevelInfo,
    LogFormat:       observability.LogFormatJSON,
}

obs, err := otel.NewProvider(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer obs.Shutdown(ctx) // Single shutdown for all components
```

### 3. Logger API Changes

#### Error Method Signature

```go
// BEFORE
logger.Error(ctx, err, "failed to process request",
    o11y.Field{Key: "user_id", Value: userID})

// AFTER
logger.Error(ctx, "failed to process request",
    observability.Error(err),
    observability.String("user_id", userID))
```

#### Field Creation

```go
// BEFORE
o11y.Field{Key: "user_id", Value: userID}
o11y.Field{Key: "count", Value: count}

// AFTER - Use type-safe helper functions
observability.String("user_id", userID)
observability.Int("count", count)
observability.Int64("timestamp", timestamp)
observability.Float64("latency", latency)
observability.Bool("success", true)
observability.Error(err)
observability.Any("custom", customValue)
```

### 4. Metrics API Changes

#### Before (old o11y)

```go
// Direct value recording
metrics.AddCounter(ctx, "http.requests", 1, "method", "GET", "status", "200")
metrics.RecordHistogram(ctx, "http.duration", 0.123, "endpoint", "/api/users")
```

#### After (new observability)

```go
// Instrument creation + recording
counter := obs.Metrics().Counter(
    "http.requests",
    "Total HTTP requests",
    "1",
)
counter.Add(ctx, 1,
    observability.String("method", "GET"),
    observability.String("status", "200"))

histogram := obs.Metrics().Histogram(
    "http.duration",
    "HTTP request duration",
    "s",
)
histogram.Record(ctx, 0.123,
    observability.String("endpoint", "/api/users"))
```

### 5. Dependency Injection

#### Before (old o11y)

```go
type UserService struct {
    logger  o11y.Logger
    metrics o11y.Metrics
    tracer  o11y.Tracer
}

func NewUserService(logger o11y.Logger, metrics o11y.Metrics, tracer o11y.Tracer) *UserService {
    return &UserService{
        logger:  logger,
        metrics: metrics,
        tracer:  tracer,
    }
}
```

#### After (new observability)

```go
type UserService struct {
    obs observability.Observability // Single dependency!
}

func NewUserService(obs observability.Observability) *UserService {
    return &UserService{obs: obs}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    // Access components through facade
    ctx, span := s.obs.Tracer().Start(ctx, "GetUser")
    defer span.End()

    s.obs.Logger().Info(ctx, "fetching user", observability.String("user_id", id))

    counter := s.obs.Metrics().Counter("users.fetched", "Users fetched", "1")
    counter.Increment(ctx)

    // ... implementation
}
```

## Security Improvements

### 1. TLS Configuration

```go
// Development (insecure)
config := &otel.Config{
    Environment:  "development",
    OTLPEndpoint: "localhost:4317",
    Insecure:     true, // Only allowed in non-production
}

// Production (secure by default)
config := &otel.Config{
    Environment:  "production",
    OTLPEndpoint: "otel-collector.prod:4317",
    // Insecure: true would return an error!
    // Uses system default TLS automatically
}

// Production with custom TLS
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    // ... custom configuration
}

config := &otel.Config{
    Environment:  "production",
    OTLPEndpoint: "otel-collector.prod:4317",
    TLSConfig:    tlsConfig,
}
```

### 2. Sensitive Data Redaction

The new logger automatically redacts sensitive field values:

```go
// These fields will be automatically redacted
logger.Info(ctx, "user login",
    observability.String("password", "secret123"),        // → [REDACTED]
    observability.String("api_key", "sk_live_123"),       // → [REDACTED]
    observability.String("authorization", "Bearer xyz"),  // → [REDACTED]
    observability.String("username", "john"),             // → "john" (safe)
)
```

Default sensitive keys: `password`, `passwd`, `secret`, `token`, `api_key`, `authorization`, `credential`, `private_key`, `ssn`, `credit_card`, `cvv`, `access_token`, `refresh_token`, `session`, `cookie`.

### 3. Input Validation

- Maximum 50 fields per log entry (prevents cardinality explosion)
- Maximum 2048 characters per field value (automatic truncation)
- Empty messages replaced with `[empty message]`

## Testing

### Using Fake Implementation

```go
import "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

func TestUserService(t *testing.T) {
    // Create fake provider
    fakeObs := fake.NewProvider()

    service := NewUserService(fakeObs)

    // Run test
    err := service.CreateUser(ctx, userData)
    require.NoError(t, err)

    // Assert logs
    logs := fakeObs.Logger().(*fake.FakeLogger).GetEntries()
    assert.Len(t, logs, 1)
    assert.Equal(t, "user created", logs[0].Message)

    // Assert metrics
    counter := fakeObs.Metrics().(*fake.FakeMetrics).GetCounter("users.created")
    assert.NotNil(t, counter)
    values := counter.GetValues()
    assert.Equal(t, int64(1), values[0].Value)

    // Assert traces
    spans := fakeObs.Tracer().(*fake.FakeTracer).GetSpans()
    assert.Len(t, spans, 1)
    assert.Equal(t, "CreateUser", spans[0].Name)
}
```

### Using NoOp Implementation

```go
import "github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

// For benchmarks or when observability is not needed
func BenchmarkUserService(b *testing.B) {
    noopObs := noop.NewProvider()
    service := NewUserService(noopObs)

    // Zero overhead from observability
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.CreateUser(ctx, userData)
    }
}
```

## Migration Checklist

- [ ] Update import paths from `o11y` to `observability`
- [ ] Replace separate Logger/Metrics/Tracer initialization with single Provider
- [ ] Update Logger.Error() calls to use observability.Error()
- [ ] Replace Field literals with type-safe helper functions
- [ ] Update Metrics calls to use instrument pattern (Counter/Histogram)
- [ ] Update dependency injection to use single Observability interface
- [ ] Configure TLS properly for production environments
- [ ] Update tests to use fake.NewProvider()
- [ ] Remove old o11y shutdown functions (now single Shutdown())
- [ ] Verify no sensitive data leaks in logs

## Rollback Plan

If you need to temporarily rollback:

1. The old `o11y` package code was removed in this commit
2. Revert this commit to restore the old implementation
3. However, we recommend fixing forward rather than rolling back

## Support

For questions or issues:
- Check examples in `pkg/observability/examples/`
- Review tests in `pkg/observability/*/test.go`
- Open an issue on GitHub

## Version Compatibility

- Old package: `pkg/o11y` (deprecated, removed)
- New package: `pkg/observability` (v2.0.0+)
- Minimum Go version: 1.21+
- OpenTelemetry SDK: v1.39.0+
