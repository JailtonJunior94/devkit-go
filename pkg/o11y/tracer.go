package o11y

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

type Tracer interface {
	Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
	WithAttributes(ctx context.Context, attrs ...Attribute)
}

type Span interface {
	End()
	SetAttributes(attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	SetStatus(status SpanStatus, msg string)
}

type SpanStatus int

const (
	SpanStatusOk SpanStatus = iota
	SpanStatusError
	SpanStatusUnset
)

type Attribute struct {
	Key   string
	Value any
}

const (
	defaultTracerBatchTimeout   = 5 * time.Second
	defaultTracerMaxExportBatch = 512
	defaultTracerShutdownTimeout = 10 * time.Second
)

// TracerConfig holds configuration options for the tracer
type TracerConfig struct {
	endpoint        string
	serviceName     string
	resource        *resource.Resource
	insecure        bool
	tlsConfig       *tls.Config
	registerGlobal  bool
	batchTimeout    time.Duration
	maxExportBatch  int
}

// TracerOption is a function that configures a TracerConfig
type TracerOption func(*TracerConfig)

// WithTracerEndpoint sets the OTLP endpoint for the tracer
func WithTracerEndpoint(endpoint string) TracerOption {
	return func(c *TracerConfig) {
		c.endpoint = endpoint
	}
}

// WithTracerServiceName sets the service name for the tracer
func WithTracerServiceName(name string) TracerOption {
	return func(c *TracerConfig) {
		c.serviceName = name
	}
}

// WithTracerResource sets the resource for the tracer
func WithTracerResource(res *resource.Resource) TracerOption {
	return func(c *TracerConfig) {
		c.resource = res
	}
}

// WithTracerInsecure enables insecure connection (not recommended for production)
func WithTracerInsecure() TracerOption {
	return func(c *TracerConfig) {
		c.insecure = true
	}
}

// WithTracerTLS sets custom TLS configuration
func WithTracerTLS(cfg *tls.Config) TracerOption {
	return func(c *TracerConfig) {
		c.tlsConfig = cfg
	}
}

// WithTracerGlobalRegistration enables/disables global tracer provider registration
func WithTracerGlobalRegistration(register bool) TracerOption {
	return func(c *TracerConfig) {
		c.registerGlobal = register
	}
}

// WithTracerBatchTimeout sets the batch timeout for span export
func WithTracerBatchTimeout(timeout time.Duration) TracerOption {
	return func(c *TracerConfig) {
		c.batchTimeout = timeout
	}
}

// WithTracerMaxExportBatch sets the maximum batch size for span export
func WithTracerMaxExportBatch(size int) TracerOption {
	return func(c *TracerConfig) {
		c.maxExportBatch = size
	}
}

func newTracerConfig(opts ...TracerOption) *TracerConfig {
	cfg := &TracerConfig{
		registerGlobal: true,
		batchTimeout:   defaultTracerBatchTimeout,
		maxExportBatch: defaultTracerMaxExportBatch,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type tracer struct {
	tracer trace.Tracer
}

type otelSpan struct {
	span trace.Span
}

// NewTracer creates a new tracer with the given configuration
// Deprecated: Use NewTracerWithOptions instead for better control over TLS and batching
func NewTracer(ctx context.Context, endpoint, serviceName string, res *resource.Resource) (Tracer, func(context.Context) error, error) {
	return NewTracerWithOptions(ctx,
		WithTracerEndpoint(endpoint),
		WithTracerServiceName(serviceName),
		WithTracerResource(res),
		WithTracerInsecure(), // Maintain backward compatibility
	)
}

// NewTracerWithOptions creates a new tracer with functional options
func NewTracerWithOptions(ctx context.Context, opts ...TracerOption) (Tracer, func(context.Context) error, error) {
	cfg := newTracerConfig(opts...)

	if cfg.endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint cannot be empty")
	}
	if cfg.serviceName == "" {
		return nil, nil, fmt.Errorf("serviceName cannot be empty")
	}
	if cfg.resource == nil {
		return nil, nil, fmt.Errorf("resource cannot be nil")
	}
	if cfg.batchTimeout <= 0 {
		return nil, nil, fmt.Errorf("batchTimeout must be greater than 0")
	}
	if cfg.maxExportBatch <= 0 {
		return nil, nil, fmt.Errorf("maxExportBatch must be greater than 0")
	}

	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.endpoint),
	}
	exporterOpts = appendTracerTLSOptions(exporterOpts, cfg)

	traceExporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize trace exporter grpc: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(cfg.resource),
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(cfg.batchTimeout),
			sdktrace.WithMaxExportBatchSize(cfg.maxExportBatch),
		),
	)
	if tracerProvider == nil {
		// Clean up exporter to prevent resource leak
		if shutdownErr := traceExporter.Shutdown(ctx); shutdownErr != nil {
			log.Printf("tracer: failed to shutdown exporter after provider creation failed: %v", shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create tracer provider")
	}

	if cfg.registerGlobal {
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	}

	shutdown := createTracerShutdown(tracerProvider)

	return &tracer{tracer: tracerProvider.Tracer(cfg.serviceName)}, shutdown, nil
}

func appendTracerTLSOptions(opts []otlptracegrpc.Option, cfg *TracerConfig) []otlptracegrpc.Option {
	if cfg.insecure {
		log.Printf("WARNING: tracer using insecure connection to %s - not recommended for production", cfg.endpoint)
		return append(opts, otlptracegrpc.WithInsecure())
	}

	if cfg.tlsConfig != nil {
		return append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(cfg.tlsConfig)))
	}

	// Uses system root CAs by default
	return opts
}

