package serverfiber

import (
	"maps"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/gofiber/fiber/v2"
)

type Option func(*Server)

func WithConfig(cfg common.Config) Option {
	return func(s *Server) {
		s.config = cfg
	}
}

func WithPort(port string) Option {
	return func(s *Server) {
		if port != "" && port[0] != ':' {
			port = ":" + port
		}
		s.config.Address = port
	}
}

func WithReadTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.ReadTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.WriteTimeout = timeout
	}
}

func WithIdleTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.IdleTimeout = timeout
	}
}

// WithShutdownTimeout sets the maximum time the server waits for graceful
// shutdown before returning. Mirrors chi_server.WithShutdownTimeout for
// adapter parity (RF-8.2, RF-9.1).
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.ShutdownTimeout = timeout
	}
}

func WithBodyLimit(limit int) Option {
	return func(s *Server) {
		s.config.BodyLimit = limit
	}
}

func WithCORS(origins string) Option {
	return func(s *Server) {
		s.config.EnableCORS = true
		s.config.CORSOrigins = origins
	}
}

func WithMetrics() Option {
	return func(s *Server) {
		s.config.EnableMetrics = true
	}
}

func WithHealthChecks(checks map[string]HealthCheckFunc) Option {
	return func(s *Server) {
		s.config.EnableHealthChecks = true
		if checks != nil {
			if s.healthChecks == nil {
				s.healthChecks = make(map[string]HealthCheckFunc)
			}
			maps.Copy(s.healthChecks, checks)
		}
	}
}

func WithErrorHandler(handler fiber.ErrorHandler) Option {
	return func(s *Server) {
		s.customErrorHandler = handler
	}
}

func WithMiddleware(middleware fiber.Handler) Option {
	return func(s *Server) {
		s.customMiddlewares = append(s.customMiddlewares, middleware)
	}
}

func WithRouteTimeout(path string, timeout time.Duration) Option {
	return func(s *Server) {
		if s.routeTimeouts == nil {
			s.routeTimeouts = make(map[string]time.Duration)
		}
		s.routeTimeouts[path] = timeout
	}
}

func WithServiceName(name string) Option {
	return func(s *Server) {
		s.config.ServiceName = name
	}
}

func WithServiceVersion(version string) Option {
	return func(s *Server) {
		s.config.ServiceVersion = version
	}
}

// WithEnvironment sets the environment label propagated to logs and health
// payloads (development, staging, production, ...). Mirrors
// chi_server.WithEnvironment for adapter parity (RF-9.1).
func WithEnvironment(env string) Option {
	return func(s *Server) {
		s.config.Environment = env
	}
}

func WithTracing() Option {
	return func(s *Server) {
		s.config.EnableTracing = true
	}
}

func WithOTelMetrics() Option {
	return func(s *Server) {
		s.config.EnableOTelMetrics = true
	}
}
