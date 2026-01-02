package chiserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents an HTTP server using Chi router.
type Server struct {
	router            chi.Router
	httpServer        *http.Server
	config            Config
	observability     observability.Observability
	healthChecks      map[string]HealthCheckFunc
	routeTimeouts     map[string]time.Duration
	customMiddlewares []func(http.Handler) http.Handler
	shutdownOnce      sync.Once
}

// New creates a new HTTP server with the given options.
func New(o11y observability.Observability, opts ...Option) (*Server, error) {
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
		return nil, fmt.Errorf("invalid server configuration: %w", err)
	}

	srv.router = chi.NewRouter()
	srv.registerMiddlewares()
	srv.registerSupportEndpoints()

	srv.httpServer = &http.Server{
		Addr:         srv.config.Address,
		Handler:      srv.router,
		ReadTimeout:  srv.config.ReadTimeout,
		WriteTimeout: srv.config.WriteTimeout,
		IdleTimeout:  srv.config.IdleTimeout,
	}

	return srv, nil
}

// RegisterRouters registers route handlers with the server.
func (s *Server) RegisterRouters(routers ...Router) *Server {
	for _, router := range routers {
		router.Register(s.router)
		s.observability.Logger().Info(context.Background(), "router registered")
	}

	return s
}

// registerMiddlewares registers all middlewares in the correct order.
func (s *Server) registerMiddlewares() {
	s.router.Use(recoverMiddleware(s.observability))
	s.router.Use(requestIDMiddleware())
	s.router.Use(bodyLimitMiddleware(int64(s.config.BodyLimit)))

	if len(s.routeTimeouts) > 0 || s.config.ReadTimeout > 0 {
		s.router.Use(timeoutMiddleware(s.config.ReadTimeout, s.routeTimeouts))
	}

	s.router.Use(securityHeadersMiddleware())

	if s.config.EnableCORS {
		s.router.Use(corsMiddleware(s.config.CORSOrigins))
		s.observability.Logger().Info(context.Background(), "CORS enabled",
			observability.String("origins", s.config.CORSOrigins))
	}

	for _, middleware := range s.customMiddlewares {
		s.router.Use(middleware)
	}
}

// registerSupportEndpoints registers health checks and metrics endpoints.
func (s *Server) registerSupportEndpoints() {
	if s.config.EnableHealthChecks {
		s.router.Get("/health", healthHandler(s.config, s.healthChecks, s.observability))
		s.router.Get("/ready", readyHandler(s.healthChecks, s.observability))
		s.router.Get("/live", liveHandler())
		s.observability.Logger().Info(context.Background(), "health check endpoints enabled")
	}

	if s.config.EnableMetrics {
		s.router.Handle("/metrics", promhttp.Handler())
		s.observability.Logger().Info(context.Background(), "metrics endpoint enabled")
	}
}
