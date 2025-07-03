package o11y

import (
	"context"
	"log"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdkLogger "go.opentelemetry.io/otel/sdk/log"
)

func WithLoggerProvider(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		loggerExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpoint(endpoint))
		if err != nil {
			log.Fatalf("failed to initialize logger exporter: %v", err)
		}

		loggerProcessor := sdkLogger.NewBatchProcessor(loggerExporter)
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

		loggerProvider.Logger(observability.serviceName)
	}
}

func WithLoggerProviderHTTP(ctx context.Context, endpoint string) Option {
	return func(observability *observability) {
		loggerExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpoint(endpoint))
		if err != nil {
			log.Fatalf("failed to initialize logger exporter: %v", err)
		}

		loggerProcessor := sdkLogger.NewBatchProcessor(loggerExporter)
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

		loggerProvider.Logger(observability.serviceName)
	}
}
