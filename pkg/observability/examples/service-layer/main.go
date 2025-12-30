package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

// OrderRepository demonstrates repository pattern with observability.
type OrderRepository struct {
	obs observability.Observability
}

func NewOrderRepository(obs observability.Observability) *OrderRepository {
	return &OrderRepository{obs: obs}
}

type Order struct {
	ID         string
	CustomerID string
	Total      float64
	Status     string
	CreatedAt  time.Time
}

// FindByID demonstrates database operation with observability.
func (r *OrderRepository) FindByID(ctx context.Context, orderID string) (*Order, error) {
	ctx, span := r.obs.Tracer().Start(ctx, "OrderRepository.FindByID",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(
			observability.String("db.system", "postgresql"),
			observability.String("db.operation", "SELECT"),
			observability.String("order_id", orderID),
		),
	)
	defer span.End()

	// Create child logger with repository context
	logger := r.obs.Logger().With(
		observability.String("component", "repository"),
		observability.String("operation", "find_by_id"),
	)

	logger.Debug(ctx, "executing database query",
		observability.String("order_id", orderID),
	)

	// Record database query metric
	dbQueryCounter := r.obs.Metrics().Counter(
		"db.queries",
		"Total database queries",
		"1",
	)
	dbQueryCounter.Increment(ctx,
		observability.String("operation", "SELECT"),
		observability.String("table", "orders"),
	)

	// Simulate database query
	start := time.Now()
	time.Sleep(20 * time.Millisecond)

	// Record query duration
	queryDuration := r.obs.Metrics().Histogram(
		"db.query.duration",
		"Database query duration",
		"ms",
	)
	queryDuration.Record(ctx, float64(time.Since(start).Milliseconds()),
		observability.String("operation", "SELECT"),
		observability.String("table", "orders"),
	)

	// Simulate not found
	if orderID == "404" {
		err := errors.New("order not found")
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "order not found")

		logger.Warn(ctx, "order not found",
			observability.String("order_id", orderID),
		)

		return nil, err
	}

	order := &Order{
		ID:         orderID,
		CustomerID: "customer-123",
		Total:      299.99,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	span.SetStatus(observability.StatusCodeOK, "order found")
	span.AddEvent("order_found",
		observability.String("status", order.Status),
		observability.Float64("total", order.Total),
	)

	logger.Info(ctx, "order found",
		observability.String("order_id", orderID),
		observability.String("status", order.Status),
	)

	return order, nil
}

// Save demonstrates insert/update operation with observability.
func (r *OrderRepository) Save(ctx context.Context, order *Order) error {
	ctx, span := r.obs.Tracer().Start(ctx, "OrderRepository.Save",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(
			observability.String("db.system", "postgresql"),
			observability.String("db.operation", "INSERT"),
			observability.String("order_id", order.ID),
		),
	)
	defer span.End()

	r.obs.Logger().Info(ctx, "saving order",
		observability.String("order_id", order.ID),
		observability.Float64("total", order.Total),
	)

	// Simulate database operation
	time.Sleep(30 * time.Millisecond)

	span.SetStatus(observability.StatusCodeOK, "order saved")

	// Increment save counter
	saveCounter := r.obs.Metrics().Counter(
		"db.operations",
		"Total database operations",
		"1",
	)
	saveCounter.Increment(ctx,
		observability.String("operation", "INSERT"),
		observability.String("table", "orders"),
	)

	return nil
}

// OrderService demonstrates service layer with business logic and observability.
type OrderService struct {
	repo *OrderRepository
	obs  observability.Observability
}

func NewOrderService(repo *OrderRepository, obs observability.Observability) *OrderService {
	return &OrderService{
		repo: repo,
		obs:  obs,
	}
}

// CreateOrder demonstrates complex business operation with multiple spans.
func (s *OrderService) CreateOrder(ctx context.Context, customerID string, total float64) (*Order, error) {
	// Start parent span for the entire operation
	ctx, span := s.obs.Tracer().Start(ctx, "OrderService.CreateOrder",
		observability.WithSpanKind(observability.SpanKindInternal),
		observability.WithAttributes(
			observability.String("customer_id", customerID),
			observability.Float64("total", total),
		),
	)
	defer span.End()

	// Create service logger
	logger := s.obs.Logger().With(
		observability.String("component", "service"),
		observability.String("service", "order"),
	)

	logger.Info(ctx, "creating order",
		observability.String("customer_id", customerID),
		observability.Float64("total", total),
	)

	// Validate order (child span)
	if err := s.validateOrder(ctx, customerID, total); err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "validation failed")

		logger.Error(ctx, "order validation failed",
			observability.Error(err),
			observability.String("customer_id", customerID),
		)

		// Increment validation error counter
		errorCounter := s.obs.Metrics().Counter(
			"order.validation.errors",
			"Total order validation errors",
			"1",
		)
		errorCounter.Increment(ctx,
			observability.String("error_type", "validation"),
		)

		return nil, err
	}

	// Create order entity
	order := &Order{
		ID:         fmt.Sprintf("order-%d", time.Now().Unix()),
		CustomerID: customerID,
		Total:      total,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	// Save order (child span created automatically by repository)
	if err := s.repo.Save(ctx, order); err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "failed to save order")

		logger.Error(ctx, "failed to save order",
			observability.Error(err),
			observability.String("order_id", order.ID),
		)

		return nil, err
	}

	// Success
	span.SetStatus(observability.StatusCodeOK, "order created successfully")
	span.AddEvent("order_created",
		observability.String("order_id", order.ID),
		observability.String("status", order.Status),
	)

	logger.Info(ctx, "order created successfully",
		observability.String("order_id", order.ID),
		observability.String("customer_id", customerID),
		observability.Float64("total", total),
	)

	// Increment success counter
	successCounter := s.obs.Metrics().Counter(
		"order.created",
		"Total orders created",
		"1",
	)
	successCounter.Increment(ctx,
		observability.String("status", "success"),
	)

	// Record order value histogram
	orderValueHistogram := s.obs.Metrics().Histogram(
		"order.value",
		"Order value distribution",
		"USD",
	)
	orderValueHistogram.Record(ctx, total)

	return order, nil
}

