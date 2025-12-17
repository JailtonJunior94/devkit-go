package o11y

import (
	"crypto/tls"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestDefaultTracerConfig(t *testing.T) {
	cfg := defaultTracerConfig("localhost:4317")

	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected endpoint localhost:4317, got %v", cfg.Endpoint)
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false by default")
	}
	if cfg.BatchSize != 512 {
		t.Errorf("expected BatchSize 512, got %d", cfg.BatchSize)
	}
	if cfg.BatchDelay != 5*time.Second {
		t.Errorf("expected BatchDelay 5s, got %v", cfg.BatchDelay)
	}
}

func TestTracerOptions(t *testing.T) {
	cfg := defaultTracerConfig("localhost:4317")

	WithTracerInsecure()(cfg)
	if !cfg.Insecure {
		t.Error("expected Insecure to be true")
	}

	tlsConfig := &tls.Config{}
	WithTracerTLS(tlsConfig)(cfg)
	if cfg.TLSConfig != tlsConfig {
		t.Error("expected TLSConfig to be set")
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false after setting TLS")
	}

	sampler := sdktrace.NeverSample()
	WithTracerSampler(sampler)(cfg)
	if cfg.Sampler == nil {
		t.Error("expected Sampler to be set")
	}

	WithTracerBatchSize(1024)(cfg)
	if cfg.BatchSize != 1024 {
		t.Errorf("expected BatchSize 1024, got %d", cfg.BatchSize)
	}

	WithTracerBatchDelay(10 * time.Second)(cfg)
	if cfg.BatchDelay != 10*time.Second {
		t.Errorf("expected BatchDelay 10s, got %v", cfg.BatchDelay)
	}
}

func TestDefaultMetricsConfig(t *testing.T) {
	cfg := defaultMetricsConfig("localhost:4317")

	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected endpoint localhost:4317, got %v", cfg.Endpoint)
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false by default")
	}
	if cfg.ExportInterval != 15*time.Second {
		t.Errorf("expected ExportInterval 15s, got %v", cfg.ExportInterval)
	}
}

func TestMetricsOptions(t *testing.T) {
	cfg := defaultMetricsConfig("localhost:4317")

	WithMetricsInsecure()(cfg)
	if !cfg.Insecure {
		t.Error("expected Insecure to be true")
	}

	tlsConfig := &tls.Config{}
	WithMetricsTLS(tlsConfig)(cfg)
	if cfg.TLSConfig != tlsConfig {
		t.Error("expected TLSConfig to be set")
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false after setting TLS")
	}

	WithMetricsExportInterval(30 * time.Second)(cfg)
	if cfg.ExportInterval != 30*time.Second {
		t.Errorf("expected ExportInterval 30s, got %v", cfg.ExportInterval)
	}
}

func TestDefaultLoggerConfig(t *testing.T) {
	cfg := defaultLoggerConfig("http://localhost:4318")

	if cfg.Endpoint != "http://localhost:4318" {
		t.Errorf("expected endpoint http://localhost:4318, got %v", cfg.Endpoint)
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false by default")
	}
}

func TestLoggerOptions(t *testing.T) {
	cfg := defaultLoggerConfig("http://localhost:4318")

	WithLoggerInsecure()(cfg)
	if !cfg.Insecure {
		t.Error("expected Insecure to be true")
	}

	tlsConfig := &tls.Config{}
	WithLoggerTLS(tlsConfig)(cfg)
	if cfg.TLSConfig != tlsConfig {
		t.Error("expected TLSConfig to be set")
	}
	if cfg.Insecure {
		t.Error("expected Insecure to be false after setting TLS")
	}
}
