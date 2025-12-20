package o11y

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type Telemetry interface {
	Tracer() Tracer
	Metrics() Metrics
	Logger() Logger
	Shutdown(ctx context.Context) error
	IsClosed() bool
}

// ServiceConfig holds service configuration for telemetry.
// This type abstracts the underlying OpenTelemetry resource configuration.
type ServiceConfig struct {
	// Name is the service name (required)
	Name string
	// Version is the service version (required)
	Version string
	// Environment is the deployment environment (e.g., "production", "staging", "development")
	Environment string
	// CustomAttributes are additional custom attributes to add to the resource
	CustomAttributes map[string]string
	// ResourceOptions allows fine-tuning what resource information is collected
	ResourceOptions ResourceConfig
}

// SamplingStrategy defines how traces should be sampled
type SamplingStrategy string

const (
	// SamplingAlways samples all traces (default, not recommended for high-volume production)
	SamplingAlways SamplingStrategy = "always"
	// SamplingNever samples no traces
	SamplingNever SamplingStrategy = "never"
	// SamplingTraceIDRatio samples traces based on trace ID ratio
	SamplingTraceIDRatio SamplingStrategy = "trace_id_ratio"
	// SamplingParentBased respects parent span sampling decision, with fallback to trace ID ratio
	SamplingParentBased SamplingStrategy = "parent_based"
)

// SamplingConfig configures trace sampling behavior
type SamplingConfig struct {
	// Strategy determines the sampling strategy to use
	Strategy SamplingStrategy
	// Ratio is the sampling ratio (0.0 to 1.0) for ratio-based sampling
	// Only used when Strategy is SamplingTraceIDRatio or SamplingParentBased
	// Example: 0.1 = sample 10% of traces
	Ratio float64
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

// NewServiceResourceFromConfig creates a resource from a ServiceConfig.
// This is the recommended way to create resources, as it doesn't expose OpenTelemetry types.
func NewServiceResourceFromConfig(ctx context.Context, cfg ServiceConfig) (*resource.Resource, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	if cfg.Version == "" {
		return nil, fmt.Errorf("service version cannot be empty")
	}

	resourceOpts := []resource.Option{
		resource.WithAttributes(
			semconv.TelemetrySDKLanguageGo,
			semconv.ServiceNameKey.String(cfg.Name),
			semconv.ServiceVersionKey.String(cfg.Version),
			semconv.TelemetrySDKName("opentelemetry-go"),
			semconv.DeploymentEnvironmentName(cfg.Environment),
		),
		resource.WithFromEnv(),
	}

	// Add custom attributes if provided
	if len(cfg.CustomAttributes) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(cfg.CustomAttributes))
		for k, v := range cfg.CustomAttributes {
			attrs = append(attrs, attribute.String(k, v))
		}
		resourceOpts = append(resourceOpts, resource.WithAttributes(attrs...))
	}

	// Apply resource collection options
	if cfg.ResourceOptions.IncludeProcess {
		resourceOpts = append(resourceOpts, resource.WithProcess())
	}
	if cfg.ResourceOptions.IncludeOS {
		resourceOpts = append(resourceOpts, resource.WithOS())
	}
	if cfg.ResourceOptions.IncludeContainer {
		resourceOpts = append(resourceOpts, resource.WithContainer())
	}
	if cfg.ResourceOptions.IncludeHost {
		resourceOpts = append(resourceOpts, resource.WithHost())
	}

	res, err := resource.New(ctx, resourceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return res, nil
}

// convertSamplingConfig converts SamplingConfig to OpenTelemetry Sampler.
// This is an internal function to hide OpenTelemetry types from the public API.
func convertSamplingConfig(cfg SamplingConfig) sdktrace.Sampler {
	switch cfg.Strategy {
	case SamplingAlways:
		return sdktrace.AlwaysSample()
	case SamplingNever:
		return sdktrace.NeverSample()
	case SamplingTraceIDRatio:
		ratio := cfg.Ratio
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return sdktrace.TraceIDRatioBased(ratio)
	case SamplingParentBased:
		ratio := cfg.Ratio
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		// Default to always sample if strategy is unknown
		return sdktrace.AlwaysSample()
	}
}
