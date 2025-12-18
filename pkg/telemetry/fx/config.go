package telemetryfx

import (
	"os"

	o11y "github.com/JailtonJunior94/devkit-go/pkg/telemetry"
	"go.uber.org/fx"
)

// ConfigModule provides telemetry config from environment variables.
// Environment variables:
//   - SERVICE_NAME: Service name (default: "unknown")
//   - SERVICE_VERSION: Service version (default: "1.0.0")
//   - ENVIRONMENT: Environment name (default: "development")
//   - OTEL_EXPORTER_OTLP_ENDPOINT: gRPC endpoint for traces and metrics (default: "localhost:4317")
//   - OTEL_EXPORTER_OTLP_LOGS_ENDPOINT: HTTP endpoint for logs (default: "http://localhost:4318/v1/logs")
var ConfigModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates telemetry config from environment variables.
func ConfigFromEnv() o11y.Config {
	return o11y.Config{
		ServiceName:     getEnv("SERVICE_NAME", "unknown"),
		ServiceVersion:  getEnv("SERVICE_VERSION", "1.0.0"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		TracerEndpoint:  getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		MetricsEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		LoggerEndpoint:  getEnv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "http://localhost:4318/v1/logs"),
	}
}

// DefaultConfig returns a default telemetry config for local development.
func DefaultConfig() o11y.Config {
	return o11y.Config{
		ServiceName:     "unknown",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TracerEndpoint:  "localhost:4317",
		MetricsEndpoint: "localhost:4317",
		LoggerEndpoint:  "http://localhost:4318/v1/logs",
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
