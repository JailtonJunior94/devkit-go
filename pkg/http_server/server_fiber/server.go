package serverfiber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	app                *fiber.App
	config             common.Config
	observability      observability.Observability
	healthChecks       map[string]HealthCheckFunc
	routeTimeouts      map[string]time.Duration
	customMiddlewares  []fiber.Handler
	customErrorHandler fiber.ErrorHandler
	shutdownOnce       sync.Once
}

func New(o11y observability.Observability, opts ...Option) (*Server, error) {
	srv := &Server{
		config:        common.DefaultConfig(),
		observability: o11y,
		healthChecks:  make(map[string]HealthCheckFunc),
		routeTimeouts: make(map[string]time.Duration),
	}

	for _, opt := range opts {
		opt(srv)
	}

	if err := srv.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server configuration: %w", err)
	}

	errorHandler := srv.customErrorHandler
	if errorHandler == nil {
		errorHandler = defaultErrorHandler
	}

	// SECURITY: BodyLimit is enforced natively by Fiber
	// This prevents DOS attacks via large request bodies
	// Fiber will automatically reject requests exceeding this limit
	srv.app = fiber.New(fiber.Config{
		AppName:      srv.config.ServiceName,
		ReadTimeout:  srv.config.ReadTimeout,
		WriteTimeout: srv.config.WriteTimeout,
		IdleTimeout:  srv.config.IdleTimeout,
		BodyLimit:    srv.config.BodyLimit, // Enforced by Fiber natively
		ErrorHandler: errorHandler,
	})

	srv.registerMiddlewares()

	if srv.config.EnableMetrics {
		srv.app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
		o11y.Logger().Info(context.Background(), "Prometheus metrics endpoint enabled",
			observability.String("endpoint", "/metrics"),
		)
	}

	if srv.config.EnableHealthChecks {
		registerHealthChecks(srv.app, srv.config, o11y, srv.healthChecks)
		o11y.Logger().Info(context.Background(), "health check endpoints enabled",
			observability.String("health", "/health"),
			observability.String("ready", "/ready"),
			observability.String("live", "/live"),
		)
	}

	return srv, nil
}

func (s *Server) registerMiddlewares() {
	// 1. Recover middleware - must be first to capture panics from all other middlewares
	s.app.Use(recoverMiddleware(s.observability))

	// 2. OpenTelemetry tracing - create root span early to capture full request lifecycle
	if s.config.EnableTracing {
		s.app.Use(otelfiber.Middleware(
			otelfiber.WithServerName(s.config.ServiceName),
		))
		s.observability.Logger().Info(context.Background(), "OpenTelemetry tracing enabled",
			observability.String("service", s.config.ServiceName),
		)
	}

	// 3. Request ID middleware - can use trace ID from span context if needed
	s.app.Use(requestIDMiddleware())

	// 4. Timeout middleware - enforce request timeouts
	if s.config.ReadTimeout > 0 {
		s.app.Use(timeoutMiddleware(s.config.ReadTimeout, s.routeTimeouts))
	}

	// 5. Security headers middleware
	s.app.Use(securityHeadersMiddleware())

	// 6. CORS middleware - handle cross-origin requests
	if s.config.EnableCORS {
		s.app.Use(corsMiddleware(s.config.CORSOrigins))
		s.observability.Logger().Info(context.Background(), "CORS enabled",
			observability.String("origins", s.config.CORSOrigins),
		)
	}

	// 7. OpenTelemetry metrics - record HTTP metrics (duration, count, active requests)
	if s.config.EnableOTelMetrics {
		s.app.Use(otelMetricsMiddleware(s.config.ServiceName))
		s.observability.Logger().Info(context.Background(), "OpenTelemetry HTTP metrics enabled",
			observability.String("service", s.config.ServiceName),
		)
	}

	// 8. Custom middlewares - user-defined middlewares registered last
	for _, middleware := range s.customMiddlewares {
		s.app.Use(middleware)
	}
}

func (s *Server) RegisterRouters(routers ...Router) *Server {
	for _, router := range routers {
		router.Register(s.app)
	}

	s.observability.Logger().Info(context.Background(), "routers registered", observability.Int("count", len(routers)))

	return s
}

func (s *Server) App() *fiber.App {
	return s.app
}
