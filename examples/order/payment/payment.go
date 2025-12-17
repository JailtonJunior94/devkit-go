package payment

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type PaymentRequest struct {
	OrderID int     `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type PaymentResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	OrderID       int    `json:"order_id"`
	Amount        float64 `json:"amount"`
	TraceID       string `json:"trace_id"`
}

type paymentServer struct {
	telemetry o11y.Telemetry
}

func NewPaymentServer() *paymentServer {
	return &paymentServer{}
}

func (s *paymentServer) Run() {
	ctx := context.Background()

	// Setup telemetry (tracer, metrics, logger)
	cfg := o11y.Config{
		ServiceName:     "payment-service",
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

	routes := []fiberserver.Route{
		fiberserver.NewRoute(fiber.MethodPost, "/payments", s.processPaymentHandler()),
		fiberserver.NewRoute(fiber.MethodGet, "/health", s.healthHandler()),
	}

	server := fiberserver.New(
		fiberserver.WithPort(getEnv("PORT", "8002")),
		fiberserver.WithRoutes(routes...),
		fiberserver.WithMiddlewares(
			fiberserver.RequestID,
			fiberserver.SecurityHeaders,
			fiberserver.Recovery,
			fiberserver.Logger,
		),
	)

	shutdown := server.Run()
	telemetry.Logger().Info(ctx, "payment-service started", o11y.LogField("port", getEnv("PORT", "8002")))

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := <-server.ShutdownListener(); err != nil {
			interrupt <- syscall.SIGTERM
		}
	}()

	<-interrupt
	telemetry.Logger().Info(ctx, "shutting down payment-service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func (s *paymentServer) processPaymentHandler() fiberserver.Handler {
	return func(c *fiber.Ctx) error {
		// Extract trace context from incoming HTTP headers
		ctx := context.Background()
		carrier := make(propagation.HeaderCarrier)
		c.Request().Header.VisitAll(func(key, value []byte) {
			carrier.Set(string(key), string(value))
		})
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

		requestID := fiberserver.GetRequestID(c)

		// Start span for payment processing (child of incoming trace)
		ctx, span := s.telemetry.Tracer().Start(ctx, "ProcessPayment",
			o11y.Attr("request_id", requestID),
		)
		defer span.End()

		s.telemetry.Logger().Info(ctx, "processing payment",
			o11y.LogField("request_id", requestID),
			o11y.LogField("trace_id", o11y.TraceIDFromContext(ctx)),
		)

		// Parse request
		var req PaymentRequest
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			span.RecordError(err)
			span.SetStatus(o11y.SpanStatusError, "invalid request body")
			s.telemetry.Logger().Error(ctx, err, "failed to parse payment request")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		span.SetAttributes(
			o11y.Attr("payment.order_id", req.OrderID),
			o11y.Attr("payment.amount", req.Amount),
		)

		// Validate payment
		validateCtx, validateSpan := s.telemetry.Tracer().Start(ctx, "ValidatePayment")
		if err := s.validatePayment(validateCtx, req); err != nil {
			validateSpan.RecordError(err)
			validateSpan.SetStatus(o11y.SpanStatusError, err.Error())
			validateSpan.End()

			s.telemetry.Metrics().AddCounter(ctx, "payments.failed", 1,
				"reason", "validation_failed",
			)

			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		validateSpan.End()

		// Process payment (simulate external call)
		processCtx, processSpan := s.telemetry.Tracer().Start(ctx, "ExecutePaymentTransaction")
		transactionID, err := s.executePayment(processCtx, req)
		if err != nil {
			processSpan.RecordError(err)
			processSpan.SetStatus(o11y.SpanStatusError, err.Error())
			processSpan.End()

			s.telemetry.Metrics().AddCounter(ctx, "payments.failed", 1,
				"reason", "transaction_failed",
			)

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "payment processing failed",
			})
		}
		processSpan.SetAttributes(o11y.Attr("transaction_id", transactionID))
		processSpan.AddEvent("payment_completed")
		processSpan.End()

		// Record success metrics
		s.telemetry.Metrics().AddCounter(ctx, "payments.success", 1)
		s.telemetry.Metrics().RecordHistogram(ctx, "payments.amount", req.Amount)

		response := PaymentResponse{
			TransactionID: transactionID,
			Status:        "approved",
			OrderID:       req.OrderID,
			Amount:        req.Amount,
			TraceID:       o11y.TraceIDFromContext(ctx),
		}

		span.SetAttributes(
			o11y.Attr("payment.transaction_id", transactionID),
			o11y.Attr("payment.status", "approved"),
		)

		s.telemetry.Logger().Info(ctx, "payment processed successfully",
			o11y.LogField("transaction_id", transactionID),
			o11y.LogField("order_id", req.OrderID),
			o11y.LogField("amount", req.Amount),
		)

		return c.Status(fiber.StatusOK).JSON(response)
	}
}

func (s *paymentServer) validatePayment(ctx context.Context, req PaymentRequest) error {
	// Simulate validation logic
	if req.Amount <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "amount must be positive")
	}
	if req.OrderID <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid order ID")
	}

	s.telemetry.Logger().Info(ctx, "payment validated",
		o11y.LogField("order_id", req.OrderID),
	)

	return nil
}

func (s *paymentServer) executePayment(ctx context.Context, req PaymentRequest) (string, error) {
	// Simulate payment processing time
	time.Sleep(50 * time.Millisecond)

	// Generate transaction ID
	transactionID := uuid.New().String()

	s.telemetry.Logger().Info(ctx, "payment executed",
		o11y.LogField("transaction_id", transactionID),
		o11y.LogField("order_id", req.OrderID),
	)

	return transactionID, nil
}

func (s *paymentServer) healthHandler() fiberserver.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
