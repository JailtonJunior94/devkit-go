package consumer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Consumer defines the interface for a message consumer server.
// It follows the same lifecycle pattern as pkg/http_server, serving as
// an entrypoint for applications that consume messages from brokers
// (Kafka, RabbitMQ, etc.).
type Consumer interface {
	// Start begins consuming messages and blocks until the context is cancelled
	// or an error occurs. It handles OS signals (SIGINT, SIGTERM) for graceful shutdown.
	Start(ctx context.Context) error

	// Shutdown gracefully stops the consumer, waiting for in-flight messages
	// to complete processing within the provided context timeout.
	Shutdown(ctx context.Context) error

	// Health returns the current health status of the consumer and its dependencies.
	Health(ctx context.Context) HealthStatus

	// RegisterHandlers registers message handlers for specific topics.
	// Returns the consumer for method chaining.
	RegisterHandlers(handlers ...Handler) Consumer
}

// Server implements the Consumer interface, providing a production-ready
// message consumer with lifecycle management, health checks, graceful shutdown,
// and observability integration.
type Server struct {
	config        Config
	observability observability.Observability

	// Handler registry and processing
	handlers     map[string][]MessageHandler
	handlerMutex sync.RWMutex

	// Middleware chain
	middlewares []Middleware

	// Health checks
	healthChecks      map[string]HealthCheckFunc
	healthChecksMutex sync.RWMutex

	// Worker pool management
	workers      sync.WaitGroup
	stopWorkers  context.CancelFunc
	workerCtx    context.Context
	isRunning    atomic.Bool
	shutdownOnce sync.Once

	// Message processing
	errorChan chan error
}

// New creates a new Consumer server with the provided observability
// and optional configuration. It panics if the configuration is invalid,
// following the same pattern as pkg/http_server.
func New(o11y observability.Observability, opts ...Option) Consumer {
	if o11y == nil {
		panic("observability is required")
	}

	// Start with default configuration
	config := DefaultConfig()

	// Create server instance
	server := &Server{
		config:        config,
		observability: o11y,
		handlers:      make(map[string][]MessageHandler),
		middlewares:   make([]Middleware, 0),
		healthChecks:  make(map[string]HealthCheckFunc),
		errorChan:     make(chan error, 1),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(server)
	}

	// Validate configuration after options are applied
	if err := server.config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid consumer configuration: %v", err))
	}

	return server
}

// RegisterHandlers registers message handlers for processing messages.
// Handlers can be registered for specific topics or patterns.
func (s *Server) RegisterHandlers(handlers ...Handler) Consumer {
	ctx := context.Background()
	for _, handler := range handlers {
		handler.Register(s)
		s.observability.Logger().Info(ctx, "handler registered",
			observability.String("handler_type", fmt.Sprintf("%T", handler)))
	}
	return s
}

// registerMessageHandler is an internal method used by handlers to register themselves.
func (s *Server) registerMessageHandler(topic string, handler MessageHandler) {
	s.handlerMutex.Lock()
	defer s.handlerMutex.Unlock()

	if s.handlers[topic] == nil {
		s.handlers[topic] = make([]MessageHandler, 0)
	}

	s.handlers[topic] = append(s.handlers[topic], handler)
}

// getHandlers retrieves all registered handlers for a given topic.
func (s *Server) getHandlers(topic string) []MessageHandler {
	s.handlerMutex.RLock()
	defer s.handlerMutex.RUnlock()

	handlers := s.handlers[topic]
	if handlers == nil {
		return []MessageHandler{}
	}

	// Return a copy to avoid concurrent modification
	result := make([]MessageHandler, len(handlers))
	copy(result, handlers)
	return result
}

// addHealthCheck adds a health check function with the given name.
func (s *Server) addHealthCheck(name string, check HealthCheckFunc) {
	s.healthChecksMutex.Lock()
	defer s.healthChecksMutex.Unlock()
	s.healthChecks[name] = check
}

// getAllHealthChecks returns a copy of all registered health checks.
func (s *Server) getAllHealthChecks() map[string]HealthCheckFunc {
	s.healthChecksMutex.RLock()
	defer s.healthChecksMutex.RUnlock()

	checks := make(map[string]HealthCheckFunc, len(s.healthChecks))
	for name, check := range s.healthChecks {
		checks[name] = check
	}
	return checks
}
