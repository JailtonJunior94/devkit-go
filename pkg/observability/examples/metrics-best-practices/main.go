package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

// OrderService demonstrates best practices for metrics usage.
type OrderService struct {
	obs observability.Observability

	// Metrics instruments - created once, reused many times
	orderCounter       observability.Counter
	orderDuration      observability.Histogram
	processingDuration observability.Histogram
	activeOrders       observability.UpDownCounter
}

// NewOrderService creates a new order service with proper metrics setup.
func NewOrderService(obs observability.Observability) *OrderService {
	metrics := obs.Metrics()

	return &OrderService{
		obs: obs,

		// Counter: Track total orders
		orderCounter: metrics.Counter(
			"orders.processed.total",
			"Total number of orders processed",
			"1",
		),

		// Histogram with default buckets: General order processing time
		orderDuration: metrics.Histogram(
			"orders.duration",
			"Order processing duration",
			"s",
		),

		// Histogram with custom buckets: Detailed payment processing time
		// Payment processing is typically 10ms-500ms, so we use fine-grained buckets
		processingDuration: metrics.HistogramWithBuckets(
			"payment.processing.duration",
			"Payment processing duration with custom buckets",
			"ms",
			[]float64{5, 10, 25, 50, 75, 100, 150, 200, 300, 500, 1000},
		),

		// UpDownCounter: Track active orders being processed
		activeOrders: metrics.UpDownCounter(
			"orders.active",
			"Number of orders currently being processed",
			"1",
		),
	}
}

// ProcessOrder demonstrates metric usage with proper labels.
func (s *OrderService) ProcessOrder(ctx context.Context, orderType, paymentMethod string, amount float64) error {
	start := time.Now()

	// Increment active orders
	s.activeOrders.Add(ctx, 1)
	defer s.activeOrders.Add(ctx, -1)

	// ✅ GOOD: Low-cardinality labels
	// - order_type: standard, express, bulk (limited set)
	// - payment_method: card, paypal, bank_transfer (known values)
	// - amount_range: small, medium, large (categorical)

	// Simulate order processing
	amountRange := categorizeAmount(amount)
	s.obs.Logger().Info(ctx, "processing order",
		observability.String("order_type", orderType),
		observability.String("payment_method", paymentMethod),
		observability.String("amount_range", amountRange),
	)

	// Simulate payment processing with variable latency
	paymentStart := time.Now()
	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	paymentDuration := time.Since(paymentStart).Milliseconds()

	// Record payment processing duration with custom buckets
	s.processingDuration.Record(ctx, float64(paymentDuration),
		observability.String("payment_method", paymentMethod),
	)

	// Simulate additional order processing
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

	// Record overall order duration
	duration := time.Since(start).Seconds()
	s.orderDuration.Record(ctx, duration,
		observability.String("order_type", orderType),
		observability.String("amount_range", amountRange),
	)

	// Increment counter with status
	s.orderCounter.Increment(ctx,
		observability.String("status", "success"),
		observability.String("order_type", orderType),
		observability.String("payment_method", paymentMethod),
	)

	s.obs.Logger().Info(ctx, "order processed successfully",
		observability.String("order_type", orderType),
		observability.Float64("duration_seconds", duration),
	)

	return nil
}

// categorizeAmount converts numeric amount into categorical label.
// ✅ This prevents high cardinality from unique dollar amounts.
func categorizeAmount(amount float64) string {
	switch {
	case amount < 50:
		return "small"
	case amount < 500:
		return "medium"
	case amount < 5000:
		return "large"
	default:
		return "xlarge"
	}
}

func main() {
	ctx := context.Background()

	// Configure observability with best practices
	config := &otel.Config{
		ServiceName:    "order-service",
		ServiceVersion: "1.0.0",
		Environment:    "production",

		// OTLP configuration
		OTLPEndpoint: "localhost:4317",
		OTLPProtocol: otel.ProtocolGRPC,
		Insecure:     false, // Use TLS in production

		// Metrics configuration
		MetricExportInterval:   30,       // Export every 30 seconds for real-time visibility
		MetricNamespace:        "orders", // Prefix all metrics with "orders."
		EnableCardinalityCheck: true,     // Protect against high-cardinality labels

		// Custom blocked labels specific to this service
		CustomBlockedLabels: []string{
			"order_id",    // Order IDs are unique per order
			"customer_id", // Customer IDs are unbounded
		},

		// Tracing and logging
		TraceSampleRate: 1.0,
		LogLevel:        observability.LogLevelInfo,
		LogFormat:       observability.LogFormatJSON,
	}

	provider, err := otel.NewProvider(ctx, config)
	if err != nil {
		log.Fatal("Failed to initialize observability:", err)
	}
	defer provider.Shutdown(ctx)

	// Create service
	service := NewOrderService(provider)

	// Simulate order processing
	orderTypes := []string{"standard", "express", "bulk"}
	paymentMethods := []string{"card", "paypal", "bank_transfer"}
	amounts := []float64{25.99, 99.99, 299.99, 1500.00, 7500.00}

	fmt.Println("=== Processing Orders (Best Practices Demo) ===\n")

	for i := 0; i < 10; i++ {
		orderType := orderTypes[rand.Intn(len(orderTypes))]
		paymentMethod := paymentMethods[rand.Intn(len(paymentMethods))]
		amount := amounts[rand.Intn(len(amounts))]

		fmt.Printf("Order %d: Type=%s, Payment=%s, Amount=$%.2f\n",
			i+1, orderType, paymentMethod, amount)

		if err := service.ProcessOrder(ctx, orderType, paymentMethod, amount); err != nil {
			log.Printf("Error processing order: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Small delay between orders
	}

	fmt.Println("\n=== Demonstration of High-Cardinality Protection ===\n")

	// ❌ This will be blocked by cardinality validator
	blockedCounter := provider.Metrics().Counter(
		"demo.blocked.metric",
		"This metric will be blocked",
		"1",
	)

	fmt.Println("Attempting to use high-cardinality labels (will be silently dropped):")

	// These increments will be silently dropped due to high-cardinality labels
	blockedCounter.Increment(ctx,
		observability.String("order_id", "ORD-12345"), // ❌ Blocked
	)
	fmt.Println("  ❌ Attempted: order_id=ORD-12345 (blocked)")

	blockedCounter.Increment(ctx,
		observability.String("customer_id", "CUST-67890"), // ❌ Blocked
	)
	fmt.Println("  ❌ Attempted: customer_id=CUST-67890 (blocked)")

	blockedCounter.Increment(ctx,
		observability.String("user_id", "USER-111"), // ❌ Blocked (default)
	)
	fmt.Println("  ❌ Attempted: user_id=USER-111 (blocked by default)")

	// ✅ This will work - low cardinality
	blockedCounter.Increment(ctx,
		observability.String("status", "success"), // ✅ Allowed
	)
	fmt.Println("  ✅ Succeeded: status=success (allowed)\n")

	fmt.Println("=== Metrics Generated ===")
	fmt.Println("All metrics are prefixed with 'orders.' due to MetricNamespace:")
	fmt.Println("  - orders.orders.processed.total")
	fmt.Println("  - orders.orders.duration")
	fmt.Println("  - orders.payment.processing.duration (custom buckets)")
	fmt.Println("  - orders.orders.active")
	fmt.Println("\nMetrics are exported every 30 seconds to localhost:4317")
	fmt.Println("\nCheck Prometheus at http://localhost:9090/targets")
	fmt.Println("Query example: rate(orders_orders_processed_total[5m])")
}