// validateOrder demonstrates nested span for sub-operation.
func (s *OrderService) validateOrder(ctx context.Context, customerID string, total float64) error {
	ctx, span := s.obs.Tracer().Start(ctx, "OrderService.validateOrder",
		observability.WithSpanKind(observability.SpanKindInternal),
	)
	defer span.End()

	s.obs.Logger().Debug(ctx, "validating order",
		observability.String("customer_id", customerID),
		observability.Float64("total", total),
	)

	if customerID == "" {
		err := errors.New("customer_id is required")
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "invalid customer_id")
		return err
	}

	if total <= 0 {
		err := errors.New("total must be greater than zero")
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "invalid total")
		return err
	}

	span.SetStatus(observability.StatusCodeOK, "validation passed")
	return nil
}

// ProcessOrder demonstrates async operation with observability.
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
	ctx, span := s.obs.Tracer().Start(ctx, "OrderService.ProcessOrder",
		observability.WithSpanKind(observability.SpanKindInternal),
		observability.WithAttributes(
			observability.String("order_id", orderID),
		),
	)
	defer span.End()

	logger := s.obs.Logger().With(
		observability.String("component", "service"),
		observability.String("operation", "process_order"),
	)

	logger.Info(ctx, "processing order",
		observability.String("order_id", orderID),
	)

	// Fetch order
	order, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "order not found")
		return err
	}

	// Simulate processing
	span.AddEvent("payment_processing")
	time.Sleep(100 * time.Millisecond)

	span.AddEvent("payment_completed")
	order.Status = "completed"

	// Update order
	if err := s.repo.Save(ctx, order); err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "failed to update order")
		return err
	}

	span.SetStatus(observability.StatusCodeOK, "order processed")

	logger.Info(ctx, "order processed successfully",
		observability.String("order_id", orderID),
		observability.String("new_status", order.Status),
	)

	// Record processing metric
	processCounter := s.obs.Metrics().Counter(
		"order.processed",
		"Total orders processed",
		"1",
	)
	processCounter.Increment(ctx,
		observability.String("status", "success"),
	)

	return nil
}

func main() {
	ctx := context.Background()

	// Initialize observability
	config := &otel.Config{
		ServiceName:     "order-service",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		OTLPEndpoint:    "localhost:4317",
		OTLPProtocol:    otel.ProtocolGRPC,
		Insecure:        true,
		TraceSampleRate: 1.0,
		LogLevel:        observability.LogLevelDebug,
		LogFormat:       observability.LogFormatJSON,
	}

	obs, err := otel.NewProvider(ctx, config)
	if err != nil {
		log.Fatal("Failed to initialize observability:", err)
	}
	defer func() {
		_ = obs.Shutdown(ctx)
	}()

	// Create repository and service
	repo := NewOrderRepository(obs)
	service := NewOrderService(repo, obs)

	// Example 1: Create order (success case)
	log.Println("\n=== Example 1: Creating order ===")
	order, err := service.CreateOrder(ctx, "customer-123", 299.99)
	if err != nil {
		log.Printf("Failed to create order: %v", err)
	} else {
		log.Printf("Order created: %+v", order)
	}

	// Example 2: Create order (validation error)
	log.Println("\n=== Example 2: Creating invalid order ===")
	_, err = service.CreateOrder(ctx, "", 299.99)
	if err != nil {
		log.Printf("Expected error: %v", err)
	}

	// Example 3: Find order
	log.Println("\n=== Example 3: Finding order ===")
	foundOrder, err := repo.FindByID(ctx, "order-123")
	if err != nil {
		log.Printf("Failed to find order: %v", err)
	} else {
		log.Printf("Order found: %+v", foundOrder)
	}

	// Example 4: Process order
	log.Println("\n=== Example 4: Processing order ===")
	if err := service.ProcessOrder(ctx, "order-123"); err != nil {
		log.Printf("Failed to process order: %v", err)
	} else {
		log.Println("Order processed successfully")
	}

	log.Println("\n=== All examples completed ===")
}
