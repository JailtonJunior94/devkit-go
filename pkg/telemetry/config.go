package o11y

import (
	"crypto/tls"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config holds the configuration for telemetry setup.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	// TracerEndpoint is the gRPC endpoint for traces (format: "host:port").
	// Example: "otel-collector:4317"
	TracerEndpoint string

	// MetricsEndpoint is the gRPC endpoint for metrics (format: "host:port").
	// Example: "otel-collector:4317"
	MetricsEndpoint string

	// LoggerEndpoint is the HTTP endpoint for logs (format: "http(s)://host:port/path").
	// Example: "http://otel-collector:4318/v1/logs"
	LoggerEndpoint string
}

// TracerConfig holds tracer-specific configuration.
type TracerConfig struct {
	Endpoint   string
	Insecure   bool
	TLSConfig  *tls.Config
	Sampler    sdktrace.Sampler
	BatchSize  int
	BatchDelay time.Duration
}

// TracerOption is a function that configures TracerConfig.
type TracerOption func(*TracerConfig)

// WithTracerInsecure disables TLS for tracer connection.
// WARNING: Only use in development environments.
func WithTracerInsecure() TracerOption {
	return func(c *TracerConfig) {
		c.Insecure = true
	}
}

// WithTracerTLS sets custom TLS configuration.
func WithTracerTLS(tlsConfig *tls.Config) TracerOption {
	return func(c *TracerConfig) {
		c.TLSConfig = tlsConfig
		c.Insecure = false
	}
}

// WithTracerSampler sets a custom sampler for traces.
func WithTracerSampler(sampler sdktrace.Sampler) TracerOption {
	return func(c *TracerConfig) {
		c.Sampler = sampler
	}
}

// WithTracerBatchSize sets the batch size for trace export.
func WithTracerBatchSize(size int) TracerOption {
	return func(c *TracerConfig) {
		c.BatchSize = size
	}
}

// WithTracerBatchDelay sets the delay between batch exports.
func WithTracerBatchDelay(delay time.Duration) TracerOption {
	return func(c *TracerConfig) {
		c.BatchDelay = delay
	}
}

// MetricsConfig holds metrics-specific configuration.
type MetricsConfig struct {
	Endpoint       string
	Insecure       bool
	TLSConfig      *tls.Config
	ExportInterval time.Duration
	ErrorHandler   func(err error)
}

// MetricsOption is a function that configures MetricsConfig.
type MetricsOption func(*MetricsConfig)

// WithMetricsInsecure disables TLS for metrics connection.
// WARNING: Only use in development environments.
func WithMetricsInsecure() MetricsOption {
	return func(c *MetricsConfig) {
		c.Insecure = true
	}
}

// WithMetricsTLS sets custom TLS configuration.
func WithMetricsTLS(tlsConfig *tls.Config) MetricsOption {
	return func(c *MetricsConfig) {
		c.TLSConfig = tlsConfig
		c.Insecure = false
	}
}

// WithMetricsExportInterval sets the interval for metrics export.
func WithMetricsExportInterval(interval time.Duration) MetricsOption {
	return func(c *MetricsConfig) {
		c.ExportInterval = interval
	}
}

// WithMetricsErrorHandler sets a custom error handler for metrics errors.
// By default, errors are logged to stderr.
func WithMetricsErrorHandler(handler func(err error)) MetricsOption {
	return func(c *MetricsConfig) {
		c.ErrorHandler = handler
	}
}

// LoggerConfig holds logger-specific configuration.
type LoggerConfig struct {
	Endpoint  string
	Insecure  bool
	TLSConfig *tls.Config
}

// LoggerOption is a function that configures LoggerConfig.
type LoggerOption func(*LoggerConfig)

// WithLoggerInsecure disables TLS for logger connection.
// WARNING: Only use in development environments.
func WithLoggerInsecure() LoggerOption {
	return func(c *LoggerConfig) {
		c.Insecure = true
	}
}

// WithLoggerTLS sets custom TLS configuration.
func WithLoggerTLS(tlsConfig *tls.Config) LoggerOption {
	return func(c *LoggerConfig) {
		c.TLSConfig = tlsConfig
		c.Insecure = false
	}
}

// defaultTracerConfig returns the default tracer configuration.
func defaultTracerConfig(endpoint string) *TracerConfig {
	return &TracerConfig{
		Endpoint:   endpoint,
		Insecure:   false,
		Sampler:    sdktrace.AlwaysSample(),
		BatchSize:  512,
		BatchDelay: 5 * time.Second,
	}
}

// defaultMetricsConfig returns the default metrics configuration.
func defaultMetricsConfig(endpoint string) *MetricsConfig {
	return &MetricsConfig{
		Endpoint:       endpoint,
		Insecure:       false,
		ExportInterval: 15 * time.Second,
	}
}

// defaultLoggerConfig returns the default logger configuration.
func defaultLoggerConfig(endpoint string) *LoggerConfig {
	return &LoggerConfig{
		Endpoint: endpoint,
		Insecure: false,
	}
}
