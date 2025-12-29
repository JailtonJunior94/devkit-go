package serverfiber

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	app                *fiber.App
	config             Config
	observability      observability.Observability
	healthChecks       map[string]HealthCheckFunc
	routeTimeouts      map[string]time.Duration
	customMiddlewares  []fiber.Handler
	customErrorHandler fiber.ErrorHandler
	shutdownOnce       sync.Once
}

func New(o11y observability.Observability, opts ...Option) *Server {
	srv := &Server{
		config:        DefaultConfig(),
		observability: o11y,
		healthChecks:  make(map[string]HealthCheckFunc),
		routeTimeouts: make(map[string]time.Duration),
	}

	for _, opt := range opts {
		opt(srv)
	}

	if err := srv.config.Validate(); err != nil {
		panic("invalid server configuration: " + err.Error())
	}

	errorHandler := srv.customErrorHandler
	if errorHandler == nil {
		errorHandler = defaultErrorHandler
	}

	srv.app = fiber.New(fiber.Config{
		AppName:      srv.config.ServiceName,
		ReadTimeout:  srv.config.ReadTimeout,
		WriteTimeout: srv.config.WriteTimeout,
		IdleTimeout:  srv.config.IdleTimeout,
		BodyLimit:    srv.config.BodyLimit,
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

	return srv
}

func (s *Server) registerMiddlewares() {
	s.app.Use(recoverMiddleware(s.observability))
	s.app.Use(requestIDMiddleware())

	if s.config.ReadTimeout > 0 {
		s.app.Use(timeoutMiddleware(s.config.ReadTimeout, s.routeTimeouts))
	}

	s.app.Use(securityHeadersMiddleware())

	if s.config.EnableCORS {
		s.app.Use(corsMiddleware(s.config.CORSOrigins))
		s.observability.Logger().Info(context.Background(), "CORS enabled",
			observability.String("origins", s.config.CORSOrigins),
		)
	}

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
