package o11y

import (
	"context"
	"fmt"
	"math"

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

// Tracer provides tracing capabilities.
type Tracer interface {
	Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
	WithAttributes(ctx context.Context, attrs ...Attribute)
}

// Span represents a single operation within a trace.
type Span interface {
	End()
	SetAttributes(attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	SetStatus(status SpanStatus, msg string)
	RecordError(err error)
}

// SpanStatus represents the status of a span.
type SpanStatus int

const (
	SpanStatusOk SpanStatus = iota
	SpanStatusError
	SpanStatusUnset
)

// Attribute represents a key-value pair for span attributes.
type Attribute struct {
	Key   string
	Value any
}

type tracer struct {
	tracer trace.Tracer
}

type otelSpan struct {
	span trace.Span
}

// NewTracer creates a new tracer with the given configuration.
// By default, TLS is enabled using system certificates. Use WithTracerInsecure() for development.
//
// NOTE: This function sets the global OpenTelemetry TracerProvider. Creating multiple
// instances will override the global provider. For isolated testing, use NewNoOpTracer().
func NewTracer(ctx context.Context, endpoint, serviceName string, resource *resource.Resource, opts ...TracerOption) (Tracer, func(context.Context) error, error) {
	cfg := defaultTracerConfig(endpoint)
	for _, opt := range opts {
		opt(cfg)
	}

	grpcOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	switch {
	case cfg.Insecure:
		grpcOpts = append(grpcOpts, otlptracegrpc.WithInsecure())
	case cfg.TLSConfig != nil:
		grpcOpts = append(grpcOpts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(cfg.TLSConfig)))
	default:
		grpcOpts = append(grpcOpts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	traceExporter, err := otlptracegrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize trace exporter grpc: %w", err)
	}

	batchOpts := []sdktrace.BatchSpanProcessorOption{}
	if cfg.BatchSize > 0 {
		batchOpts = append(batchOpts, sdktrace.WithMaxExportBatchSize(cfg.BatchSize))
	}
	if cfg.BatchDelay > 0 {
		batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(cfg.BatchDelay))
	}

	providerOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource),
		sdktrace.WithBatcher(traceExporter, batchOpts...),
	}

	if cfg.Sampler != nil {
		providerOpts = append(providerOpts, sdktrace.WithSampler(cfg.Sampler))
	}

	tracerProvider := sdktrace.NewTracerProvider(providerOpts...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		return tracerProvider.Shutdown(ctx)
	}

	return &tracer{
		tracer: tracerProvider.Tracer(serviceName),
	}, shutdown, nil
}

func (t *tracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(convertAttrs(attrs)...))
	return ctx, &otelSpan{span: span}
}

func (t *tracer) WithAttributes(ctx context.Context, attrs ...Attribute) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.SetAttributes(convertAttrs(attrs)...)
	}
}

func (s *otelSpan) End() {
	s.span.End()
}

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	s.span.SetAttributes(convertAttrs(attrs)...)
}

func (s *otelSpan) AddEvent(name string, attrs ...Attribute) {
	s.span.AddEvent(name, trace.WithAttributes(convertAttrs(attrs)...))
}

func (s *otelSpan) RecordError(err error) {
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
}

var statusMap = map[SpanStatus]codes.Code{
	SpanStatusOk:    codes.Ok,
	SpanStatusError: codes.Error,
	SpanStatusUnset: codes.Unset,
}

func (s *otelSpan) SetStatus(status SpanStatus, msg string) {
	if code, ok := statusMap[status]; ok {
		s.span.SetStatus(code, msg)
		return
	}
	s.span.SetStatus(codes.Unset, msg)
}

func convertAttrs(attrs []Attribute) []attribute.KeyValue {
	kv := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if attr, ok := convertAttr(a); ok {
			kv = append(kv, attr)
		}
	}
	return kv
}

func convertAttr(a Attribute) (attribute.KeyValue, bool) {
	switch v := a.Value.(type) {
	case string:
		return attribute.String(a.Key, v), true
	case int:
		return attribute.Int(a.Key, v), true
	case int64:
		return attribute.Int64(a.Key, v), true
	case int32:
		return attribute.Int64(a.Key, int64(v)), true
	case int16:
		return attribute.Int64(a.Key, int64(v)), true
	case int8:
		return attribute.Int64(a.Key, int64(v)), true
	case uint:
		if v > math.MaxInt64 {
			return attribute.String(a.Key, fmt.Sprintf("%d", v)), true
		}
		return attribute.Int64(a.Key, int64(v)), true
	case uint64:
		if v > math.MaxInt64 {
			return attribute.String(a.Key, fmt.Sprintf("%d", v)), true
		}
		return attribute.Int64(a.Key, int64(v)), true
	case uint32:
		return attribute.Int64(a.Key, int64(v)), true
	case uint16:
		return attribute.Int64(a.Key, int64(v)), true
	case uint8:
		return attribute.Int64(a.Key, int64(v)), true
	case bool:
		return attribute.Bool(a.Key, v), true
	case float64:
		return attribute.Float64(a.Key, v), true
	case float32:
		return attribute.Float64(a.Key, float64(v)), true
	case []string:
		return attribute.StringSlice(a.Key, v), true
	case []int:
		return attribute.IntSlice(a.Key, v), true
	case []int64:
		return attribute.Int64Slice(a.Key, v), true
	case []float64:
		return attribute.Float64Slice(a.Key, v), true
	case []bool:
		return attribute.BoolSlice(a.Key, v), true
	case fmt.Stringer:
		return attribute.String(a.Key, v.String()), true
	default:
		return attribute.String(a.Key, fmt.Sprintf("%v", v)), true
	}
}

// TraceIDFromContext extracts the trace ID from the context.
// Returns empty string if no valid trace is present.
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts the span ID from the context.
// Returns empty string if no valid span is present.
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
