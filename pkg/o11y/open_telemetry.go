package o11y

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdkLogger "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Code uint

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
		WithStdoutTracerProvider(),
		WithStdoutMeterProvider(),
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
				semconv.ServiceName(observability.serviceName),
				semconv.ServiceVersion(observability.serviceVersion),
				semconv.HostName(host),
			),
		)

		if err != nil {
			log.Fatal(err)
		}
		observability.resource = resource
	}
}

func WithTracerProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		traceExporter, err := otlptracegrpc.New(
			ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			log.Fatal(err)
		}

		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(traceExporter),
			sdktrace.WithResource(observability.resource),
		)

		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

		observability.tracer = tracerProvider.Tracer(observability.serviceName)
		observability.tracerProvider = tracerProvider
	}
}

func WithLoggerProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		loggerExporter, err := otlploggrpc.New(
			ctx,
			otlploggrpc.WithInsecure(),
			otlploggrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			log.Fatal(err)
		}

		loggerProcessor := sdkLogger.NewSimpleProcessor(loggerExporter)
		loggerProvider := sdkLogger.NewLoggerProvider(
			sdkLogger.WithProcessor(loggerProcessor),
			sdkLogger.WithResource(observability.resource),
		)

		global.SetLoggerProvider(loggerProvider)
		observability.logger = otelslog.NewLogger(
			observability.serviceName,
			otelslog.WithLoggerProvider(loggerProvider),
			otelslog.WithVersion(observability.serviceVersion),
		)

		// observability.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
		loggerProvider.Logger(observability.serviceName)
	}
}

func WithStdoutTracerProvider() Option {
	return func(observability *observability) {
		exporter, err := stdouttrace.New()
		if err != nil {
			log.Fatalf("failed to initialize stdout export pipeline: %v", err)
		}

		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSyncer(exporter),
		)

		observability.tracer = tracerProvider.Tracer(observability.serviceName)
		observability.tracerProvider = tracerProvider
	}
}

func WithMeterProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		metricExporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithEndpoint(endpoint),
		)

		if err != nil {
			log.Fatal(err)
		}

		meterProvider := metric.NewMeterProvider(
			metric.WithResource(observability.resource),
			metric.WithReader(metric.NewPeriodicReader(
				metricExporter,
				metric.WithInterval(2*time.Second)),
			),
		)

		otel.SetMeterProvider(meterProvider)
		observability.meterProvider = meterProvider
	}
}

func WithStdoutMeterProvider() Option {
	return func(observability *observability) {
		exporter, err := stdoutmetric.New()
		if err != nil {
			log.Fatalf("failed to initialize stdout export pipeline: %v", err)
		}

		meterProvider := metric.NewMeterProvider(
			metric.WithResource(observability.resource),
			metric.WithReader(metric.NewPeriodicReader(
				exporter,
				metric.WithInterval(2*time.Second)),
			),
		)

		otel.SetMeterProvider(meterProvider)
		observability.meterProvider = meterProvider
	}
}