func createTracerShutdown(provider *sdktrace.TracerProvider) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTracerShutdownTimeout)
			defer cancel()
		}

		if err := provider.ForceFlush(ctx); err != nil {
			log.Printf("tracer: flush failed during shutdown: %v", err)
		}

		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("tracer: shutdown failed: %w", err)
		}
		return nil
	}
}

func (t *tracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(convertAttrs(attrs)...))
	return ctx, &otelSpan{span: span}
}

func (t *tracer) WithAttributes(ctx context.Context, attrs ...Attribute) {
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		span.SetAttributes(convertAttrs(attrs)...)
	}
}

func (s *otelSpan) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(convertAttrs(attrs)...)
}

func (s *otelSpan) AddEvent(name string, attrs ...Attribute) {
	if s == nil || s.span == nil {
		return
	}
	s.span.AddEvent(name, trace.WithAttributes(convertAttrs(attrs)...))
}

var statusMap = map[SpanStatus]codes.Code{
	SpanStatusOk:    codes.Ok,
	SpanStatusError: codes.Error,
	SpanStatusUnset: codes.Unset,
}

func (s *otelSpan) SetStatus(status SpanStatus, msg string) {
	if s == nil || s.span == nil {
		return
	}
	if code, ok := statusMap[status]; ok {
		s.span.SetStatus(code, msg)
		return
	}
	s.span.SetStatus(codes.Unset, msg)
}

func convertAttrs(attrs []Attribute) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	kv := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		switch v := a.Value.(type) {
		case string:
			kv = append(kv, attribute.String(a.Key, v))
		case int:
			kv = append(kv, attribute.Int(a.Key, v))
		case int64:
			kv = append(kv, attribute.Int64(a.Key, v))
		case int32:
			kv = append(kv, attribute.Int64(a.Key, int64(v)))
		case float64:
			kv = append(kv, attribute.Float64(a.Key, v))
		case float32:
			kv = append(kv, attribute.Float64(a.Key, float64(v)))
		case bool:
			kv = append(kv, attribute.Bool(a.Key, v))
		case []string:
			kv = append(kv, attribute.StringSlice(a.Key, v))
		case []int:
			kv = append(kv, attribute.IntSlice(a.Key, v))
		case []int64:
			kv = append(kv, attribute.Int64Slice(a.Key, v))
		case []float64:
			kv = append(kv, attribute.Float64Slice(a.Key, v))
		case []bool:
			kv = append(kv, attribute.BoolSlice(a.Key, v))
		case fmt.Stringer:
			kv = append(kv, attribute.String(a.Key, v.String()))
		case nil:
			kv = append(kv, attribute.String(a.Key, "<nil>"))
		default:
			log.Printf("o11y: unsupported attribute type %T for key %q, using fmt.Sprintf", v, a.Key)
			kv = append(kv, attribute.String(a.Key, fmt.Sprintf("%+v", v)))
		}
	}
	return kv
}
