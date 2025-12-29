package chiserver

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config holds the HTTP server configuration.
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
}

// DefaultConfig returns a new Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Address:            ":8080",
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		BodyLimit:          4 * 1024 * 1024,
		ServiceName:        "unknown-service",
		ServiceVersion:     "unknown",
		Environment:        "development",
		CORSOrigins:        "",
		EnableCORS:         false,
		EnableMetrics:      false,
		EnableHealthChecks: true,
	}
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return errors.New("service name is required")
	}

	if strings.TrimSpace(c.ServiceVersion) == "" {
		return errors.New("service version is required")
	}

	if strings.TrimSpace(c.Environment) == "" {
		return errors.New("environment is required")
	}

	if c.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive, got %v", c.ReadTimeout)
	}

	if c.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive, got %v", c.WriteTimeout)
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive, got %v", c.IdleTimeout)
	}

	if c.BodyLimit <= 0 {
		return fmt.Errorf("body limit must be positive, got %d", c.BodyLimit)
	}

	if c.EnableCORS && strings.TrimSpace(c.CORSOrigins) == "" {
		return errors.New("CORS origins are required when CORS is enabled")
	}

	return nil
}
