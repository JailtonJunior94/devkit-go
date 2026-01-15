package events_test

import (
	"context"
	"fmt"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/events"
)

// OrderCreatedEvent represents a concrete event implementation
type OrderCreatedEvent struct {
	orderID string
}

func (e *OrderCreatedEvent) GetEventType() string {
	return "order.created"
}

func (e *OrderCreatedEvent) GetPayload() any {
	return map[string]string{"order_id": e.orderID}
}

// EmailNotificationHandler sends email when order is created
type EmailNotificationHandler struct{}

func (h *EmailNotificationHandler) Handle(ctx context.Context, event events.Event) error {
	payload, ok := event.GetPayload().(map[string]string)
	if !ok {
		return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
	}

	orderID := payload["order_id"]

	// Check context cancellation for long operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate sending email
	fmt.Printf("Sending email for order: %s\n", orderID)
	return nil
}

// MetricsHandler records metrics
type MetricsHandler struct{}

func (h *MetricsHandler) Handle(ctx context.Context, event events.Event) error {
	fmt.Printf("Recording metric for event: %s\n", event.GetEventType())
	return nil
}

func Example() {
	// Create dispatcher
	dispatcher := events.NewEventDispatcher()

	// Register handlers (use pointers!)
	emailHandler := &EmailNotificationHandler{}
	metricsHandler := &MetricsHandler{}

	if err := dispatcher.Register("order.created", emailHandler); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.Register("order.created", metricsHandler); err != nil {
		log.Fatal(err)
	}

	// Create and dispatch event
	event := &OrderCreatedEvent{orderID: "12345"}
	ctx := context.Background()

	if err := dispatcher.Dispatch(ctx, event); err != nil {
		log.Fatal(err)
	}

	// Output:
	// Sending email for order: 12345
	// Recording metric for event: order.created
}

func ExampleEventDispatcher_Register() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	err := dispatcher.Register("order.created", handler)
	if err != nil {
		log.Fatal(err)
	}

	// Check if handler is registered
	if dispatcher.Has("order.created", handler) {
		fmt.Println("Handler registered successfully")
	}

	// Output:
	// Handler registered successfully
}

func ExampleEventDispatcher_Remove() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	dispatcher.Register("order.created", handler)

	// Remove handler
	err := dispatcher.Remove("order.created", handler)
	if err != nil {
		log.Fatal(err)
	}

	// Verify removal
	if !dispatcher.Has("order.created", handler) {
		fmt.Println("Handler removed successfully")
	}

	// Output:
	// Handler removed successfully
}

func ExampleEventDispatcher_Dispatch_withContextCancellation() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	dispatcher.Register("order.created", handler)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := &OrderCreatedEvent{orderID: "12345"}
	err := dispatcher.Dispatch(ctx, event)

	if err == context.Canceled {
		fmt.Println("Dispatch cancelled due to context cancellation")
	}

	// Output:
	// Dispatch cancelled due to context cancellation
}

func ExampleNewEventDispatcher_withCapacity() {
	// Create dispatcher with pre-allocated capacity for 50 event types
	// This avoids map reallocations when registering many event types
	dispatcher := events.NewEventDispatcher(events.WithCapacity(50))

	handler := &EmailNotificationHandler{}
	dispatcher.Register("order.created", handler)

	fmt.Println("Dispatcher created with capacity optimization")

	// Output:
	// Dispatcher created with capacity optimization
}
