package consumer

import "time"

// Option is a functional option for configuring the Consumer server.
// It follows the same pattern as pkg/http_server for consistency.
type Option func(*Server)

// WithConfig sets the complete configuration, overriding defaults.
// This is useful for loading configuration from external sources.
func WithConfig(config Config) Option {
	return func(s *Server) {
		s.config = config
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

// WithEnvironment sets the deployment environment.
func WithEnvironment(env string) Option {
	return func(s *Server) {
		s.config.Environment = env
	}
}

// WithTopics sets the list of topics to consume from.
func WithTopics(topics ...string) Option {
	return func(s *Server) {
		s.config.Topics = topics
	}
}

// WithWorkerCount sets the number of concurrent workers.
func WithWorkerCount(count int) Option {
	return func(s *Server) {
		s.config.WorkerCount = count
	}
}

// WithBatchSize sets the maximum number of messages per batch.
func WithBatchSize(size int) Option {
	return func(s *Server) {
		s.config.BatchSize = size
	}
}

// WithProcessingTimeout sets the timeout for processing a single message.
func WithProcessingTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.ProcessingTimeout = timeout
	}
}

// WithShutdownTimeout sets the timeout for graceful shutdown.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.config.ShutdownTimeout = timeout
	}
}

// WithCommitInterval sets how often to commit offsets.
func WithCommitInterval(interval time.Duration) Option {
	return func(s *Server) {
		s.config.CommitInterval = interval
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(retries int) Option {
	return func(s *Server) {
		s.config.MaxRetries = retries
	}
}

// WithRetryBackoff sets the base duration for exponential backoff.
func WithRetryBackoff(backoff time.Duration) Option {
	return func(s *Server) {
		s.config.RetryBackoff = backoff
	}
}

// WithHealthChecks enables health checks and registers custom check functions.
func WithHealthChecks(checks map[string]HealthCheckFunc) Option {
	return func(s *Server) {
		s.config.EnableHealthChecks = true
		for name, check := range checks {
			s.addHealthCheck(name, check)
		}
	}
}

// WithMetrics enables or disables metrics collection.
func WithMetrics(enabled bool) Option {
	return func(s *Server) {
		s.config.EnableMetrics = enabled
	}
}

// WithDLQ enables dead letter queue with the specified topic.
func WithDLQ(dlqTopic string) Option {
	return func(s *Server) {
		s.config.EnableDLQ = true
		s.config.DLQTopic = dlqTopic
	}
}

// WithMiddleware adds message middleware to the processing chain.
// Middleware is executed in the order it is registered.
func WithMiddleware(middleware ...Middleware) Option {
	return func(s *Server) {
		s.middlewares = append(s.middlewares, middleware...)
	}
}
