package o11y

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Telemetry provides unified access to tracing, metrics, and logging.
type Telemetry interface {
	Tracer() Tracer
	Metrics() Metrics
	Logger() Logger
	Shutdown(ctx context.Context) error
	ShutdownWithTimeout(timeout time.Duration) error
}

// NewServiceResource creates a new OpenTelemetry resource with service metadata.
func NewServiceResource(ctx context.Context, name, version, environment string) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.TelemetrySDKLanguageGo,
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String(version),
			semconv.TelemetrySDKName("opentelemetry-go"),
			semconv.DeploymentEnvironmentName(environment),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return res, nil
}

// SetupTelemetry is a convenience function that sets up all telemetry components.
// It creates tracer, metrics, and logger with the given configuration.
// For development environments, use the WithXXXInsecure() options.
func SetupTelemetry(
	ctx context.Context,
	cfg Config,
	tracerOpts []TracerOption,
	metricsOpts []MetricsOption,
	loggerOpts []LoggerOption,
) (Telemetry, error) {
	res, err := NewServiceResource(ctx, cfg.ServiceName, cfg.ServiceVersion, cfg.Environment)
	if err != nil {
		return nil, fmt.Errorf("failed to create service resource: %w", err)
	}

	tracer, shutdownTracer, err := NewTracer(ctx, cfg.TracerEndpoint, cfg.ServiceName, res, tracerOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer: %w", err)
	}

	metrics, shutdownMetrics, err := NewMetrics(ctx, cfg.MetricsEndpoint, cfg.ServiceName, res, metricsOpts...)
	if err != nil {
		if shutdownErr := shutdownTracer(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown tracer during cleanup: %v", shutdownErr)
		}
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	logger, shutdownLogger, err := NewLogger(ctx, tracer, cfg.LoggerEndpoint, cfg.ServiceName, res, loggerOpts...)
	if err != nil {
		if shutdownErr := shutdownTracer(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown tracer during cleanup: %v", shutdownErr)
		}
		if shutdownErr := shutdownMetrics(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown metrics during cleanup: %v", shutdownErr)
		}
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	tel, err := NewTelemetry(tracer, metrics, logger, shutdownTracer, shutdownMetrics, shutdownLogger)
	if err != nil {
		if shutdownErr := shutdownTracer(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown tracer during cleanup: %v", shutdownErr)
		}
		if shutdownErr := shutdownMetrics(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown metrics during cleanup: %v", shutdownErr)
		}
		if shutdownErr := shutdownLogger(ctx); shutdownErr != nil {
			log.Printf("telemetry: failed to shutdown logger during cleanup: %v", shutdownErr)
		}
		return nil, fmt.Errorf("failed to create telemetry: %w", err)
	}

	return tel, nil
}

// SetupTelemetryInsecure is a convenience function for development environments.
// It sets up all telemetry components with TLS disabled.
func SetupTelemetryInsecure(ctx context.Context, cfg Config) (Telemetry, error) {
	return SetupTelemetry(
		ctx,
		cfg,
		[]TracerOption{WithTracerInsecure()},
		[]MetricsOption{WithMetricsInsecure()},
		[]LoggerOption{WithLoggerInsecure()},
	)
}

// Attr creates an Attribute from a key-value pair.
// This is a convenience function for creating attributes.
func Attr(key string, value any) Attribute {
	return Attribute{Key: key, Value: value}
}

// LogField creates a Field from a key-value pair.
// This is a convenience function for creating log fields.
func LogField(key string, value any) Field {
	return Field{Key: key, Value: value}
}
