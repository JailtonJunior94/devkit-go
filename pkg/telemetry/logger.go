package o11y

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdkLogger "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// Logger provides structured logging capabilities.
type Logger interface {
	Info(ctx context.Context, msg string, fields ...Field)
	Debug(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, err error, msg string, fields ...Field)
	With(fields ...Field) Logger
}

type logger struct {
	slogger        *slog.Logger
	loggerProvider *sdkLogger.LoggerProvider
	baseFields     []Field
}

// NewLogger creates a new logger with the given configuration.
// By default, TLS is enabled using system certificates. Use WithLoggerInsecure() for development.
//
// NOTE: This function sets the global OpenTelemetry LoggerProvider. Creating multiple
// instances will override the global provider. For isolated testing, use NewNoOpLogger().
func NewLogger(ctx context.Context, tracer Tracer, endpoint, serviceName string, resource *resource.Resource, opts ...LoggerOption) (Logger, func(context.Context) error, error) {
	cfg := defaultLoggerConfig(endpoint)
	for _, opt := range opts {
		opt(cfg)
	}

	httpOpts := []otlploghttp.Option{
		otlploghttp.WithEndpointURL(cfg.Endpoint),
	}

	switch {
	case cfg.Insecure:
		httpOpts = append(httpOpts, otlploghttp.WithInsecure())
	case cfg.TLSConfig != nil:
		httpOpts = append(httpOpts, otlploghttp.WithTLSClientConfig(cfg.TLSConfig))
	}

	loggerExporter, err := otlploghttp.New(ctx, httpOpts...)
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
		return loggerProvider.Shutdown(ctx)
	}

	return &logger{
		slogger:        slogger,
		loggerProvider: loggerProvider,
	}, shutdown, nil
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

// With returns a new logger with the given fields added to all log entries.
func (l *logger) With(fields ...Field) Logger {
	newFields := make([]Field, len(l.baseFields)+len(fields))
	copy(newFields, l.baseFields)
	copy(newFields[len(l.baseFields):], fields)

	return &logger{
		slogger:        l.slogger,
		loggerProvider: l.loggerProvider,
		baseFields:     newFields,
	}
}

func (l *logger) log(ctx context.Context, level slog.Level, msg string, err error, fields ...Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()

	// Calculate exact capacity needed:
	// - base fields
	// - user fields
	// - trace_id, span_id (if valid)
	// - error (if present)
	capacity := len(l.baseFields) + len(fields)
	if sc.IsValid() {
		capacity += 2 // trace_id + span_id
	}
	if err != nil {
		capacity++ // error field
	}

	attrs := make([]slog.Attr, 0, capacity)

	// Add base fields first
	for _, f := range l.baseFields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}

	// Add user-provided fields
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}

	// Add trace context if available
	if sc.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	// Add error if present (sanitized)
	if err != nil {
		attrs = append(attrs, slog.String("error", sanitizeError(err)))
	}

	l.slogger.LogAttrs(ctx, level, msg, attrs...)
}

// sanitizeError removes potentially sensitive information from error messages.
// It redacts common patterns like file paths, connection strings, and tokens.
// Customize this function based on your security requirements.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Redact common sensitive patterns
	patterns := []struct {
		pattern string
		replace string
	}{
		// Connection strings with credentials
		{`://[^:]+:[^@]+@`, "://[REDACTED]@"},
		// Bearer tokens
		{`[Bb]earer\s+[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+`, "Bearer [REDACTED]"},
		// API keys (common patterns)
		{`[Aa]pi[_-]?[Kk]ey[=:]\s*["']?[A-Za-z0-9\-_]{20,}["']?`, "api_key=[REDACTED]"},
		// Passwords in URLs or strings
		{`[Pp]assword[=:]\s*["']?[^"'\s]+["']?`, "password=[REDACTED]"},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		msg = re.ReplaceAllString(msg, p.replace)
	}

	return msg
}
