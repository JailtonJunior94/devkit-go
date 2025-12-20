package o11y

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type Telemetry interface {
	Tracer() Tracer
	Metrics() Metrics
	Logger() Logger
	Shutdown(ctx context.Context) error
	IsClosed() bool
}

// ResourceConfig holds configuration for resource collection.
// By default, sensitive information like process command line is not collected.
type ResourceConfig struct {
	// IncludeProcess includes process information (may expose secrets in command line args).
	// WARNING: Command line arguments may contain passwords or API keys.
	IncludeProcess bool
	// IncludeOS includes operating system information (type, version).
	IncludeOS bool
	// IncludeContainer includes container information (ID, runtime).
	IncludeContainer bool
	// IncludeHost includes host information (name, ID).
	// WARNING: May expose internal infrastructure details.
	IncludeHost bool
}

// DefaultResourceConfig returns a secure default configuration.
// Process and Host are excluded by default as they may expose sensitive data.
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		IncludeProcess:   false, // May contain secrets in args
		IncludeOS:        true,
		IncludeContainer: true,
		IncludeHost:      false, // May expose infrastructure
	}
}

// NewServiceResource creates a new resource with default (secure) configuration.
// For custom resource collection, use NewServiceResourceWithConfig.
func NewServiceResource(ctx context.Context, name, version, environment string) (*resource.Resource, error) {
	return NewServiceResourceWithConfig(ctx, name, version, environment, DefaultResourceConfig())
}

// NewServiceResourceWithConfig creates a new resource with configurable collection options.
// Use DefaultResourceConfig() as a starting point and modify as needed.
func NewServiceResourceWithConfig(ctx context.Context, name, version, environment string, cfg ResourceConfig) (*resource.Resource, error) {
	resourceOpts := []resource.Option{
		resource.WithAttributes(
			semconv.TelemetrySDKLanguageGo,
			semconv.ServiceNameKey.String(name),
			semconv.ServiceVersionKey.String(version),
			semconv.TelemetrySDKName("opentelemetry-go"),
			semconv.DeploymentEnvironmentName(environment),
		),
		resource.WithFromEnv(),
	}

	if cfg.IncludeProcess {
		resourceOpts = append(resourceOpts, resource.WithProcess())
	}
	if cfg.IncludeOS {
		resourceOpts = append(resourceOpts, resource.WithOS())
	}
	if cfg.IncludeContainer {
		resourceOpts = append(resourceOpts, resource.WithContainer())
	}
	if cfg.IncludeHost {
		resourceOpts = append(resourceOpts, resource.WithHost())
	}

	res, err := resource.New(ctx, resourceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return res, nil
}
