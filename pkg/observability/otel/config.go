package otel

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc/credentials"
)

type OTLPProtocol string

const (
	ProtocolGRPC OTLPProtocol = "grpc"
	ProtocolHTTP OTLPProtocol = "http"
)

type Config struct {
	ServiceName            string
	ServiceVersion         string
	Environment            string
	OTLPEndpoint           string
	OTLPProtocol           OTLPProtocol
	Insecure               bool
	TLSConfig              *tls.Config
	TraceSampleRate        float64
	LogLevel               observability.LogLevel
	LogFormat              observability.LogFormat
	Sanitize               bool
	ConsoleLog             bool
	MetricExportInterval   int64
	MetricNamespace        string
	EnableCardinalityCheck bool
	CustomBlockedLabels    []string
	ResourceAttributes     map[string]string
	PropagationHeaders     observability.PropagationHeaders
	RegisterGlobal         bool
}

func DefaultConfig(serviceName string) *Config {
	return &Config{
		ServiceName:            serviceName,
		ServiceVersion:         "unknown",
		Environment:            "development",
		OTLPEndpoint:           "localhost:4317",
		OTLPProtocol:           ProtocolGRPC,
		TraceSampleRate:        1.0,
		LogLevel:               observability.LogLevelInfo,
		LogFormat:              observability.LogFormatJSON,
		MetricExportInterval:   60,
		EnableCardinalityCheck: false,
		RegisterGlobal:         true,
	}
}

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

type Provider struct {
	config         *Config
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	tracer         *otelTracer
	logger         *otelLogger
	metrics        *otelMetrics
	runtime        *runtime
}

func validateConfig(config *Config) error {
	if strings.TrimSpace(config.ServiceName) == "" {
		return observability.NewInvalidConfigError("service name cannot be empty")
	}
	if config.OTLPEndpoint == "" {
		return observability.NewInvalidConfigError("OTLP endpoint cannot be empty")
	}
	if config.TraceSampleRate < 0.0 || config.TraceSampleRate > 1.0 {
		return observability.NewInvalidConfigError(fmt.Sprintf("trace sample rate must be between 0.0 and 1.0, got %f", config.TraceSampleRate))
	}
	return nil
}

func validateSecurityConfig(config *Config) error {
	if config.Insecure {
		env := strings.ToLower(config.Environment)
		if env == "production" || env == "prod" {
			return observability.NewInvalidConfigError("insecure connections are not allowed in production environment")
		}
	}
	if config.TLSConfig != nil {
		if config.TLSConfig.MinVersion > 0 && config.TLSConfig.MinVersion < tls.VersionTLS12 {
			return observability.NewInvalidConfigError("minimum TLS version must be 1.2 or higher for security compliance")
		}
	}
	return nil
}

func NewProvider(ctx context.Context, config *Config) (*Provider, error) {
	if config == nil {
		return nil, observability.NewInvalidConfigError("config cannot be nil")
	}
	config.ServiceName = strings.TrimSpace(config.ServiceName)
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if err := validateSecurityConfig(config); err != nil {
		return nil, err
	}
	config.OTLPProtocol = normalizeProtocol(string(config.OTLPProtocol))
	rt, err := newRuntime(ctx, config)
	if err != nil {
		return nil, err
	}
	return rt.observability(), nil
}

func (p *Provider) createResource(ctx context.Context) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(p.config.ServiceName),
			semconv.ServiceVersion(p.config.ServiceVersion),
			semconv.DeploymentEnvironment(p.config.Environment),
		),
	}

	if len(p.config.ResourceAttributes) > 0 {
		customAttrs := make([]attribute.KeyValue, 0, len(p.config.ResourceAttributes))
		for k, v := range p.config.ResourceAttributes {
			customAttrs = append(customAttrs, attribute.String(k, v))
		}
		attrs = append(attrs, resource.WithAttributes(customAttrs...))
	}

	return resource.New(ctx, attrs...)
}

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

	if p.config.RegisterGlobal {
		otel.SetTracerProvider(p.tracerProvider)
	}

	p.runtime.shutdown.register(shutdownStep{
		name:       "tracer_provider",
		forceFlush: p.tracerProvider.ForceFlush,
		shutdown:   p.tracerProvider.Shutdown,
	})

	return nil
}

