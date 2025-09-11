package o11y

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
)

func WithMeterProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		metricExporter, err := otlpmetricgrpc.New(
			ctx,
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			log.Fatalf("failed to initialize metric exporter grpc: %v", err)
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

func WithMeterProviderHTTP(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(endpoint))
		if err != nil {
			log.Fatalf("failed to initialize metric exporter grpc: %v", err)
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

func WithMeterProviderStdout() Option {
	return func(observability *observability) {
		exporter, err := stdoutmetric.New()
		if err != nil {
			log.Fatalf("failed to initialize stdout export pipeline: %v", err)
		}

		meterProvider := metric.NewMeterProvider(
			metric.WithResource(observability.resource),
			metric.WithReader(metric.NewPeriodicReader(
				exporter,
				metric.WithInterval(1*time.Second)),
			),
		)

		otel.SetMeterProvider(meterProvider)
		observability.meterProvider = meterProvider
	}
}
