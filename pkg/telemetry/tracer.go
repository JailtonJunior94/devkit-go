package o11y

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
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

type tracer struct {
	tracer trace.Tracer
}

type otelSpan struct {
	span trace.Span
}

func NewTracer(ctx context.Context, endpoint, serviceName string, resource *resource.Resource) (Tracer, func(context.Context) error, error) {
	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize trace exporter grpc: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource),
		sdktrace.WithSyncer(traceExporter),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	shutdown := func(ctx context.Context) error {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return err
		}
		return nil
	}

	return &tracer{tracer: tracerProvider.Tracer(serviceName)}, shutdown, nil
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
	s.span.End()
}

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	s.span.SetAttributes(convertAttrs(attrs)...)
}

func (s *otelSpan) AddEvent(name string, attrs ...Attribute) {
	s.span.AddEvent(name, trace.WithAttributes(convertAttrs(attrs)...))
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
	kv := make([]attribute.KeyValue, len(attrs))
	for i, a := range attrs {
		switch v := a.Value.(type) {
		case string:
			kv[i] = attribute.String(a.Key, v)
		case int:
			kv[i] = attribute.Int(a.Key, v)
		case bool:
			kv[i] = attribute.Bool(a.Key, v)
		case float64:
			kv[i] = attribute.Float64(a.Key, v)
		default:
			kv[i] = attribute.String(a.Key, "unsupported_type")
		}
	}
	return kv
}
