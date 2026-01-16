# events

Thread-safe, in-process event dispatcher for domain events and publish-subscribe patterns.

## Introduction

### Problem It Solves

In Domain-Driven Design and event-driven architectures, you need a way to:
- Decouple components by publishing domain events
- Allow multiple handlers to react to the same event
- Maintain loose coupling between business logic layers
- Enable cross-cutting concerns (logging, notifications) without tight dependencies

This package provides a simple, thread-safe event dispatcher for in-process event handling.

### When to Use

✅ **Use when:**
- Building DDD applications with domain events
- Need pub/sub pattern within a single process
- Want to decouple business logic layers
- Multiple handlers need to react to same event
- Building plugin/hook systems

❌ **Don't use when:**
- Need distributed messaging (use `pkg/messaging/kafka` or `rabbitmq` instead)
- Events must survive process restarts (use message queue)
- Need guaranteed delivery or persistence

---

## Architecture

### Core Concepts

```
EventDispatcher (manages subscriptions)
    ├── Register(eventType, handler)  - Subscribe to event type
    ├── Dispatch(ctx, event)          - Publish event to all handlers
    ├── Remove(eventType, handler)    - Unsubscribe
    └── Has(eventType, handler)       - Check if subscribed
```

### Components

1. **Event**: Carries data with type identifier
2. **EventHandler**: Processes events
3. **EventDispatcher**: Routes events to handlers

### Thread Safety

