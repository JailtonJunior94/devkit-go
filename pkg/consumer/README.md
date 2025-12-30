# Consumer Package

A production-ready message consumer server with lifecycle management, graceful shutdown, health checks, and observability integration. Follows the same architectural patterns as `pkg/http_server`.

## Features

✅ **Lifecycle Management** - Explicit `Start()` and `Shutdown()` methods  
✅ **Graceful Shutdown** - Waits for in-flight messages with configurable timeout  
✅ **Signal Handling** - Automatic SIGINT/SIGTERM handling  
✅ **Worker Pool** - Concurrent message processing with configurable workers  
✅ **Health Checks** - Built-in health checks with custom check support  
✅ **Middleware Chain** - Composable middleware for logging, metrics, tracing  
✅ **Retry Logic** - Exponential backoff with configurable max retries  
✅ **Dead Letter Queue** - Failed message handling with DLQ support  
✅ **Observability** - Full integration with pkg/observability  
✅ **Context-Aware** - Respects context cancellation throughout  
✅ **Production-Ready** - Sensible defaults, validation, error handling  

## Architecture

```
pkg/consumer/
├── consumer.go      # Main Consumer interface and Server struct
├── config.go        # Configuration with validation and defaults
├── options.go       # Functional options pattern
├── lifecycle.go     # Start() and Shutdown() with signal handling
├── health.go        # Health check implementation
├── handler.go       # Message handler registry and dispatch
├── middleware.go    # Message middleware chain
├── errors.go        # Consumer-specific error types
└── doc.go           # Package documentation
```

## Quick Start

### Basic Example

```go
package main

import (
    "context"
    "log"

    "github.com/jailtonjunior/devkit-go/pkg/consumer"
    "github.com/jailtonjunior/devkit-go/pkg/observability"
)

func main() {
    // Initialize observability
    o11y, err := observability.New(
        observability.WithServiceName("my-service"),
        observability.WithServiceVersion("1.0.0"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create consumer
    c := consumer.New(o11y,
        consumer.WithServiceName("my-service"),
        consumer.WithServiceVersion("1.0.0"),
        consumer.WithEnvironment("production"),
        consumer.WithTopics("orders", "payments"),
        consumer.WithWorkerCount(10),
    )

    // Register handlers
    c.RegisterHandlers(
        consumer.NewFuncHandler([]string{"orders"}, handleOrder),
        consumer.NewFuncHandler([]string{"payments"}, handlePayment),
    )

    // Start consumer (blocks until shutdown)
    if err := c.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func handleOrder(ctx context.Context, msg *consumer.Message) error {
    // Process order message
    log.Printf("Processing order: %s", string(msg.Value))
    return nil
}

func handlePayment(ctx context.Context, msg *consumer.Message) error {
    // Process payment message
    log.Printf("Processing payment: %s", string(msg.Value))
    return nil
}
```

## Configuration

### Using Functional Options

```go
c := consumer.New(o11y,
    consumer.WithServiceName("my-service"),
    consumer.WithServiceVersion("1.0.0"),
    consumer.WithEnvironment("production"),
    consumer.WithTopics("orders", "payments"),
    consumer.WithWorkerCount(10),
    consumer.WithBatchSize(100),
    consumer.WithProcessingTimeout(30*time.Second),
    consumer.WithShutdownTimeout(30*time.Second),
    consumer.WithMaxRetries(3),
    consumer.WithRetryBackoff(1*time.Second),
    consumer.WithDLQ("orders-dlq"),
)
```

### Using Config Struct

```go
config := consumer.Config{
    ServiceName:       "my-service",
    ServiceVersion:    "1.0.0",
    Environment:       "production",
    Topics:            []string{"orders", "payments"},
    WorkerCount:       10,
    BatchSize:         100,
    ProcessingTimeout: 30 * time.Second,
    ShutdownTimeout:   30 * time.Second,
    MaxRetries:        3,
    RetryBackoff:      1 * time.Second,
    EnableDLQ:         true,
    DLQTopic:          "orders-dlq",
}

c := consumer.New(o11y, consumer.WithConfig(config))
```

### Default Configuration

