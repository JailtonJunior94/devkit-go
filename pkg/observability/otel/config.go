package otel

import (
	"context"
	"fmt"
	"strings"

	"github.com/jailtonjunior94/order/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// OTLPProtocol defines the protocol to use for OTLP export.
type OTLPProtocol string

const (
	// ProtocolGRPC uses gRPC protocol for OTLP export (default: port 4317).
	ProtocolGRPC OTLPProtocol = "grpc"
	// ProtocolHTTP uses HTTP/protobuf protocol for OTLP export (default: port 4318).
	ProtocolHTTP OTLPProtocol = "http"
)

// Config holds the configuration for the OpenTelemetry provider.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	OTLPProtocol   OTLPProtocol // "grpc" or "http", defaults to "grpc"

	// Trace configuration
	TraceSampleRate float64 // 0.0 to 1.0, default 1.0 (always sample)

	// Log configuration
	LogLevel  observability.LogLevel
	LogFormat observability.LogFormat

	// Resource attributes (optional)
	ResourceAttributes map[string]string
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig(serviceName string) *Config {
	return &Config{
		ServiceName:     serviceName,
		ServiceVersion:  "unknown",
		Environment:     "development",
		OTLPEndpoint:    "localhost:4317",
		OTLPProtocol:    ProtocolGRPC,
		TraceSampleRate: 1.0,
		LogLevel:        observability.LogLevelInfo,
		LogFormat:       observability.LogFormatJSON,
	}
}

// normalizeProtocol normalizes the protocol string to a valid OTLPProtocol.
func normalizeProtocol(protocol string) OTLPProtocol {
	switch strings.ToLower(protocol) {
	case "http", "http/protobuf":
		return ProtocolHTTP
	case "grpc", "":
		return ProtocolGRPC
	default:
		return ProtocolGRPC
	}
}

// Provider implements the observability.Observability interface using OpenTelemetry.
type Provider struct {
	config          *Config
	tracerProvider  *sdktrace.TracerProvider
	meterProvider   *sdkmetric.MeterProvider
	loggerProvider  *sdklog.LoggerProvider
	tracer          *otelTracer
	logger          *otelLogger
	metrics         *otelMetrics
	shutdownFuncs   []func(context.Context) error
}

// NewProvider creates and initializes a new OpenTelemetry provider.
func NewProvider(ctx context.Context, config *Config) (*Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Normalize protocol
	config.OTLPProtocol = normalizeProtocol(string(config.OTLPProtocol))

	provider := &Provider{
		config:        config,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	// Create resource with service information
	res, err := provider.createResource()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracer provider
	if err := provider.initTracerProvider(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to initialize tracer provider: %w", err)
	}

	// Initialize meter provider
	if err := provider.initMeterProvider(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to initialize meter provider: %w", err)
	}

	// Initialize logger provider
	if err := provider.initLoggerProvider(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to initialize logger provider: %w", err)
	}

	// Set global propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize components
	provider.tracer = newOtelTracer(provider.tracerProvider.Tracer(config.ServiceName))
	provider.logger = newOtelLogger(
		config.LogLevel,
		config.LogFormat,
		config.ServiceName,
		provider.loggerProvider.Logger(config.ServiceName),
	)
	provider.metrics = newOtelMetrics(provider.meterProvider.Meter(config.ServiceName))

	return provider, nil
}

// createResource creates an OTLP resource with service information.
func (p *Provider) createResource() (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(p.config.ServiceName),
			semconv.ServiceVersion(p.config.ServiceVersion),
			semconv.DeploymentEnvironment(p.config.Environment),
		),
	}

	// Add custom resource attributes
	if len(p.config.ResourceAttributes) > 0 {
		customAttrs := make([]attribute.KeyValue, 0, len(p.config.ResourceAttributes))
		for k, v := range p.config.ResourceAttributes {
			customAttrs = append(customAttrs, attribute.String(k, v))
		}
		attrs = append(attrs, resource.WithAttributes(customAttrs...))
	}

	return resource.New(
		context.Background(),
		attrs...,
	)
}

// initTracerProvider initializes the OpenTelemetry tracer provider.
func (p *Provider) initTracerProvider(ctx context.Context, res *resource.Resource) error {
	exporter, err := p.createTraceExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	sampler := p.createTraceSampler()

	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(p.tracerProvider)
	p.shutdownFuncs = append(p.shutdownFuncs, p.tracerProvider.Shutdown)

	return nil
}

// createTraceExporter creates the appropriate trace exporter based on protocol.
func (p *Provider) createTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		return otlptracehttp.New(
			ctx,
			otlptracehttp.WithEndpoint(p.config.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		)
	}

	return otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(p.config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
}

// createTraceSampler creates the appropriate sampler based on sample rate.
func (p *Provider) createTraceSampler() sdktrace.Sampler {
	if p.config.TraceSampleRate >= 1.0 {
		return sdktrace.AlwaysSample()
	}

	if p.config.TraceSampleRate <= 0.0 {
		return sdktrace.NeverSample()
	}

	return sdktrace.TraceIDRatioBased(p.config.TraceSampleRate)
}

// initMeterProvider initializes the OpenTelemetry meter provider.
func (p *Provider) initMeterProvider(ctx context.Context, res *resource.Resource) error {
	exporter, err := p.createMetricExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	otel.SetMeterProvider(p.meterProvider)
	p.shutdownFuncs = append(p.shutdownFuncs, p.meterProvider.Shutdown)

	return nil
}

// createMetricExporter creates the appropriate metric exporter based on protocol.
func (p *Provider) createMetricExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		return otlpmetrichttp.New(
			ctx,
			otlpmetrichttp.WithEndpoint(p.config.OTLPEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
	}

	return otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(p.config.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
}

// initLoggerProvider initializes the OpenTelemetry logger provider.
func (p *Provider) initLoggerProvider(ctx context.Context, res *resource.Resource) error {
	exporter, err := p.createLogExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create log exporter: %w", err)
	}

	p.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	p.shutdownFuncs = append(p.shutdownFuncs, p.loggerProvider.Shutdown)

	return nil
}

// createLogExporter creates the appropriate log exporter based on protocol.
func (p *Provider) createLogExporter(ctx context.Context) (sdklog.Exporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		return otlploghttp.New(
			ctx,
			otlploghttp.WithEndpoint(p.config.OTLPEndpoint),
			otlploghttp.WithInsecure(),
		)
	}

	return otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(p.config.OTLPEndpoint),
		otlploggrpc.WithInsecure(),
	)
}

// Tracer returns the OpenTelemetry tracer.
func (p *Provider) Tracer() observability.Tracer {
	return p.tracer
}

// Logger returns the OpenTelemetry logger.
func (p *Provider) Logger() observability.Logger {
	return p.logger
}

// Metrics returns the OpenTelemetry metrics recorder.
func (p *Provider) Metrics() observability.Metrics {
	return p.metrics
}

// Shutdown gracefully shuts down the provider, flushing any pending telemetry.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	for _, shutdown := range p.shutdownFuncs {
		if err := shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}
