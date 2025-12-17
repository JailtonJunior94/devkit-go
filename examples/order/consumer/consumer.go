package consumer

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	OrdersExchange = "order"
	OrderCreated   = "order_created"
	OrderUpdated   = "order_updated"
	OrderQueue     = "order"
	FinanceQueue   = "finance_order"
)

var (
	Exchanges = []*rabbitmq.Exchange{
		rabbitmq.NewExchange(OrdersExchange, "direct"),
	}

	Bindings = []*rabbitmq.Binding{
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderCreated),
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderUpdated),
		rabbitmq.NewBindingRouting(FinanceQueue, OrdersExchange, OrderCreated),
	}
)

type Order struct {
	ID     int     `json:"id"`
	Status string  `json:"status"`
	Value  float64 `json:"value"`
}

type orderConsumer struct {
	telemetry o11y.Telemetry
}

func NewConsumer() *orderConsumer {
	return &orderConsumer{}
}

func (s *orderConsumer) Run() {
	ctx := context.Background()

	// Setup telemetry (tracer, metrics, logger)
	cfg := o11y.Config{
		ServiceName:     "order-consumer",
		ServiceVersion:  "1.0.0",
		Environment:     getEnv("ENVIRONMENT", "development"),
		TracerEndpoint:  getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		MetricsEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		LoggerEndpoint:  getEnv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "http://localhost:4318/v1/logs"),
	}

	telemetry, err := o11y.SetupTelemetryInsecure(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to setup telemetry: %v", err)
	}
	defer telemetry.ShutdownWithTimeout(10 * time.Second)

	s.telemetry = telemetry

	// Setup RabbitMQ
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672")
	connection, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	consumer, err := rabbitmq.NewConsumer(
		rabbitmq.WithName("order-consumer"),
		rabbitmq.WithChannel(channel),
		rabbitmq.WithQueue(OrderQueue),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Register handlers with tracing
	consumer.RegisterHandler(OrderCreated, s.orderCreatedHandler())
	consumer.RegisterHandler(OrderUpdated, s.orderUpdatedHandler())

	telemetry.Logger().Info(ctx, "order-consumer started", o11y.LogField("queue", OrderQueue))

	// Start consuming in background
	go func() {
		if err := consumer.Consume(ctx); err != nil {
			telemetry.Logger().Error(ctx, err, "consumer error")
		}
	}()

	// Wait for interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-interrupt

	telemetry.Logger().Info(ctx, "shutting down order-consumer")
}

func (s *orderConsumer) orderCreatedHandler() func(ctx context.Context, params map[string]string, body []byte) error {
	return func(ctx context.Context, params map[string]string, body []byte) error {
		// Extract trace context from message headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(params))

		// Start span for processing (child of the producer span)
		ctx, span := s.telemetry.Tracer().Start(ctx, "ProcessOrderCreatedEvent",
			o11y.Attr("messaging.system", "rabbitmq"),
			o11y.Attr("messaging.operation", "receive"),
			o11y.Attr("messaging.destination", OrderQueue),
		)
		defer span.End()

		requestID := params["request_id"]
		s.telemetry.Logger().Info(ctx, "processing order created event",
			o11y.LogField("request_id", requestID),
			o11y.LogField("trace_id", o11y.TraceIDFromContext(ctx)),
		)

		// Parse order
		var order Order
		if err := json.Unmarshal(body, &order); err != nil {
			span.RecordError(err)
			span.SetStatus(o11y.SpanStatusError, "failed to unmarshal order")
			s.telemetry.Logger().Error(ctx, err, "failed to unmarshal order")
			return err
		}

		span.SetAttributes(
			o11y.Attr("order.id", order.ID),
			o11y.Attr("order.status", order.Status),
			o11y.Attr("order.value", order.Value),
		)

		// Simulate order processing
		processCtx, processSpan := s.telemetry.Tracer().Start(ctx, "ProcessOrderLogic")
		if err := s.processOrder(processCtx, order); err != nil {
			processSpan.RecordError(err)
			processSpan.SetStatus(o11y.SpanStatusError, err.Error())
			processSpan.End()
			return err
		}
		processSpan.AddEvent("order_processed_successfully")
		processSpan.End()

		// Record metric
		s.telemetry.Metrics().AddCounter(ctx, "orders.processed", 1,
			"event_type", OrderCreated,
			"status", "success",
		)

		s.telemetry.Logger().Info(ctx, "order created event processed successfully",
			o11y.LogField("order_id", order.ID),
		)

		return nil
	}
}

func (s *orderConsumer) orderUpdatedHandler() func(ctx context.Context, params map[string]string, body []byte) error {
	return func(ctx context.Context, params map[string]string, body []byte) error {
		// Extract trace context from message headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(params))

		// Start span for processing
		ctx, span := s.telemetry.Tracer().Start(ctx, "ProcessOrderUpdatedEvent",
			o11y.Attr("messaging.system", "rabbitmq"),
			o11y.Attr("messaging.operation", "receive"),
			o11y.Attr("messaging.destination", OrderQueue),
		)
		defer span.End()

		requestID := params["request_id"]
		s.telemetry.Logger().Info(ctx, "processing order updated event",
			o11y.LogField("request_id", requestID),
			o11y.LogField("trace_id", o11y.TraceIDFromContext(ctx)),
		)

		// Parse order
		var order Order
		if err := json.Unmarshal(body, &order); err != nil {
			span.RecordError(err)
			span.SetStatus(o11y.SpanStatusError, "failed to unmarshal order")
			s.telemetry.Logger().Error(ctx, err, "failed to unmarshal order")
			return err
		}

		span.SetAttributes(
			o11y.Attr("order.id", order.ID),
			o11y.Attr("order.status", order.Status),
		)

		// Record metric
		s.telemetry.Metrics().AddCounter(ctx, "orders.processed", 1,
			"event_type", OrderUpdated,
			"status", "success",
		)

		s.telemetry.Logger().Info(ctx, "order updated event processed successfully",
			o11y.LogField("order_id", order.ID),
		)

		return nil
	}
}

func (s *orderConsumer) processOrder(ctx context.Context, order Order) error {
	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	s.telemetry.Logger().Info(ctx, "order processed",
		o11y.LogField("order_id", order.ID),
		o11y.LogField("status", order.Status),
		o11y.LogField("value", order.Value),
	)

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
