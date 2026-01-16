# messaging

Production-ready Kafka and RabbitMQ integrations with automatic retries, dead-letter queues, and distributed tracing.

## Introduction

Unified messaging interface for Kafka and RabbitMQ with:
- Automatic retries and error handling
- Dead-letter queue (DLQ) support
- OpenTelemetry distributed tracing
- Graceful shutdown
- Health checks

### When to Use

✅ **Use when:** Event streaming, pub/sub, microservices communication, async processing
❌ **Don't use when:** Synchronous HTTP is sufficient, in-process events (use `pkg/events`)

---

## Architecture

```
messaging/
├── publisher.go          # Publisher interface
├── consumer.go           # Consumer interface
├── kafka/                # Kafka implementation
│   ├── new_producer.go   # Producer
│   ├── config.go         # Configuration
│   ├── options.go        # Publisher/Consumer options
│   ├── dlq.go            # Dead-letter queue
│   └── auth/             # Authentication strategies
└── rabbitmq/             # RabbitMQ implementation
    ├── publisher.go      # Publisher
    ├── consumer.go       # Consumer
    ├── config.go         # Configuration
    └── dlq.go            # Dead-letter queue
```

---

## API Reference

### Core Interfaces

```go
type Publisher interface {
    Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *Message) error
    PublishBatch(ctx context.Context, topicOrQueue, key string, headers map[string]string, messages []*Message) error
    Close() error
}

type Consumer interface {
    Consume(ctx context.Context, topics []string, handler MessageHandler) error
    Close() error
}

type Message struct {
    Body    []byte
    Headers []Header
}

type MessageHandler func(ctx context.Context, message *Message) error
```

---

## Kafka Examples

### Producer Setup

```go
package main

import (
    "context"
    "github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
    "github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka/auth"
)

func main() {
    ctx := context.Background()

    config := &kafka.ProducerConfig{
        Brokers: []string{"localhost:9092"},
        Auth:    auth.NewPlaintextStrategy(),
    }

    producer, err := kafka.NewProducer(ctx, config)
    if err != nil {
        panic(err)
    }
    defer producer.Close()

    // Publish message
    message := &messaging.Message{
        Body: []byte(`{"event":"user.created","user_id":"123"}`),
    }

    err = producer.Publish(ctx, "user-events", "user-123", nil, message)
    if err != nil {
        panic(err)
    }
}
```

### Consumer Setup

```go
func main() {
    ctx := context.Background()

    config := &kafka.ConsumerConfig{
        Brokers:      []string{"localhost:9092"},
        GroupID:      "user-service",
        Auth:         auth.NewPlaintextStrategy(),
        MaxRetries:   3,  // Automatic retries
        EnableDLQ:    true,  // Dead-letter queue
    }

    consumer, err := kafka.NewConsumer(ctx, config)
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    // Consume messages
    handler := func(ctx context.Context, msg *messaging.Message) error {
        fmt.Printf("Received: %s\n", string(msg.Body))
        // Process message
        return nil  // Return error to trigger retry
    }

    err = consumer.Consume(ctx, []string{"user-events"}, handler)
    if err != nil {
        panic(err)
    }
}
```

### Kafka Authentication

```go
// Plaintext (development)
auth := auth.NewPlaintextStrategy()

// SASL/PLAIN
auth := auth.NewPlainStrategy("username", "password")

// SASL/SCRAM-SHA-256
auth := auth.NewScramStrategy(
    "username",
    "password",
    auth.ScramSHA256,
)

// SASL/SCRAM-SHA-512
auth := auth.NewScramStrategy(
    "username",
    "password",
    auth.ScramSHA512,
)

// Confluent Cloud
auth := auth.NewConfluentStrategy("api-key", "api-secret")
```

### Kafka with Distributed Tracing

```go
// Messages automatically include trace context
producer, _ := kafka.NewProducer(ctx, config)

// This message will carry trace context
ctx, span := tracer.Start(ctx, "publish-user-event")
defer span.End()

producer.Publish(ctx, "user-events", "user-123", nil, message)

// Consumer automatically extracts trace context
consumer.Consume(ctx, []string{"user-events"}, func(ctx context.Context, msg *messaging.Message) error {
    // ctx includes parent trace from producer
    span := trace.SpanFromContext(ctx)
    span.AddEvent("processing message")
    return nil
})
```

### Kafka DLQ (Dead-Letter Queue)

```go
config := &kafka.ConsumerConfig{
    Brokers:      []string{"localhost:9092"},
    GroupID:      "user-service",
    MaxRetries:   3,
    EnableDLQ:    true,  // Enable DLQ
    DLQTopic:     "user-events-dlq",  // Optional: custom DLQ topic
}

consumer, _ := kafka.NewConsumer(ctx, config)

// If handler returns error after 3 retries, message goes to DLQ
consumer.Consume(ctx, []string{"user-events"}, func(ctx context.Context, msg *messaging.Message) error {
    if err := processMessage(msg); err != nil {
        return err  // Will retry up to 3 times, then move to DLQ
    }
    return nil
})
```

---

## RabbitMQ Examples

### Publisher Setup

```go
import "github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"

func main() {
    ctx := context.Background()

    config := &rabbitmq.Config{
        URL:      "amqp://guest:guest@localhost:5672/",
        Exchange: "user-events",
        Strategy: rabbitmq.FanoutStrategy,  // or DirectStrategy, TopicStrategy
    }

    publisher, err := rabbitmq.NewPublisher(ctx, config)
    if err != nil {
        panic(err)
    }
    defer publisher.Close()

    message := &messaging.Message{
        Body: []byte(`{"event":"user.created"}`),
    }

    err = publisher.Publish(ctx, "user-events", "user.created", nil, message)
}
```

