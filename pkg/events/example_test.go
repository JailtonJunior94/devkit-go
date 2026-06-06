package events_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/events"
)

type OrderCreatedEvent struct {
	orderID string
}

func (e *OrderCreatedEvent) GetEventType() string {
	return "order.created"
}

func (e *OrderCreatedEvent) GetPayload() any {
	return map[string]string{"order_id": e.orderID}
}

type EmailNotificationHandler struct{}

func (h *EmailNotificationHandler) Handle(ctx context.Context, event events.Event) error {
	payload, ok := event.GetPayload().(map[string]string)
	if !ok {
		return fmt.Errorf("unexpected payload type: %T", event.GetPayload())
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	fmt.Printf("Sending email for order: %s\n", payload["order_id"])
	return nil
}

type MetricsHandler struct{}

func (h *MetricsHandler) Handle(ctx context.Context, event events.Event) error {
	fmt.Printf("Recording metric for event: %s\n", event.GetEventType())
	return nil
}

func Example() {
	dispatcher := events.NewEventDispatcher()
	emailHandler := &EmailNotificationHandler{}
	metricsHandler := &MetricsHandler{}

	if err := dispatcher.Register("order.created", emailHandler); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.Register("order.created", metricsHandler); err != nil {
		log.Fatal(err)
	}

	event := &OrderCreatedEvent{orderID: "12345"}
	if err := dispatcher.Dispatch(context.Background(), event); err != nil {
		log.Fatal(err)
	}

	// Output:
	// Sending email for order: 12345
	// Recording metric for event: order.created
}

func ExampleEventDispatcher_Register() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	if err := dispatcher.Register("order.created", handler); err != nil {
		log.Fatal(err)
	}
	if dispatcher.Has("order.created", handler) {
		fmt.Println("Handler registered successfully")
	}

	// Output:
	// Handler registered successfully
}

func ExampleEventDispatcher_Remove() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	if err := dispatcher.Register("order.created", handler); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.Remove("order.created", handler); err != nil {
		log.Fatal(err)
	}
	if !dispatcher.Has("order.created", handler) {
		fmt.Println("Handler removed successfully")
	}

	// Output:
	// Handler removed successfully
}

func ExampleEventDispatcher_Dispatch_withContextCancellation() {
	dispatcher := events.NewEventDispatcher()
	handler := &EmailNotificationHandler{}

	if err := dispatcher.Register("order.created", handler); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := &OrderCreatedEvent{orderID: "12345"}
	err := dispatcher.Dispatch(ctx, event)

	if errors.Is(err, context.Canceled) {
		fmt.Println("Dispatch cancelled due to context cancellation")
	}

	// Output:
	// Dispatch cancelled due to context cancellation
}

func ExampleNewEventDispatcher_withCapacity() {
	dispatcher := events.NewEventDispatcher(events.WithCapacity(50))
	handler := &EmailNotificationHandler{}

	if err := dispatcher.Register("order.created", handler); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Dispatcher created with capacity optimization")

	// Output:
	// Dispatcher created with capacity optimization
}