func (p *Provider) createTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(p.config.OTLPEndpoint),
		}

		switch {
		case p.config.Insecure:
			opts = append(opts, otlptracehttp.WithInsecure())
		case p.config.TLSConfig != nil:
			opts = append(opts, otlptracehttp.WithTLSClientConfig(p.config.TLSConfig))
		}

		return otlptracehttp.New(ctx, opts...)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(p.config.OTLPEndpoint),
	}

	switch {
	case p.config.Insecure:
		opts = append(opts, otlptracegrpc.WithInsecure())
	case p.config.TLSConfig != nil:
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(p.config.TLSConfig)))
	}

	return otlptracegrpc.New(ctx, opts...)
}

func (p *Provider) createTraceSampler() sdktrace.Sampler {
	if p.config.TraceSampleRate >= 1.0 {
		return sdktrace.AlwaysSample()
	}

	if p.config.TraceSampleRate <= 0.0 {
		return sdktrace.NeverSample()
	}

	return sdktrace.TraceIDRatioBased(p.config.TraceSampleRate)
}

func (p *Provider) initMeterProvider(ctx context.Context, res *resource.Resource) error {
	exporter, err := p.createMetricExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	interval := time.Duration(p.config.MetricExportInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(interval),
	)

	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	if p.config.RegisterGlobal {
		otel.SetMeterProvider(p.meterProvider)
	}

	p.runtime.shutdown.register(shutdownStep{
		name:       "meter_provider",
		forceFlush: p.meterProvider.ForceFlush,
		shutdown:   p.meterProvider.Shutdown,
	})

	return nil
}

func (p *Provider) createMetricExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(p.config.OTLPEndpoint),
		}

		switch {
		case p.config.Insecure:
			opts = append(opts, otlpmetrichttp.WithInsecure())
		case p.config.TLSConfig != nil:
			opts = append(opts, otlpmetrichttp.WithTLSClientConfig(p.config.TLSConfig))
		}

		return otlpmetrichttp.New(ctx, opts...)
	}

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(p.config.OTLPEndpoint),
	}

	switch {
	case p.config.Insecure:
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	case p.config.TLSConfig != nil:
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(p.config.TLSConfig)))
	}

	return otlpmetricgrpc.New(ctx, opts...)
}

func (p *Provider) initLoggerProvider(ctx context.Context, res *resource.Resource) error {
	exporter, err := p.createLogExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create log exporter: %w", err)
	}

	p.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	p.runtime.shutdown.register(shutdownStep{
		name:       "logger_provider",
		forceFlush: p.loggerProvider.ForceFlush,
		shutdown:   p.loggerProvider.Shutdown,
	})

	return nil
}

func (p *Provider) createLogExporter(ctx context.Context) (sdklog.Exporter, error) {
	if p.config.OTLPProtocol == ProtocolHTTP {
		opts := []otlploghttp.Option{
			otlploghttp.WithEndpoint(p.config.OTLPEndpoint),
		}

		switch {
		case p.config.Insecure:
			opts = append(opts, otlploghttp.WithInsecure())
		case p.config.TLSConfig != nil:
			opts = append(opts, otlploghttp.WithTLSClientConfig(p.config.TLSConfig))
		}

		return otlploghttp.New(ctx, opts...)
	}

	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(p.config.OTLPEndpoint),
	}

	switch {
	case p.config.Insecure:
		opts = append(opts, otlploggrpc.WithInsecure())
	case p.config.TLSConfig != nil:
		opts = append(opts, otlploggrpc.WithTLSCredentials(credentials.NewTLS(p.config.TLSConfig)))
	}

	return otlploggrpc.New(ctx, opts...)
}

func (p *Provider) Tracer() observability.Tracer   { return p.tracer }
func (p *Provider) Logger() observability.Logger   { return p.logger }
func (p *Provider) Metrics() observability.Metrics { return p.metrics }

func (p *Provider) HTTP() HTTPInstrumentation {
	if p == nil || p.runtime == nil || p.runtime.http == nil {
		return noopHTTPInstrumentation{}
	}
	return p.runtime.http
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if p.runtime == nil {
		return nil
	}
	return p.runtime.Shutdown(ctx)
}
