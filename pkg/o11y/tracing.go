package o11y

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func WithTracerProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		traceExporter, err := otlptracegrpc.New(
			ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			log.Fatalf("failed to initialize trace exporter: %v", err)
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

func WithTracerProviderHTTP(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint))
		if err != nil {
			log.Fatalf("failed to initialize trace exporter: %v", err)
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

func WithTracerProviderStdout() Option {
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

func WithTracerProviderMemory() Option {
	return func(observability *observability) {
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.NeverSample()),
			sdktrace.WithSyncer(tracetest.NewInMemoryExporter()),
		)

		observability.tracer = tracerProvider.Tracer(observability.serviceName)
		observability.tracerProvider = tracerProvider
	}
}
