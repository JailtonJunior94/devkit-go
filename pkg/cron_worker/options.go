package cron_worker

import "time"

// Option é uma função que configura o worker
type Option func(*Config)

// WithServiceName configura o nome do serviço
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.ServiceName = name
	}
}

// WithSeconds habilita precisão de segundos nos cron schedules
func WithSeconds(enabled bool) Option {
	return func(c *Config) {
		c.WithSeconds = enabled
	}
}

// WithLocation configura o timezone do scheduler
func WithLocation(location *time.Location) Option {
	return func(c *Config) {
		c.Location = location
	}
}

// WithShutdownTimeout configura o timeout de shutdown
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ShutdownTimeout = timeout
	}
}

// WithJobTimeout configura o timeout de execução de jobs
func WithJobTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.JobTimeout = timeout
	}
}

// WithHealthCheck habilita/desabilita health checks
func WithHealthCheck(enabled bool) Option {
	return func(c *Config) {
		c.EnableHealthCheck = enabled
	}
}

// WithHealthChecks adiciona health checks customizados
func WithHealthChecks(checks ...HealthCheck) Option {
	return func(c *Config) {
		c.HealthChecks = append(c.HealthChecks, checks...)
	}
}

// WithMetrics habilita coleta de métricas
func WithMetrics(enabled bool, namespace string) Option {
	return func(c *Config) {
		c.EnableMetrics = enabled
		if namespace != "" {
			c.MetricsNamespace = namespace
		}
	}
}

// WithTracing habilita tracing
func WithTracing(enabled bool) Option {
	return func(c *Config) {
		c.EnableTracing = enabled
	}
}

// WithMaxConcurrentJobs configura o número máximo de jobs concorrentes
func WithMaxConcurrentJobs(max int) Option {
	return func(c *Config) {
		c.MaxConcurrentJobs = max
	}
}

// WithRecovery habilita/desabilita recuperação de panics
func WithRecovery(enabled bool) Option {
	return func(c *Config) {
		c.RecoveryEnabled = enabled
	}
}

// WithOnJobStart configura callback de início de job
func WithOnJobStart(callback func(jobName string)) Option {
	return func(c *Config) {
		c.OnJobStart = callback
	}
}

// WithOnJobComplete configura callback de conclusão de job
func WithOnJobComplete(callback func(jobName string, duration time.Duration, err error)) Option {
	return func(c *Config) {
		c.OnJobComplete = callback
	}
}

// WithOnJobPanic configura callback de panic de job
func WithOnJobPanic(callback func(jobName string, recovered interface{})) Option {
	return func(c *Config) {
		c.OnJobPanic = callback
	}
}
