package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

// UserService demonstrates how to use observability in a service.
type UserService struct {
	obs observability.Observability
}

func NewUserService(obs observability.Observability) *UserService {
	return &UserService{obs: obs}
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetUser demonstrates complete observability usage: tracing, logging, and metrics.
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	// Start a span for tracing
	ctx, span := s.obs.Tracer().Start(ctx, "UserService.GetUser",
		observability.WithSpanKind(observability.SpanKindInternal),
		observability.WithAttributes(
			observability.String("user_id", userID),
		),
	)
	defer span.End()

	// Log the operation
	s.obs.Logger().Info(ctx, "fetching user",
		observability.String("user_id", userID),
	)

	// Increment request counter
	requestCounter := s.obs.Metrics().Counter(
		"user.get.requests",
		"Total number of GetUser requests",
		"1",
	)
	requestCounter.Increment(ctx,
		observability.String("user_id", userID),
	)

	// Record operation latency
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds()
		histogram := s.obs.Metrics().Histogram(
			"user.get.duration",
			"GetUser operation duration",
			"s",
		)
		histogram.Record(ctx, latency,
			observability.String("user_id", userID),
		)
	}()

	// Simulate fetching user from database
	time.Sleep(50 * time.Millisecond)

	// Simulate validation error
	if userID == "invalid" {
		err := fmt.Errorf("user not found")

		// Record error in span
		span.RecordError(err,
			observability.String("user_id", userID),
		)
		span.SetStatus(observability.StatusCodeError, "user not found")

		// Log the error
		s.obs.Logger().Error(ctx, "failed to fetch user",
			observability.Error(err),
			observability.String("user_id", userID),
		)

		// Increment error counter
		errorCounter := s.obs.Metrics().Counter(
			"user.get.errors",
			"Total number of GetUser errors",
			"1",
		)
		errorCounter.Increment(ctx,
			observability.String("error_type", "not_found"),
		)

		return nil, err
	}

	// Success case
	user := &User{
		ID:    userID,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Add event to span
	span.AddEvent("user_fetched",
		observability.String("user_name", user.Name),
	)
	span.SetStatus(observability.StatusCodeOK, "user fetched successfully")

	s.obs.Logger().Info(ctx, "user fetched successfully",
		observability.String("user_id", userID),
		observability.String("user_name", user.Name),
	)

	return user, nil
}

// HTTPHandler wraps the service with HTTP handling and observability.
type HTTPHandler struct {
	service *UserService
	obs     observability.Observability
}

func NewHTTPHandler(service *UserService, obs observability.Observability) *HTTPHandler {
	return &HTTPHandler{
		service: service,
		obs:     obs,
	}
}

// GetUserHandler handles HTTP GET requests with full observability.
func (h *HTTPHandler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	// Start HTTP span
	ctx, span := h.obs.Tracer().Start(r.Context(), "GET /users/:id",
		observability.WithSpanKind(observability.SpanKindServer),
		observability.WithAttributes(
			observability.String("http.method", r.Method),
			observability.String("http.path", r.URL.Path),
			observability.String("http.user_agent", r.UserAgent()),
		),
	)
	defer span.End()

	// Extract user ID from URL
	userID := r.URL.Query().Get("id")
	if userID == "" {
		h.handleError(ctx, w, span, http.StatusBadRequest, "missing user_id parameter")
		return
	}

	// Call service
	user, err := h.service.GetUser(ctx, userID)
	if err != nil {
		h.handleError(ctx, w, span, http.StatusNotFound, err.Error())
		return
	}

	// Success response
	span.SetStatus(observability.StatusCodeOK, "request successful")
	span.SetAttributes(observability.Int("http.status_code", http.StatusOK))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)

	// Record success metric
	successCounter := h.obs.Metrics().Counter(
		"http.requests",
		"Total HTTP requests",
		"1",
	)
	successCounter.Increment(ctx,
		observability.String("method", r.Method),
		observability.String("path", "/users"),
		observability.Int("status", http.StatusOK),
	)
}

func (h *HTTPHandler) handleError(ctx context.Context, w http.ResponseWriter, span observability.Span, statusCode int, message string) {
	err := fmt.Errorf("%s", message)

	// Record error in span
	span.RecordError(err)
	span.SetStatus(observability.StatusCodeError, message)
	span.SetAttributes(observability.Int("http.status_code", statusCode))

	// Log error
	h.obs.Logger().Error(ctx, "HTTP request failed",
		observability.Error(err),
		observability.Int("status_code", statusCode),
	)

	// Increment error counter
	errorCounter := h.obs.Metrics().Counter(
		"http.requests",
		"Total HTTP requests",
		"1",
	)
	errorCounter.Increment(ctx,
		observability.String("method", "GET"),
		observability.String("path", "/users"),
		observability.Int("status", statusCode),
	)

	// Send error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry provider
	config := &otel.Config{
		ServiceName:     "user-api",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		OTLPEndpoint:    "localhost:4317",
		OTLPProtocol:    otel.ProtocolGRPC,
		Insecure:        true, // Only for development
		TraceSampleRate: 1.0,
		LogLevel:        observability.LogLevelInfo,
		LogFormat:       observability.LogFormatJSON,
	}

	obs, err := otel.NewProvider(ctx, config)
	if err != nil {
		log.Fatal("Failed to initialize observability:", err)
	}
	defer obs.Shutdown(ctx)

	// Create service and handler
	service := NewUserService(obs)
	handler := NewHTTPHandler(service, obs)

	// Setup HTTP routes
	http.HandleFunc("/users", handler.GetUserHandler)

	// Start server
	log.Println("Server starting on :8080")
	log.Println("Try: curl 'http://localhost:8080/users?id=123'")
	log.Println("Try: curl 'http://localhost:8080/users?id=invalid'")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
