package cron_worker

import (
	"errors"
	"time"
)

// Config contém as configurações do worker de cron.
type Config struct {
	// ServiceName é o nome do serviço
	ServiceName string

	// WithSeconds habilita precisão de segundos nos cron schedules
	WithSeconds bool

	// Location define o timezone para o scheduler
	Location *time.Location

	// ShutdownTimeout é o tempo máximo para aguardar jobs em execução
	ShutdownTimeout time.Duration

	// JobTimeout é o tempo máximo de execução de um job
	JobTimeout time.Duration

	// EnableHealthCheck habilita o health check
	EnableHealthCheck bool

	// HealthChecks são verificações customizadas de saúde
	HealthChecks []HealthCheck

	// EnableMetrics habilita coleta de métricas
	EnableMetrics bool

	// MetricsNamespace é o namespace para as métricas
	MetricsNamespace string

	// EnableTracing habilita tracing
	EnableTracing bool

	// MaxConcurrentJobs é o número máximo de jobs concorrentes
	MaxConcurrentJobs int

	// RecoveryEnabled habilita recuperação de panics
	RecoveryEnabled bool

	// OnJobStart é um callback executado quando um job inicia
	OnJobStart func(jobName string)

	// OnJobComplete é um callback executado quando um job completa
	OnJobComplete func(jobName string, duration time.Duration, err error)

	// OnJobPanic é um callback executado quando um job entra em panic
	OnJobPanic func(jobName string, recovered interface{})
}

// DefaultConfig retorna uma configuração padrão.
func DefaultConfig() *Config {
	return &Config{
		ServiceName:       "cron-worker",
		WithSeconds:       false,
		Location:          time.UTC,
		ShutdownTimeout:   30 * time.Second,
		JobTimeout:        5 * time.Minute,
		EnableHealthCheck: true,
		HealthChecks:      []HealthCheck{},
		EnableMetrics:     false,
		MetricsNamespace:  "cron_worker",
		EnableTracing:     false,
		MaxConcurrentJobs: 0, // 0 = sem limite
		RecoveryEnabled:   true,
		OnJobStart:        nil,
		OnJobComplete:     nil,
		OnJobPanic:        nil,
	}
}

// Validate valida as configurações.
func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return errors.New("service name is required")
	}

	if c.ShutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be positive")
	}

	if c.JobTimeout <= 0 {
		return errors.New("job timeout must be positive")
	}

	if c.Location == nil {
		return errors.New("location cannot be nil")
	}

	if c.MaxConcurrentJobs < 0 {
		return errors.New("max concurrent jobs cannot be negative")
	}

	if c.EnableMetrics && c.MetricsNamespace == "" {
		return errors.New("metrics namespace is required when metrics are enabled")
	}

	// Valida health checks
	for i, hc := range c.HealthChecks {
		if hc.Name == "" {
			return errors.New("health check name is required")
		}
		if hc.Check == nil {
			return errors.New("health check function is required")
		}
		// Verifica duplicatas
		for j := 0; j < i; j++ {
			if c.HealthChecks[j].Name == hc.Name {
				return errors.New("duplicate health check name: " + hc.Name)
			}
		}
	}

	return nil
}