### Consumer Setup

```go
func main() {
    ctx := context.Background()

    config := &rabbitmq.Config{
        URL:         "amqp://guest:guest@localhost:5672/",
        Exchange:    "user-events",
        Queue:       "user-service-queue",
        Strategy:    rabbitmq.FanoutStrategy,
        MaxRetries:  3,
        EnableDLQ:   true,
    }

    consumer, err := rabbitmq.NewConsumer(ctx, config)
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    handler := func(ctx context.Context, msg *messaging.Message) error {
        fmt.Printf("Received: %s\n", string(msg.Body))
        return nil
    }

    err = consumer.Consume(ctx, nil, handler)
    if err != nil {
        panic(err)
    }
}
```

### RabbitMQ Exchange Strategies

```go
// Fanout: Broadcast to all queues
config.Strategy = rabbitmq.FanoutStrategy

// Direct: Route by routing key (exact match)
config.Strategy = rabbitmq.DirectStrategy

// Topic: Route by pattern (e.g., "user.*", "order.created")
config.Strategy = rabbitmq.TopicStrategy
```

### RabbitMQ with Connection Pooling

```go
config := &rabbitmq.Config{
    URL:             "amqp://guest:guest@localhost:5672/",
    ChannelPoolSize: 10,  // Connection pooling
}
```

---

## Best Practices

### 1. Always Handle Errors in Message Handlers

```go
// ✅ Good: Return error for retry
handler := func(ctx context.Context, msg *messaging.Message) error {
    if err := processMessage(msg); err != nil {
        return fmt.Errorf("failed to process: %w", err)  // Triggers retry
    }
    return nil  // Acknowledge
}

// ❌ Bad: Swallowing errors
handler := func(ctx context.Context, msg *messaging.Message) error {
    processMessage(msg)  // Ignores errors!
    return nil
}
```

### 2. Use Context for Graceful Shutdown

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan
    cancel()  // Stops consumer gracefully
}()

consumer.Consume(ctx, []string{"topic"}, handler)
```

### 3. Enable DLQ for Production

```go
// ✅ Production: Enable DLQ
config := &kafka.ConsumerConfig{
    MaxRetries: 3,
    EnableDLQ:  true,
}

// ❌ Development: Can disable for simplicity
config.EnableDLQ = false
```

### 4. Use Appropriate Retry Counts

```go
// Transient errors: Moderate retries
MaxRetries: 3

// Idempotent operations: More retries
MaxRetries: 10

// Non-idempotent operations: Fewer retries
MaxRetries: 1
```

### 5. Batch Publishing for Performance

```go
// ✅ Better: Batch publish
messages := []*messaging.Message{msg1, msg2, msg3}
producer.PublishBatch(ctx, "topic", "key", nil, messages)

// ⚠️ Slower: Individual publishes
for _, msg := range messages {
    producer.Publish(ctx, "topic", "key", nil, msg)
}
```

---

## Caveats and Limitations

### Kafka Consumer Lag

**Issue:** Consumer can't keep up with producer.
**Solution:** Scale consumers horizontally (same GroupID), optimize handler, increase partition count.

### Message Order

**Kafka:** Order guaranteed per partition. Use same key for related messages.
**RabbitMQ:** Order not guaranteed with multiple consumers.

### Exactly-Once Semantics

**Limitation:** At-least-once delivery. Handlers should be idempotent.

```go
// ✅ Idempotent handler
func handler(ctx context.Context, msg *messaging.Message) error {
    userID := extractUserID(msg)

    // Use UPSERT or check if already processed
    _, err := db.Exec(`
        INSERT INTO users (id, name) VALUES ($1, $2)
        ON CONFLICT (id) DO NOTHING
    `, userID, name)
    return err
}
```

### DLQ Monitoring

**Important:** Monitor DLQ topics/queues. Messages in DLQ require manual intervention.

```go
// Set up alerts for DLQ message count
// Review DLQ messages periodically
```

---

## Configuration Reference

### Kafka ProducerConfig

```go
type ProducerConfig struct {
    Brokers        []string           // Required
    Auth           auth.Strategy      // Authentication
    Compression    kafka.Compression  // None, Gzip, Snappy, Lz4, Zstd
    RequiredAcks   int                // 0, 1, or -1 (all)
    MaxRetries     int
    BatchSize      int
    BatchTimeout   time.Duration
}
```

### Kafka ConsumerConfig

```go
type ConsumerConfig struct {
    Brokers        []string
    GroupID        string        // Required
    Auth           auth.Strategy
    MaxRetries     int           // Default: 3
    EnableDLQ      bool          // Default: false
    DLQTopic       string        // Optional custom DLQ topic
    CommitInterval time.Duration
}
```

### RabbitMQ Config

```go
type Config struct {
    URL             string
    Exchange        string
    Queue           string
    Strategy        ExchangeStrategy  // Fanout, Direct, Topic
    MaxRetries      int
    EnableDLQ       bool
    ChannelPoolSize int              // Default: 5
}
```

---

## Health Checks

```go
// Kafka producer health
if err := producer.Ping(ctx); err != nil {
    // Producer unhealthy
}

// RabbitMQ health
if err := publisher.Health(ctx); err != nil {
    // Publisher unhealthy
}
```

---

## Related Packages

- `pkg/observability` - Automatic distributed tracing
- `pkg/events` - In-process events (simpler, no persistence)
- Integration tests use `testcontainers-go`
