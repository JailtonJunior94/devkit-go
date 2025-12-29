package chiserver

import (
	"net/http"
	"strings"
	"time"
)

// Option is a function that configures a Server.
type Option func(*Server)

// WithConfig sets the full configuration for the server.
func WithConfig(cfg Config) Option {
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
