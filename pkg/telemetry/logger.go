package o11y

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdkLogger "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

type Field struct {
	Key   string
	Value any
}

type Logger interface {
	Info(ctx context.Context, msg string, fields ...Field)
	Debug(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, err error, msg string, fields ...Field)
}

type logger struct {
	tracer         Tracer
	slogger        *slog.Logger
	mu             sync.RWMutex
	loggerProvider *sdkLogger.LoggerProvider
}

func NewLogger(ctx context.Context, tracer Tracer, endpoint, serviceName string, resource *resource.Resource) (Logger, func(context.Context) error, error) {
	loggerExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize logger exporter: %w", err)
	}

	loggerProcessor := sdkLogger.NewBatchProcessor(loggerExporter)
	loggerProvider := sdkLogger.NewLoggerProvider(
		sdkLogger.WithProcessor(loggerProcessor),
		sdkLogger.WithResource(resource),
	)
	global.SetLoggerProvider(loggerProvider)
	slogger := otelslog.NewLogger(serviceName, otelslog.WithLoggerProvider(loggerProvider))

	shutdown := func(ctx context.Context) error {
		if err := loggerProvider.Shutdown(ctx); err != nil {
			return err
		}
		return nil
	}

	return &logger{tracer: tracer, slogger: slogger, loggerProvider: loggerProvider}, shutdown, nil
}

func (l *logger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, slog.LevelDebug, msg, nil, fields...)
}

func (l *logger) Info(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, slog.LevelInfo, msg, nil, fields...)
}

func (l *logger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.log(ctx, slog.LevelWarn, msg, nil, fields...)
}

func (l *logger) Error(ctx context.Context, err error, msg string, fields ...Field) {
	l.log(ctx, slog.LevelError, msg, err, fields...)
}

func (l *logger) log(ctx context.Context, level slog.Level, msg string, err error, fields ...Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	attrs := make([]slog.Attr, 0, len(fields)+3)
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}

	if sc.IsValid() {
		attrs = append(attrs, slog.String("trace_id", sc.TraceID().String()))
		attrs = append(attrs, slog.String("span_id", sc.SpanID().String()))
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	attrs = append(attrs, slog.String("level", level.String()))
	attrs = append(attrs, slog.Time("ts", time.Now()))

	l.mu.RLock()
	defer l.mu.RUnlock()
	l.slogger.LogAttrs(ctx, level, msg, attrs...)
}
