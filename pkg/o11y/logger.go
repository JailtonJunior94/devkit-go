package o11y

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
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

const (
	defaultLoggerShutdownTimeout = 10 * time.Second
)

// LoggerConfig holds configuration options for the logger
type LoggerConfig struct {
	endpoint       string
	serviceName    string
	resource       *resource.Resource
	insecure       bool
	tlsConfig      *tls.Config
	registerGlobal bool
}

// LoggerOption is a function that configures a LoggerConfig
type LoggerOption func(*LoggerConfig)

// WithLoggerEndpoint sets the OTLP endpoint for the logger
func WithLoggerEndpoint(endpoint string) LoggerOption {
	return func(c *LoggerConfig) {
		c.endpoint = endpoint
	}
}

// WithLoggerServiceName sets the service name for the logger
func WithLoggerServiceName(name string) LoggerOption {
	return func(c *LoggerConfig) {
		c.serviceName = name
	}
}

// WithLoggerResource sets the resource for the logger
func WithLoggerResource(res *resource.Resource) LoggerOption {
	return func(c *LoggerConfig) {
		c.resource = res
	}
}

// WithLoggerInsecure enables insecure connection (not recommended for production)
func WithLoggerInsecure() LoggerOption {
	return func(c *LoggerConfig) {
		c.insecure = true
	}
}

// WithLoggerTLS sets custom TLS configuration
func WithLoggerTLS(cfg *tls.Config) LoggerOption {
	return func(c *LoggerConfig) {
		c.tlsConfig = cfg
	}
}

// WithLoggerGlobalRegistration enables/disables global logger provider registration
func WithLoggerGlobalRegistration(register bool) LoggerOption {
	return func(c *LoggerConfig) {
		c.registerGlobal = register
	}
}

func newLoggerConfig(opts ...LoggerOption) *LoggerConfig {
	cfg := &LoggerConfig{
		registerGlobal: true,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type logger struct {
	slogger        *slog.Logger
	loggerProvider *sdkLogger.LoggerProvider
}

// NewLogger creates a new logger with the given configuration
// Deprecated: Use NewLoggerWithOptions instead for better control over TLS
func NewLogger(ctx context.Context, tracer Tracer, endpoint, serviceName string, res *resource.Resource) (Logger, func(context.Context) error, error) {
	return NewLoggerWithOptions(ctx,
		WithLoggerEndpoint(endpoint),
		WithLoggerServiceName(serviceName),
		WithLoggerResource(res),
		WithLoggerInsecure(), // Maintain backward compatibility
	)
}

// NewLoggerWithOptions creates a new logger with functional options
func NewLoggerWithOptions(ctx context.Context, opts ...LoggerOption) (Logger, func(context.Context) error, error) {
	cfg := newLoggerConfig(opts...)

	if cfg.endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint cannot be empty")
	}
	if cfg.serviceName == "" {
		return nil, nil, fmt.Errorf("serviceName cannot be empty")
	}
	if cfg.resource == nil {
		return nil, nil, fmt.Errorf("resource cannot be nil")
	}

	exporterOpts := []otlploghttp.Option{
		otlploghttp.WithEndpointURL(cfg.endpoint),
	}
	exporterOpts = appendLoggerTLSOptions(exporterOpts, cfg)

	loggerExporter, err := otlploghttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize logger exporter: %w", err)
	}

	loggerProcessor := sdkLogger.NewBatchProcessor(loggerExporter)
	loggerProvider := sdkLogger.NewLoggerProvider(
		sdkLogger.WithProcessor(loggerProcessor),
		sdkLogger.WithResource(cfg.resource),
	)
	if loggerProvider == nil {
		// Clean up exporter to prevent resource leak
		if shutdownErr := loggerExporter.Shutdown(ctx); shutdownErr != nil {
			log.Printf("logger: failed to shutdown exporter after provider creation failed: %v", shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create logger provider")
	}

	if cfg.registerGlobal {
		global.SetLoggerProvider(loggerProvider)
	}
	slogger := otelslog.NewLogger(cfg.serviceName, otelslog.WithLoggerProvider(loggerProvider))

	shutdown := createLoggerShutdown(loggerProvider)

	return &logger{slogger: slogger, loggerProvider: loggerProvider}, shutdown, nil
}

func appendLoggerTLSOptions(opts []otlploghttp.Option, cfg *LoggerConfig) []otlploghttp.Option {
	if cfg.insecure {
		log.Printf("WARNING: logger using insecure connection to %s - not recommended for production", cfg.endpoint)
		return append(opts, otlploghttp.WithInsecure())
	}

	if cfg.tlsConfig != nil {
		return append(opts, otlploghttp.WithTLSClientConfig(cfg.tlsConfig))
	}

	// Uses system root CAs by default
	return opts
}

func createLoggerShutdown(provider *sdkLogger.LoggerProvider) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultLoggerShutdownTimeout)
			defer cancel()
		}

		if err := provider.ForceFlush(ctx); err != nil {
			log.Printf("logger: flush failed during shutdown: %v", err)
		}

		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("logger: shutdown failed: %w", err)
		}
		return nil
	}
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
	// Handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	attrs := make([]slog.Attr, 0, len(fields)+5)
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

	// slog.Logger is already thread-safe, no need for mutex
	l.slogger.LogAttrs(ctx, level, msg, attrs...)
}
