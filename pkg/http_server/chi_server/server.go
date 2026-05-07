package chiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// defaultTimeoutCleanup is the default cleanup window applied to per-route
// timeouts when WithTimeoutCleanup is not provided. The value matches the
// budget assumed by RF-4.5 and balances responsiveness with the cost of
// running an extra timer per leaked goroutine.
const defaultTimeoutCleanup = 100 * time.Millisecond

// Server represents an HTTP server using Chi router.
type Server struct {
	router            chi.Router
	httpServer        *http.Server
	config            common.Config
	observability     observability.Observability
	healthChecks      map[string]HealthCheckFunc
	routeTimeouts     map[string]time.Duration
	customMiddlewares []func(http.Handler) http.Handler
	errorHandler      ErrorHandler
	leakCounter       observability.Counter
	timeoutCleanup    time.Duration
	shutdownOnce      sync.Once
}

// New creates a new HTTP server with the given options.
func New(o11y observability.Observability, opts ...Option) (*Server, error) {
	srv := &Server{
		config:         common.DefaultConfig(),
		observability:  o11y,
		healthChecks:   make(map[string]HealthCheckFunc),
		routeTimeouts:  make(map[string]time.Duration),
		timeoutCleanup: defaultTimeoutCleanup,
	}

	for _, opt := range opts {
		opt(srv)
	}

	if err := srv.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server configuration: %w", err)
	}

	srv.leakCounter = newLeakCounter(o11y)

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

// RegisterHandler registers a Handler that returns an error against the
// given method/path combination. Middlewares are applied in declaration
// order (first one becomes the outermost wrapper). When the path has a
// per-route timeout configured via WithRouteTimeout, wrapWithTimeout is
// installed at registration time.
func (s *Server) RegisterHandler(method, path string, h Handler, mws ...Middleware) *Server {
	wrapped := http.Handler(adaptHandler(h, s.resolveErrorHandler()))
	for i := len(mws) - 1; i >= 0; i-- {
		wrapped = mws[i](wrapped)
	}
	if d, ok := s.routeTimeouts[path]; ok {
		wrapped = wrapWithTimeout(s, d, wrapped, path)
	}
	s.router.Method(method, path, wrapped)
	return s
}

// resolveErrorHandler returns the user-provided ErrorHandler when set,
// otherwise the default sanitizing handler.
func (s *Server) resolveErrorHandler() ErrorHandler {
	if s.errorHandler != nil {
		return s.errorHandler
	}
	return s.defaultErrorHandler
}

// defaultErrorHandler logs the original error via pkg/observability and
// writes an RFC 7807 application/problem+json response derived from
// common.ProblemFromError. The raw error message is never reflected
// back to the client (R-SEC-001).
func (s *Server) defaultErrorHandler(ctx context.Context, w http.ResponseWriter, err error) {
	requestID, _ := ctx.Value(requestIDKey).(string)
	instance := requestPath(ctx)

	s.observability.Logger().Error(ctx, "http handler error",
		observability.Error(err),
		observability.String("request_id", requestID),
		observability.String("path", instance),
	)

	problem := common.ProblemFromError(err, instance, requestID)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

// registerMiddlewares registers all middlewares in the correct order.
//
// Ordering note: securityHeadersMiddleware and corsMiddleware run BEFORE
// timeoutMiddleware. timeoutMiddleware spawns a goroutine and serves the
// remaining chain through a buffered timeoutWriter; if security/cors ran
// inside that goroutine they would set headers on tw.h which is dropped
// when the parent goroutine writes the 408 directly to w. Placing them
// upstream guarantees the 408 response carries security/cors headers
// without sharing tw.h between goroutines (no extra synchronization).
func (s *Server) registerMiddlewares() {
	s.router.Use(recoverMiddleware(s.observability))
	s.router.Use(requestIDMiddleware(s.observability))
	if s.config.EnableTracing || s.config.EnableOTelMetrics {
		s.router.Use(observabilityMiddleware(s.router, s.observability))
		s.observability.Logger().Info(context.Background(), "shared HTTP observability enabled",
			observability.Bool("tracing", s.config.EnableTracing),
			observability.Bool("metrics", s.config.EnableOTelMetrics))
	}
	s.router.Use(bodyLimitMiddleware(int64(s.config.BodyLimit)))

	s.router.Use(securityHeadersMiddleware())

	if s.config.EnableCORS {
		s.router.Use(corsMiddleware(s.config.CORSOrigins))
		s.observability.Logger().Info(context.Background(), "CORS enabled",
			observability.String("origins", s.config.CORSOrigins))
	}

	if s.config.ReadTimeout > 0 {
		// Global fallback timeout for handlers registered directly against
		// chi.Router (without going through Server.RegisterHandler). Per-route
		// timeouts are installed by RegisterHandler via wrapWithTimeout, so
		// this middleware deliberately ignores routeTimeouts (RF-5.4).
		s.router.Use(timeoutMiddleware(s.config.ReadTimeout))
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

// recordTimeoutLeak emits a structured warning and increments the leak
// counter for a per-route timeout whose handler goroutine did not return
// within s.timeoutCleanup. The counter is only touched when wired (RF-4.3);
// callers that disable cleanup (s.timeoutCleanup <= 0) MUST NOT invoke this
// function, ensuring RF-4.6 (cleanup=0 never increments).
func (s *Server) recordTimeoutLeak(ctx context.Context, route, path string) {
	requestID, _ := ctx.Value(requestIDKey).(string)
	s.observability.Logger().Warn(ctx, "http handler timeout leak",
		observability.String("request_id", requestID),
		observability.String("route", route),
		observability.String("path", path),
	)
	if s.leakCounter != nil {
		s.leakCounter.Increment(ctx,
			observability.String("adapter", "chi"),
			observability.String("route", route),
		)
	}
}

// newLeakCounter resolves the OTel counter used to publish timeout leaks.
// It returns nil when the provider does not expose a metrics surface so
// recordTimeoutLeak can short-circuit the increment (RF-4.3).
func newLeakCounter(o11y observability.Observability) observability.Counter {
	metrics := o11y.Metrics()
	if metrics == nil {
		return nil
	}
	return metrics.Counter(
		"http_handler_timeout_leak_total",
		"Total per-route timeouts whose handler goroutine did not drain within the cleanup window",
		"1",
	)
}
