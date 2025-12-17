package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"github.com/JailtonJunior94/devkit-go/pkg/httpserver"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	OrdersExchange = "order"
	OrderQueue     = "order"
	OrderCreated   = "order_created"
	OrderUpdated   = "order_updated"
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

type PaymentRequest struct {
	OrderID int     `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type PaymentResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

type apiServer struct {
	telemetry  o11y.Telemetry
	httpClient httpclient.HTTPClient
}

func NewApiServer() *apiServer {
	return &apiServer{}
}

func (s *apiServer) Run() {
	ctx := context.Background()

	// Setup telemetry (tracer, metrics, logger)
	cfg := o11y.Config{
		ServiceName:     "order-api",
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
	s.httpClient = httpclient.NewHTTPClient()

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

	_, err = rabbitmq.NewAmqpBuilder(channel).
		DeclareExchanges(Exchanges...).
		DeclareBindings(Bindings...).
		DeclarePrefetchCount(5).
		WithDLQ().
		WithRetry().
		DeclareTTL(3 * time.Second).
		Apply()

	if err != nil {
		log.Fatal(err)
	}
	producer := rabbitmq.NewRabbitMQPublisher(channel)

	routes := []httpserver.Route{
		httpserver.NewRoute(http.MethodPost, "/orders", s.createOrderHandler(producer)),
		httpserver.NewRoute(http.MethodGet, "/health", s.healthHandler()),
	}

	server := httpserver.New(
		httpserver.WithPort(getEnv("PORT", "8001")),
		httpserver.WithRoutes(routes...),
		httpserver.WithMiddlewares(
			httpserver.RequestID,
			httpserver.SecurityHeaders,
			httpserver.Recovery,
		),
	)

	shutdown := server.Run()
	telemetry.Logger().Info(ctx, "order-api started", o11y.LogField("port", getEnv("PORT", "8001")))

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil && err != http.ErrServerClosed {
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	telemetry.Logger().Info(ctx, "shutting down order-api")
	if err := shutdown(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func (s *apiServer) createOrderHandler(producer messaging.Publisher) httpserver.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		requestID := r.Context().Value(httpserver.ContextKeyRequestID).(string)

		// Start main span for order creation
		ctx, span := s.telemetry.Tracer().Start(ctx, "CreateOrder",
			o11y.Attr("request_id", requestID),
		)
		defer span.End()

		s.telemetry.Logger().Info(ctx, "creating order",
			o11y.LogField("request_id", requestID),
		)

		// Decode order from request body
		var order Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			order = Order{ID: 1, Status: "pending", Value: 100.0}
		}
		order.Status = "pending"

		span.SetAttributes(
			o11y.Attr("order.id", order.ID),
			o11y.Attr("order.value", order.Value),
		)

		// Record metric for order creation
		s.telemetry.Metrics().AddCounter(ctx, "orders.created", 1,
			"status", "pending",
		)

		// Call Payment Service with trace propagation
		paymentCtx, paymentSpan := s.telemetry.Tracer().Start(ctx, "CallPaymentService")
		paymentResponse, err := s.callPaymentService(paymentCtx, order)
		if err != nil {
			paymentSpan.RecordError(err)
			paymentSpan.SetStatus(o11y.SpanStatusError, err.Error())
			paymentSpan.End()

			s.telemetry.Logger().Error(ctx, err, "payment service call failed",
				o11y.LogField("order_id", order.ID),
			)

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "payment failed"})
			return nil
		}
		paymentSpan.SetAttributes(
			o11y.Attr("payment.transaction_id", paymentResponse.TransactionID),
			o11y.Attr("payment.status", paymentResponse.Status),
		)
		paymentSpan.End()

		// Update order status
		order.Status = "paid"
		span.AddEvent("payment_completed", o11y.Attr("transaction_id", paymentResponse.TransactionID))

		// Publish order created event with trace context
		publishCtx, publishSpan := s.telemetry.Tracer().Start(ctx, "PublishOrderCreatedEvent")

		// Extract trace context for propagation via message headers
		traceHeaders := make(map[string]string)
		otel.GetTextMapPropagator().Inject(publishCtx, propagation.MapCarrier(traceHeaders))

		// Add custom headers
		traceHeaders["content_type"] = "application/json"
		traceHeaders["event_type"] = OrderCreated
		traceHeaders["request_id"] = requestID

		orderJSON, err := json.Marshal(order)
		if err != nil {
			publishSpan.RecordError(err)
			publishSpan.End()
			return err
		}

		err = producer.Publish(publishCtx, OrderQueue, OrderCreated, traceHeaders, &messaging.Message{
			Body: orderJSON,
		})
		if err != nil {
			publishSpan.RecordError(err)
			publishSpan.SetStatus(o11y.SpanStatusError, err.Error())
			publishSpan.End()

			s.telemetry.Logger().Error(ctx, err, "failed to publish order event",
				o11y.LogField("order_id", order.ID),
			)
			return err
		}

		publishSpan.SetAttributes(
			o11y.Attr("messaging.system", "rabbitmq"),
			o11y.Attr("messaging.destination", OrderQueue),
			o11y.Attr("messaging.operation", "publish"),
		)
		publishSpan.End()

		s.telemetry.Logger().Info(ctx, "order created successfully",
			o11y.LogField("order_id", order.ID),
			o11y.LogField("transaction_id", paymentResponse.TransactionID),
		)

		// Record duration metric
		s.telemetry.Metrics().AddCounter(ctx, "orders.completed", 1,
			"status", "success",
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		return json.NewEncoder(w).Encode(map[string]any{
			"order":          order,
			"transaction_id": paymentResponse.TransactionID,
			"trace_id":       o11y.TraceIDFromContext(ctx),
		})
	}
}

func (s *apiServer) callPaymentService(ctx context.Context, order Order) (*PaymentResponse, error) {
	paymentURL := getEnv("PAYMENT_SERVICE_URL", "http://localhost:8002")

	paymentReq := PaymentRequest{
		OrderID: order.ID,
		Amount:  order.Value,
	}

	reqBody, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentURL+"/payments", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Inject trace context into HTTP headers for distributed tracing
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payment service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("payment service returned status %d", resp.StatusCode)
	}

	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, fmt.Errorf("failed to decode payment response: %w", err)
	}

	return &paymentResp, nil
}

func (s *apiServer) healthHandler() httpserver.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