- All operations are thread-safe (uses RWMutex)
- Handlers are called synchronously in registration order
- Dispatcher copies handler list before calling (won't block other operations)

---

## API Reference

### EventDispatcher

```go
type EventDispatcher interface {
    Register(eventType string, handler EventHandler) error
    Dispatch(ctx context.Context, event Event) error
    Remove(eventType string, handler EventHandler) error
    Has(eventType string, handler EventHandler) bool
    Clear()
}

// Constructor
NewEventDispatcher(opts ...DispatcherOption) EventDispatcher

// Options
WithCapacity(capacity int) DispatcherOption
```

### Event

```go
type Event interface {
    GetEventType() string
    GetPayload() any
}
```

### EventHandler

```go
type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

---

## Examples

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/JailtonJunior94/devkit-go/pkg/events"
)

// Define an event
type UserCreatedEvent struct {
    UserID string
    Email  string
}

func (e UserCreatedEvent) GetEventType() string {
    return "user.created"
}

func (e UserCreatedEvent) GetPayload() any {
    return e
}

// Define a handler
type EmailNotificationHandler struct{}

func (h *EmailNotificationHandler) Handle(ctx context.Context, event events.Event) error {
    // Type assert the payload
    payload, ok := event.GetPayload().(UserCreatedEvent)
    if !ok {
        return fmt.Errorf("unexpected event type")
    }

    fmt.Printf("Sending welcome email to: %s\n", payload.Email)
    return nil
}

func main() {
    ctx := context.Background()

    // Create dispatcher
    dispatcher := events.NewEventDispatcher()

    // Register handler
    emailHandler := &EmailNotificationHandler{}
    dispatcher.Register("user.created", emailHandler)

    // Dispatch event
    event := UserCreatedEvent{
        UserID: "123",
        Email:  "user@example.com",
    }

    if err := dispatcher.Dispatch(ctx, event); err != nil {
        panic(err)
    }
}
```

### Multiple Handlers

```go
// Different handlers for same event
type AuditLogHandler struct{}

func (h *AuditLogHandler) Handle(ctx context.Context, event events.Event) error {
    payload := event.GetPayload().(UserCreatedEvent)
    fmt.Printf("Audit: User %s created\n", payload.UserID)
    return nil
}

type AnalyticsHandler struct{}

func (h *AnalyticsHandler) Handle(ctx context.Context, event events.Event) error {
    payload := event.GetPayload().(UserCreatedEvent)
    fmt.Printf("Analytics: Track user signup for %s\n", payload.UserID)
    return nil
}

func main() {
    dispatcher := events.NewEventDispatcher()

    // Register multiple handlers for same event
    dispatcher.Register("user.created", &EmailNotificationHandler{})
    dispatcher.Register("user.created", &AuditLogHandler{})
    dispatcher.Register("user.created", &AnalyticsHandler{})

    // All three handlers will be called in order
    event := UserCreatedEvent{UserID: "123", Email: "user@example.com"}
    dispatcher.Dispatch(context.Background(), event)
}
```

### Error Handling

```go
type FailingHandler struct{}

func (h *FailingHandler) Handle(ctx context.Context, event events.Event) error {
    return fmt.Errorf("handler failed")
}

func main() {
    dispatcher := events.NewEventDispatcher()

    dispatcher.Register("user.created", &EmailNotificationHandler{})
    dispatcher.Register("user.created", &FailingHandler{})
    dispatcher.Register("user.created", &AuditLogHandler{})

    event := UserCreatedEvent{UserID: "123", Email: "user@example.com"}

    // Dispatch stops at first error
    err := dispatcher.Dispatch(context.Background(), event)
    if err != nil {
        // "handler failed" - AuditLogHandler never called
        fmt.Println(err)
    }
}
```

### Context Cancellation

```go
type SlowHandler struct{}

func (h *SlowHandler) Handle(ctx context.Context, event events.Event) error {
    select {
    case <-ctx.Done():
        return ctx.Err()  // Respect cancellation
    case <-time.After(5 * time.Second):
        // Long operation
        return nil
    }
}

func main() {
    dispatcher := events.NewEventDispatcher()
    dispatcher.Register("user.created", &SlowHandler{})

    // Context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    event := UserCreatedEvent{UserID: "123", Email: "user@example.com"}

    // Returns context.DeadlineExceeded after 1 second
    err := dispatcher.Dispatch(ctx, event)
}
```

### Removing Handlers

```go
func main() {
    dispatcher := events.NewEventDispatcher()

    handler := &EmailNotificationHandler{}
    dispatcher.Register("user.created", handler)

    // Check if registered
    if dispatcher.Has("user.created", handler) {
        fmt.Println("Handler is registered")
    }

    // Remove handler
    dispatcher.Remove("user.created", handler)

    // No longer registered
    if !dispatcher.Has("user.created", handler) {
        fmt.Println("Handler removed")
    }
}
```

### Domain Service with Events

```go
type UserService struct {
    dispatcher events.EventDispatcher
}

func NewUserService(dispatcher events.EventDispatcher) *UserService {
    return &UserService{dispatcher: dispatcher}
}

func (s *UserService) CreateUser(ctx context.Context, email string) error {
    // Business logic
    userID := generateID()

    // Save to database...

    // Publish domain event
    event := UserCreatedEvent{
        UserID: userID,
        Email:  email,
    }

    // Handlers will be notified (email, audit, etc.)
    return s.dispatcher.Dispatch(ctx, event)
}
```

---

## Best Practices

### 1. Use Pointer Receivers for Handlers

```go
// ✅ Good: Pointer receiver ensures identity
type MyHandler struct{}

func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    return nil
}

// Register
handler := &MyHandler{}
dispatcher.Register("event.type", handler)

// ❌ Bad: Value receiver may cause issues with Has() and Remove()
func (h MyHandler) Handle(ctx context.Context, event events.Event) error {
    return nil
}
```

### 2. Always Type Assert Event Payload

```go
// ✅ Good: Safe type assertion
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    payload, ok := event.GetPayload().(UserCreatedEvent)
    if !ok {
        return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
    }
    // Use payload safely
}

// ❌ Bad: Panic if wrong type
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    payload := event.GetPayload().(UserCreatedEvent)  // May panic!
}
```

### 3. Respect Context Cancellation

```go
// ✅ Good: Check context in long operations
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    for _, item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Process item
        }
    }
    return nil
}
```

### 4. Handle Errors Gracefully

```go
// Handler should return error, not panic
func (h *MyHandler) Handle(ctx context.Context, event events.Event) error {
    if err := doSomething(); err != nil {
        // Log error, but return it
        return fmt.Errorf("failed to process event: %w", err)
    }
    return nil
}
```

### 5. Don't Modify Event Payload

```go
// ❌ Bad: Mutating shared event data
func (h *Handler1) Handle(ctx context.Context, event events.Event) error {
    payload := event.GetPayload().(*SomeStruct)
    payload.Field = "modified"  // Affects other handlers!
    return nil
}

// ✅ Good: Read-only access
func (h *Handler2) Handle(ctx context.Context, event events.Event) error {
    payload := event.GetPayload().(SomeStruct)  // Value copy
    localCopy := payload
    localCopy.Field = "modified"  // Only affects local copy
    return nil
}
```

---

## Caveats and Limitations

### Synchronous Execution

**Limitation:** Handlers are called synchronously in registration order.

```go
// All handlers block dispatcher
dispatcher.Register("event", &SlowHandler{})  // Takes 5 seconds
dispatcher.Register("event", &FastHandler{})  // Waits for SlowHandler

dispatcher.Dispatch(ctx, event)  // Blocks until all handlers complete
```

**Workaround:** Spawn goroutines inside handlers for async work:

```go
func (h *AsyncHandler) Handle(ctx context.Context, event events.Event) error {
    go func() {
        // Async work (be careful with ctx lifecycle)
    }()
    return nil  // Return immediately
}
```

### No Event Persistence

**Limitation:** Events are not persisted. If process crashes, events are lost.

**Workaround:** Use `pkg/messaging/kafka` or `rabbitmq` for durability.

### No Retry Logic

**Limitation:** If a handler fails, event is not retried.

**Workaround:** Implement retry logic inside handler or use message queue.

### Handler Order Matters

**Caveat:** Handlers are called in registration order. If order is important, register in correct sequence.

```go
// Handler1 will always run before Handler2
dispatcher.Register("event", handler1)
dispatcher.Register("event", handler2)
```

### Memory Usage with Many Event Types

**Consideration:** Each event type creates a map entry. For thousands of event types, use `WithCapacity()`:

```go
dispatcher := events.NewEventDispatcher(events.WithCapacity(1000))
```

---

## Error Reference

```go
ErrHandlerAlreadyRegistered  // Handler is already registered for this event type
ErrEventNil                  // Nil event passed to Dispatch
ErrHandlerNil                // Nil handler passed to Register
ErrEventTypeEmpty            // Empty event type string
```

---

## Thread Safety

- **EventDispatcher** is fully thread-safe
- **Dispatch** can be called concurrently
- **Register/Remove** can be called while Dispatch is running
- Handlers are copied before execution (doesn't block new registrations)

---

## Testing

```go
func TestEventDispatcher(t *testing.T) {
    dispatcher := events.NewEventDispatcher()

    called := false
    handler := &testHandler{
        handleFunc: func(ctx context.Context, event events.Event) error {
            called = true
            return nil
        },
    }

    dispatcher.Register("test.event", handler)

    event := testEvent{eventType: "test.event"}
    err := dispatcher.Dispatch(context.Background(), event)

    assert.NoError(t, err)
    assert.True(t, called)
}
```

---

## Related Packages

- `pkg/messaging/kafka` - Distributed event streaming
- `pkg/messaging/rabbitmq` - Distributed pub/sub
- `pkg/observability` - Add tracing to event handlers
