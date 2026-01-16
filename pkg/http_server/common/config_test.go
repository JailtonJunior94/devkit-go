package common

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Address != ":8080" {
		t.Errorf("expected address :8080, got %s", cfg.Address)
	}

	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", cfg.ReadTimeout)
	}

	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("expected WriteTimeout 30s, got %v", cfg.WriteTimeout)
	}

	if cfg.IdleTimeout != 120*time.Second {
		t.Errorf("expected IdleTimeout 120s, got %v", cfg.IdleTimeout)
	}

	if cfg.BodyLimit != 4*1024*1024 {
		t.Errorf("expected BodyLimit 4MB, got %d", cfg.BodyLimit)
	}

	if cfg.ServiceName != "unknown-service" {
		t.Errorf("expected ServiceName unknown-service, got %s", cfg.ServiceName)
	}

	if !cfg.EnableHealthChecks {
		t.Error("expected EnableHealthChecks to be true")
	}
}

func TestConfigValidate_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test-service"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "production"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfigValidate_EmptyServiceName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty service name")
	}

	if !strings.Contains(err.Error(), "service name is required") {
		t.Errorf("expected 'service name is required' error, got %v", err)
	}
}

func TestConfigValidate_TrimmedEmptyServiceName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "   " // Only spaces

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for trimmed empty service name")
	}
}

func TestConfigValidate_EmptyServiceVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty service version")
	}

	if !strings.Contains(err.Error(), "service version is required") {
		t.Errorf("expected 'service version is required' error, got %v", err)
	}
}

func TestConfigValidate_EmptyEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty environment")
	}

	if !strings.Contains(err.Error(), "environment is required") {
		t.Errorf("expected 'environment is required' error, got %v", err)
	}
}

func TestConfigValidate_InvalidReadTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.ReadTimeout = 0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid read timeout")
	}

	if !strings.Contains(err.Error(), "read timeout must be positive") {
		t.Errorf("expected 'read timeout must be positive' error, got %v", err)
	}
}

func TestConfigValidate_InvalidWriteTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.WriteTimeout = -1 * time.Second

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid write timeout")
	}
}

func TestConfigValidate_InvalidIdleTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.IdleTimeout = 0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid idle timeout")
	}
}

func TestConfigValidate_InvalidBodyLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.BodyLimit = -100

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid body limit")
	}

	if !strings.Contains(err.Error(), "body limit must be positive") {
		t.Errorf("expected 'body limit must be positive' error, got %v", err)
	}
}

func TestConfigValidate_CORSEnabledWithoutOrigins(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.EnableCORS = true
	cfg.CORSOrigins = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for CORS enabled without origins")
	}

	if !strings.Contains(err.Error(), "CORS origins are required when CORS is enabled") {
		t.Errorf("expected 'CORS origins are required' error, got %v", err)
	}
}

func TestConfigValidate_CORSEnabledWithOrigins(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "test"
	cfg.ServiceVersion = "1.0.0"
	cfg.Environment = "dev"
	cfg.EnableCORS = true
	cfg.CORSOrigins = "https://example.com"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
