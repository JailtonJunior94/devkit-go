package chiserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
)

// Option is a function that configures a Server.
type Option func(*Server)

// WithConfig sets the full configuration for the server.
func WithConfig(cfg common.Config) Option {
	return func(s *Server) {
		s.config = cfg
	}
}

// WithPort sets the server port.
func WithPort(port string) Option {
	return func(s *Server) {
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		s.config.Address = port
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.WriteTimeout = timeout
	}
}

// WithIdleTimeout sets the idle timeout.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.IdleTimeout = timeout
	}
}

// WithBodyLimit sets the maximum request body size in bytes.
func WithBodyLimit(limit int) Option {
	return func(s *Server) {
		s.config.BodyLimit = limit
	}
}

// WithCORS enables CORS with the specified origins.
func WithCORS(origins string) Option {
	return func(s *Server) {
		s.config.EnableCORS = true
		s.config.CORSOrigins = origins
	}
}

// WithMetrics enables the /metrics endpoint.
func WithMetrics() Option {
	return func(s *Server) {
		s.config.EnableMetrics = true
	}
}

// WithHealthChecks registers health checks.
func WithHealthChecks(checks map[string]HealthCheckFunc) Option {
	return func(s *Server) {
		s.config.EnableHealthChecks = true
		for name, check := range checks {
			s.healthChecks[name] = check
		}
	}
}

// WithMiddleware adds a custom middleware to the server.
func WithMiddleware(middleware func(http.Handler) http.Handler) Option {
	return func(s *Server) {
		s.customMiddlewares = append(s.customMiddlewares, middleware)
	}
}

// WithRouteTimeout sets a timeout for a specific route.
func WithRouteTimeout(path string, timeout time.Duration) Option {
	return func(s *Server) {
		s.routeTimeouts[path] = timeout
	}
}

// WithServiceName sets the service name.
func WithServiceName(name string) Option {
	return func(s *Server) {
		s.config.ServiceName = name
	}
}

// WithServiceVersion sets the service version.
func WithServiceVersion(version string) Option {
	return func(s *Server) {
		s.config.ServiceVersion = version
	}
}

// WithEnvironment sets the environment.
func WithEnvironment(env string) Option {
	return func(s *Server) {
		s.config.Environment = env
	}
}

// WithTracing enables shared HTTP distributed tracing.
func WithTracing() Option {
	return func(s *Server) {
		s.config.EnableTracing = true
	}
}

// WithOTelMetrics enables shared OpenTelemetry HTTP metrics.
func WithOTelMetrics() Option {
	return func(s *Server) {
		s.config.EnableOTelMetrics = true
	}
}

// WithErrorHandler overrides the default ErrorHandler used by
// handlers registered through Server.RegisterHandler. Errors returned
// by the user Handler are passed to fn together with the request
// context and the underlying http.ResponseWriter.
func WithErrorHandler(fn ErrorHandler) Option {
	return func(s *Server) {
		s.errorHandler = fn
	}
}

// WithShutdownTimeout sets the deadline used by Shutdown when invoked
// from Start. The timeout is applied via context.WithTimeout deriving
// from the parent context, preserving its deadline (RF-8.2, RF-8.3).
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) {
		s.config.ShutdownTimeout = d
	}
}

// WithTimeoutCleanup configures how long the per-route timeout middleware
// waits for the handler goroutine to drain after a 408 has been written.
// Passing d <= 0 disables the leak detector entirely: the timeout response
// is still sent but http_handler_timeout_leak_total is never incremented
// even if the handler ignores ctx.Done() (RF-4.6). The default applied by
// New is 100ms when this option is not provided (RF-4.5).
func WithTimeoutCleanup(d time.Duration) Option {
	return func(s *Server) {
		s.timeoutCleanup = d
	}
}
