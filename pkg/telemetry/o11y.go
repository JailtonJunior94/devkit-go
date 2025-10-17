package o11y

import (
	"fmt"

	"context"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type Telemetry interface {
	Tracer() Tracer
	Metrics() Metrics
	Logger() Logger
}

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