```go
// DefaultConfig() provides sensible defaults:
{
    WorkerCount:        5,
    BatchSize:          10,
    ProcessingTimeout:  30 * time.Second,
    ShutdownTimeout:    30 * time.Second,
    CommitInterval:     5 * time.Second,
    MaxRetries:         3,
    RetryBackoff:       1 * time.Second,
    EnableHealthChecks: true,
    EnableMetrics:      true,
    EnableDLQ:          false,
}
```

## Message Handlers

### Function Handler

```go
handler := consumer.NewFuncHandler(
    []string{"orders"},
    func(ctx context.Context, msg *consumer.Message) error {
        // Process message
        return nil
    },
)

c.RegisterHandlers(handler)
```

### Custom Handler

```go
type OrderHandler struct {
    db Database
}

func (h *OrderHandler) Handle(ctx context.Context, msg *consumer.Message) error {
    var order Order
    if err := json.Unmarshal(msg.Value, &order); err != nil {
        return err
    }

    return h.db.SaveOrder(ctx, order)
}

// Register
handler := consumer.NewTopicHandler(&OrderHandler{db: db}, "orders")
c.RegisterHandlers(handler)
```

### Batch Processing

```go
processor := consumer.NewBatchProcessor(100, func(ctx context.Context, msgs []*consumer.Message) error {
    // Process batch of messages
    return batchInsert(ctx, msgs)
})

c.RegisterHandlers(consumer.NewTopicHandler(processor, "orders"))
```

## Middleware

### Built-in Middleware

```go
c := consumer.New(o11y,
    consumer.WithMiddleware(
        // Recover from panics
        consumer.RecoveryMiddleware(logger),
        
        // Log all messages
        consumer.LoggingMiddleware(logger),
        
        // Record metrics
        consumer.MetricsMiddleware(metricsRecorder),
        
        // Add distributed tracing
        consumer.TracingMiddleware(tracer),
        
        // Enforce timeout
        consumer.TimeoutMiddleware(30*time.Second),
        
        // Retry failed messages
        consumer.RetryMiddleware(3, 1*time.Second),
    ),
)
```

### Custom Middleware

```go
func ValidationMiddleware() consumer.Middleware {
    return func(next consumer.MessageHandlerFunc) consumer.MessageHandlerFunc {
        return func(ctx context.Context, msg *consumer.Message) error {
            // Validate message
            if len(msg.Value) == 0 {
                return fmt.Errorf("empty message")
            }
            
            // Continue processing
            return next(ctx, msg)
        }
    }
}

c := consumer.New(o11y,
    consumer.WithMiddleware(ValidationMiddleware()),
)
```

## Health Checks

### Built-in Health Checks

```go
// Check health status
status := c.Health(context.Background())

fmt.Printf("Status: %s\n", status.Status)
for name, check := range status.Checks {
    fmt.Printf("%s: %s\n", name, check.Status)
}
```

### Custom Health Checks

```go
c := consumer.New(o11y,
    consumer.WithHealthChecks(map[string]consumer.HealthCheckFunc{
        "database": func(ctx context.Context) error {
            return db.Ping(ctx)
        },
        "cache": func(ctx context.Context) error {
            return cache.Ping(ctx)
        },
        "external_api": func(ctx context.Context) error {
            resp, err := http.Get("https://api.example.com/health")
            if err != nil {
                return err
            }
            defer resp.Body.Close()
            
            if resp.StatusCode != 200 {
                return fmt.Errorf("unhealthy: %d", resp.StatusCode)
            }
            return nil
        },
    }),
)
```

### Kubernetes Probes

```go
// Readiness probe - consumer ready to process messages
ready := c.Readiness(context.Background())

// Liveness probe - consumer is alive
alive := c.Liveness(context.Background())
```

## Graceful Shutdown

The consumer automatically handles graceful shutdown:

1. **Receives signal** (SIGINT, SIGTERM) or context cancellation
2. **Stops accepting new messages** by cancelling worker context
3. **Waits for workers to finish** processing in-flight messages
4. **Respects shutdown timeout** to prevent hanging
5. **Shuts down observability** provider last
6. **Returns any errors** encountered during shutdown

