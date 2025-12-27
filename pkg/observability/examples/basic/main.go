package main

import (
	"context"
	"fmt"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

// OrderService demonstrates how to use observability in your services.
type OrderService struct {
	o11y observability.Observability
}

// NewOrderService creates a new order service with observability.
func NewOrderService(o11y observability.Observability) *OrderService {
	return &OrderService{o11y: o11y}
}

// CreateOrder demonstrates tracing, logging, and metrics in action.
func (s *OrderService) CreateOrder(ctx context.Context, customerID string, items int) error {
	// Start a trace span
	ctx, span := s.o11y.Tracer().Start(ctx, "OrderService.CreateOrder",
		observability.WithSpanKind(observability.SpanKindServer),
		observability.WithAttributes(
			observability.String("customer_id", customerID),
			observability.Int("items_count", items),
		),
	)
	defer span.End()

	// Log with structured fields (automatically includes trace context)
	s.o11y.Logger().Info(ctx, "creating order",
		observability.String("customer_id", customerID),
		observability.Int("items", items),
	)

	// Record metrics
	counter := s.o11y.Metrics().Counter(
		"orders.created.total",
		"Total number of orders created",
		"1",
	)

	// Simulate business logic
	if items == 0 {
		err := fmt.Errorf("cannot create order with 0 items")

		// Record error in span
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "invalid order")

		// Log error
		s.o11y.Logger().Error(ctx, "failed to create order",
			observability.Error(err),
			observability.String("customer_id", customerID),
		)

		return err
	}

	// Add span event
	span.AddEvent("order.validated",
		observability.String("validation", "passed"),
	)

	// Success - increment counter
	counter.Add(ctx, 1,
		observability.String("status", "success"),
		observability.String("customer_type", "premium"),
	)

	// Record duration metric
	histogram := s.o11y.Metrics().Histogram(
		"order.creation.duration",
		"Order creation duration",
		"ms",
	)
	histogram.Record(ctx, 45.5,
		observability.String("customer_type", "premium"),
	)

	// Mark span as successful
	span.SetStatus(observability.StatusCodeOK, "order created")

	s.o11y.Logger().Info(ctx, "order created successfully",
		observability.String("order_id", "ORD-12345"),
		observability.String("customer_id", customerID),
	)

	return nil
}

func main() {
	ctx := context.Background()

	// Option 1: Use NoOp provider (zero overhead, no observability)
	o11y := noop.NewProvider()

	// Option 2: Use Fake provider (for testing)
	// obs := fake.NewProvider()

	// Option 3: Use OpenTelemetry provider (for production)
	// config := &otel.Config{
	//     ServiceName:     "order-service",
	//     ServiceVersion:  "1.0.0",
	//     Environment:     "production",
	//     OTLPEndpoint:    "localhost:4317",
	//     TraceSampleRate: 1.0,
	//     LogLevel:        observability.LogLevelInfo,
	//     LogFormat:       observability.LogFormatJSON,
	// }
	// o11y, err := otel.NewProvider(ctx, config)
	// if err != nil {
	//     log.Fatalf("failed to initialize observability: %v", err)
	// }
	// defer o11y.Shutdown(ctx)

	// Create service with observability
	service := NewOrderService(o11y)

	// Example 1: Successful order
	fmt.Println("=== Creating valid order ===")
	if err := service.CreateOrder(ctx, "CUST-001", 3); err != nil {
		log.Printf("Error: %v\n", err)
	}

	fmt.Println("\n=== Creating invalid order (0 items) ===")
	// Example 2: Failed order
	if err := service.CreateOrder(ctx, "CUST-002", 0); err != nil {
		log.Printf("Expected error: %v\n", err)
	}

	fmt.Println("\n=== Example completed ===")
}
