package serverfiber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

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
		errorHandler = defaultErrorHandler(o11y)
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

	// 2. Request ID middleware - must run before observability to propagate request IDs
	s.app.Use(requestIDMiddleware(s.observability))

	// 3. Shared HTTP observability - tracing and metrics are delegated to the runtime hook
	if s.config.EnableTracing || s.config.EnableOTelMetrics {
		s.app.Use(observabilityMiddleware(s.observability))
		s.observability.Logger().Info(context.Background(), "shared HTTP observability enabled",
			observability.String("service", s.config.ServiceName),
			observability.Bool("tracing", s.config.EnableTracing),
			observability.Bool("metrics", s.config.EnableOTelMetrics),
		)
	}

	// 4. Global timeout middleware via the official fiber middleware/timeout.
	// Applies cfg.ReadTimeout to the entire chain as a fallback for routes
	// registered directly through *fiber.App (e.g., RegisterRouters). Routes
	// registered via Server.RegisterHandler get an additional inner wrap with
	// a per-route timeout when WithRouteTimeout was set; the inner shorter
	// deadline derives from the outer ctx and fires first as expected.
	if s.config.ReadTimeout > 0 {
		s.app.Use(makeTimeoutHandler(s.config.ReadTimeout, func(c *fiber.Ctx) error {
			return c.Next()
		}))
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

	// 7. Custom middlewares - user-defined middlewares registered last
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

// RegisterHandler installs handler at method+path. When WithRouteTimeout(path, d)
// was applied before this call, the handler is wrapped with
// makeTimeoutHandler(d, handler) so that route gets the shorter per-route
// deadline. Otherwise no inner wrap is applied: the global timeout middleware
// already installed in registerMiddlewares (when cfg.ReadTimeout > 0) covers
// the route, and double-wrapping with the same ReadTimeout would only churn
// context derivations without changing semantics.
//
// The official fiber timeout middleware does NOT interrupt a hung handler
// (see makeTimeoutHandler godoc): handlers must honor c.UserContext().Done()
// to surface a 408. cfg.WriteTimeout on *fiber.App is the last-resort guard
// for handlers that ignore cancellation.
func (s *Server) RegisterHandler(method, path string, handler fiber.Handler) *Server {
	if rt, ok := s.routeTimeouts[path]; ok {
		handler = makeTimeoutHandler(rt, handler)
	}
	s.app.Add(method, path, handler)
	return s
}

func (s *Server) App() *fiber.App {
	return s.app
}