```go
c := consumer.New(o11y,
    consumer.WithShutdownTimeout(30*time.Second),
)

// Blocks until signal or error
if err := c.Start(context.Background()); err != nil {
    log.Printf("Consumer error: %v", err)
}
```

## Error Handling

### Error Types

```go
// ConsumerError - general consumer errors
type ConsumerError struct {
    Op      string
    Topic   string
    Message string
    Err     error
}

// HandlerError - handler execution errors
type HandlerError struct {
    Handler string
    Topic   string
    Message string
    Err     error
    Retry   bool
}

// ProcessingError - message processing errors
type ProcessingError struct {
    Topic      string
    Partition  int32
    Offset     int64
    Attempt    int
    MaxRetries int
    Err        error
}

// ShutdownError - shutdown errors
type ShutdownError struct {
    Message string
    Err     error
}
```

### Error Handling Example

```go
func handleMessage(ctx context.Context, msg *consumer.Message) error {
    var data MyData
    if err := json.Unmarshal(msg.Value, &data); err != nil {
        // Return non-retryable error
        return &consumer.HandlerError{
            Handler: "MyHandler",
            Topic:   msg.Topic,
            Message: "invalid JSON",
            Err:     err,
            Retry:   false,
        }
    }

    if err := processData(ctx, data); err != nil {
        // Return retryable error
        return &consumer.HandlerError{
            Handler: "MyHandler",
            Topic:   msg.Topic,
            Message: "processing failed",
            Err:     err,
            Retry:   true,
        }
    }

    return nil
}
```

## Integration with Message Brokers

This package provides the framework for lifecycle management and message processing. To integrate with actual message brokers (Kafka, RabbitMQ, etc.), extend the `consume()` method:

### Kafka Integration Example

```go
// In a real implementation, you would:
// 1. Create a Kafka consumer client
// 2. Subscribe to topics
// 3. Fetch messages in the worker loop
// 4. Pass messages through the handler chain
// 5. Commit offsets on success

func (s *Server) worker(ctx context.Context, workerID int) {
    defer s.workers.Done()
    
    // Create Kafka consumer
    kafkaConsumer := kafka.NewConsumer(...)
    defer kafkaConsumer.Close()
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Fetch messages
            messages, err := kafkaConsumer.FetchMessages(ctx, s.config.BatchSize)
            if err != nil {
                s.observability.Logger().Error(ctx, "fetch error", "error", err)
                continue
            }
            
            // Process each message
            for _, msg := range messages {
                if err := s.processMessage(ctx, msg); err != nil {
                    s.handleError(ctx, msg, err)
                } else {
                    kafkaConsumer.CommitMessage(ctx, msg)
                }
            }
        }
    }
}
```

## Best Practices

1. **Use middleware for cross-cutting concerns** - Logging, metrics, tracing should be middleware
2. **Set appropriate timeouts** - ProcessingTimeout should match your SLA
3. **Configure worker count based on load** - Start with CPU count * 2
4. **Use health checks** - Monitor dependencies and consumer lag
5. **Enable DLQ for critical topics** - Don't lose failed messages
6. **Handle errors appropriately** - Distinguish retryable vs non-retryable errors
7. **Use context cancellation** - Respect context throughout your handlers
8. **Monitor metrics** - Track processing time, error rate, lag
9. **Test graceful shutdown** - Ensure no message loss during deployment
10. **Use batch processing when appropriate** - Improves throughput for high-volume topics

## Comparison with pkg/http_server

| Feature | http_server | consumer |
|---------|-------------|----------|
| **Entrypoint** | `Start()` | `Start()` |
| **Lifecycle** | HTTP server | Worker pool |
| **Signal handling** | ✅ Same | ✅ Same |
| **Graceful shutdown** | ✅ Same | ✅ Same |
| **Health checks** | ✅ Same | ✅ Same |
| **Middleware** | HTTP middleware | Message middleware |
| **Options pattern** | ✅ Same | ✅ Same |
| **Config validation** | ✅ Same | ✅ Same |
| **Observability** | ✅ Same | ✅ Same |
| **Error handling** | ✅ Same | ✅ Same |

## License

Part of devkit-go - internal package.
