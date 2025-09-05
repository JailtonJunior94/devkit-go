package o11y

import (
	"context"
	"log"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Code uint32

const (
	Unset Code = 0
	Error Code = 1
	Ok    Code = 2
)

type (
	Observability interface {
		Tracer() trace.Tracer
		LoggerProvider() *slog.Logger
		MeterProvider() *metric.MeterProvider
		TracerProvider() *sdktrace.TracerProvider
		Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, Span)
	}

	Span interface {
		trace.Span
		AddStatus(ctx context.Context, code Code, description string)
		AddAttributes(ctx context.Context, code Code, description string, attrs ...Attributes)
	}

	span struct {
		trace.Span
		logger *slog.Logger
	}

	Attributes struct {
		Key   string
		Value any
	}

	Option        func(observability *observability)
	observability struct {
		serviceName    string
		serviceVersion string
		Span           Span
		tracer         trace.Tracer
		resource       *resource.Resource
		meterProvider  *metric.MeterProvider
		tracerProvider *sdktrace.TracerProvider
		logger         *slog.Logger
	}
)

func NewObservability(options ...Option) Observability {
	observability := &observability{}
	for _, option := range options {
		option(observability)
	}
	return observability
}

func NewDevelopmentObservability(serviceName, serviceVersion string) Observability {
	return NewObservability(
		WithServiceName(serviceName),
		WithServiceVersion(serviceVersion),
		WithResource(),
		WithMeterProviderStdout(),
		WithTracerProviderStdout(),
	)
}

func (o *observability) Tracer() trace.Tracer {
	return o.tracer
}

func (o *observability) MeterProvider() *metric.MeterProvider {
	return o.meterProvider
}

func (o *observability) TracerProvider() *sdktrace.TracerProvider {
	return o.tracerProvider
}

func (o *observability) LoggerProvider() *slog.Logger {
	return o.logger
}

func (o *observability) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, Span) {
	if len(opts) == 0 {
		ctx, startSpan := o.tracer.Start(ctx, name)
		return ctx, &span{Span: startSpan, logger: o.logger}
	}
	ctx, startSpan := o.tracer.Start(ctx, name, opts...)
	return ctx, &span{Span: startSpan, logger: o.logger}
}

func (s *span) AddStatus(ctx context.Context, code Code, description string) {
	s.Span.SetStatus(codes.Code(code), description)
}

func (s *span) AddAttributes(ctx context.Context, code Code, description string, attrs ...Attributes) {
	s.Span.SetStatus(codes.Code(code), description)
	s.addLogger(ctx, code, description, attrs...)
	for _, attr := range attrs {
		switch attr.Value.(type) {
		case string:
			s.Span.SetAttributes(attribute.Key(attr.Key).String(attr.Value.(string)))
		case int:
			s.Span.SetAttributes(attribute.Key(attr.Key).Int64(int64(attr.Value.(int))))
		case int64:
			s.Span.SetAttributes(attribute.Key(attr.Key).Int64(attr.Value.(int64)))
		case float64:
			s.Span.SetAttributes(attribute.Key(attr.Key).Float64(attr.Value.(float64)))
		case bool:
			s.Span.SetAttributes(attribute.Key(attr.Key).Bool(attr.Value.(bool)))
		case error:
			s.Span.SetAttributes(attribute.Key(attr.Key).String(attr.Value.(error).Error()))
		default:
		}
	}
}

func WithServiceName(serviceName string) Option {
	return func(observability *observability) {
		observability.serviceName = serviceName
	}
}

func WithServiceVersion(serviceVersion string) Option {
	return func(observability *observability) {
		observability.serviceVersion = serviceVersion
	}
}

func WithResource() Option {
	return func(observability *observability) {
		host, _ := os.Hostname()
		resource, err := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.HostName(host),
				semconv.ServiceName(observability.serviceName),
				semconv.ServiceVersion(observability.serviceVersion),
			),
		)

		if err != nil {
			log.Fatalf("failed to create resource: %v", err)
		}
		observability.resource = resource
	}
}

func (s *span) addLogger(ctx context.Context, code Code, description string, attrs ...Attributes) {
	slogAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		slogAttrs[i] = slog.Attr{
			Key:   attr.Key,
			Value: slog.AnyValue(attr.Value),
		}
	}

	if code == Error {
		s.logger.ErrorContext(ctx, description, slogAttrs...)
		return
	}
	s.logger.InfoContext(ctx, description, slogAttrs...)
}
