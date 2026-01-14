package serverfiber

import (
	"errors"
	"time"
)

type Config struct {
	Address            string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	BodyLimit          int
	ServiceName        string
	ServiceVersion     string
	Environment        string
	CORSOrigins        string
	EnableCORS         bool
	EnableMetrics      bool
	EnableHealthChecks bool
	EnableTracing      bool // Enable OpenTelemetry distributed tracing
	EnableOTelMetrics  bool // Enable OpenTelemetry HTTP metrics
}

func DefaultConfig() Config {
	return Config{
		Address:            ":8080",
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		BodyLimit:          4 * 1024 * 1024, // 4MB
		ServiceName:        "unknown-service",
		ServiceVersion:     "unknown",
		Environment:        "development",
		CORSOrigins:        "",
		EnableCORS:         false,
		EnableMetrics:      false,
		EnableHealthChecks: true, // Health checks enabled by default
		EnableTracing:      false,
		EnableOTelMetrics:  false,
	}
}

func (c Config) Validate() error {
	if c.ServiceName == "" {
		return errors.New("service name is required")
	}

	if c.ServiceVersion == "" {
		return errors.New("service version is required")
	}

	if c.Environment == "" {
		return errors.New("environment is required")
	}

	if c.ReadTimeout <= 0 {
		return errors.New("read timeout must be positive")
	}

	if c.WriteTimeout <= 0 {
		return errors.New("write timeout must be positive")
	}

	if c.IdleTimeout <= 0 {
		return errors.New("idle timeout must be positive")
	}

	if c.BodyLimit <= 0 {
		return errors.New("body limit must be positive")
	}

	if c.EnableCORS && c.CORSOrigins == "" {
		return errors.New("CORS origins must be specified when CORS is enabled")
	}

	return nil
}
