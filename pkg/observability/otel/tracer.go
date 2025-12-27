package otel

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// otelTracer implements observability.Tracer using OpenTelemetry.
type otelTracer struct {
	tracer oteltrace.Tracer
}

// newOtelTracer creates a new OpenTelemetry tracer.
func newOtelTracer(tracer oteltrace.Tracer) *otelTracer {
	return &otelTracer{tracer: tracer}
}

// Start creates a new span and returns a context containing the span.
func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	// Build span configuration
	cfg := observability.NewSpanConfig(opts)

	// Convert observability options to OTel options
	otelOpts := make([]oteltrace.SpanStartOption, 0, 2)
	otelOpts = append(otelOpts, oteltrace.WithSpanKind(convertSpanKind(cfg.Kind())))

	if len(cfg.Attributes()) > 0 {
		otelOpts = append(otelOpts, oteltrace.WithAttributes(convertFieldsToAttributes(cfg.Attributes())...))
	}

	ctx, otelSpan := t.tracer.Start(ctx, spanName, otelOpts...)

	return ctx, &otelSpanImpl{span: otelSpan}
}

// SpanFromContext returns the current span from the context.
func (t *otelTracer) SpanFromContext(ctx context.Context) observability.Span {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return &otelSpanImpl{span: oteltrace.SpanFromContext(context.Background())}
	}
	return &otelSpanImpl{span: span}
}

// ContextWithSpan returns a new context with the given span.
func (t *otelTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	if otelSpan, ok := span.(*otelSpanImpl); ok {
		return oteltrace.ContextWithSpan(ctx, otelSpan.span)
	}
	return ctx
}

// otelSpanImpl implements observability.Span using OpenTelemetry.
type otelSpanImpl struct {
	span oteltrace.Span
}

// End finishes the span.
func (s *otelSpanImpl) End() {
	s.span.End()
}

// SetAttributes sets additional attributes on the span.
func (s *otelSpanImpl) SetAttributes(fields ...observability.Field) {
	s.span.SetAttributes(convertFieldsToAttributes(fields)...)
}

// SetStatus sets the status of the span.
func (s *otelSpanImpl) SetStatus(code observability.StatusCode, description string) {
	s.span.SetStatus(convertStatusCode(code), description)
}

// RecordError records an error as an event on the span.
func (s *otelSpanImpl) RecordError(err error, fields ...observability.Field) {
	opts := []oteltrace.EventOption{}
	if len(fields) > 0 {
		opts = append(opts, oteltrace.WithAttributes(convertFieldsToAttributes(fields)...))
	}
	s.span.RecordError(err, opts...)
}

// AddEvent adds an event to the span.
func (s *otelSpanImpl) AddEvent(name string, fields ...observability.Field) {
	opts := []oteltrace.EventOption{}
	if len(fields) > 0 {
		opts = append(opts, oteltrace.WithAttributes(convertFieldsToAttributes(fields)...))
	}
	s.span.AddEvent(name, opts...)
}

// Context returns the span context.
func (s *otelSpanImpl) Context() observability.SpanContext {
	return &otelSpanContext{ctx: s.span.SpanContext()}
}

// otelSpanContext implements observability.SpanContext.
type otelSpanContext struct {
	ctx oteltrace.SpanContext
}

// TraceID returns the trace ID as a hex string.
func (c *otelSpanContext) TraceID() string {
	return c.ctx.TraceID().String()
}

// SpanID returns the span ID as a hex string.
func (c *otelSpanContext) SpanID() string {
	return c.ctx.SpanID().String()
}

// IsSampled returns whether the span is sampled.
func (c *otelSpanContext) IsSampled() bool {
	return c.ctx.IsSampled()
}

// convertSpanKind converts observability.SpanKind to oteltrace.SpanKind.
func convertSpanKind(kind observability.SpanKind) oteltrace.SpanKind {
	switch kind {
	case observability.SpanKindInternal:
		return oteltrace.SpanKindInternal
	case observability.SpanKindServer:
		return oteltrace.SpanKindServer
	case observability.SpanKindClient:
		return oteltrace.SpanKindClient
	case observability.SpanKindProducer:
		return oteltrace.SpanKindProducer
	case observability.SpanKindConsumer:
		return oteltrace.SpanKindConsumer
	default:
		return oteltrace.SpanKindInternal
	}
}

// convertStatusCode converts observability.StatusCode to codes.Code.
func convertStatusCode(code observability.StatusCode) codes.Code {
	switch code {
	case observability.StatusCodeOK:
		return codes.Ok
	case observability.StatusCodeError:
		return codes.Error
	default:
		return codes.Unset
	}
}

// convertFieldsToAttributes converts observability.Field to OTel attributes.
func convertFieldsToAttributes(fields []observability.Field) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))
	for _, field := range fields {
		attrs = append(attrs, convertFieldToAttribute(field))
	}
	return attrs
}

// convertFieldToAttribute converts a single observability.Field to an OTel attribute.
func convertFieldToAttribute(field observability.Field) attribute.KeyValue {
	switch v := field.Value.(type) {
	case string:
		return attribute.String(field.Key, v)
	case int:
		return attribute.Int(field.Key, v)
	case int64:
		return attribute.Int64(field.Key, v)
	case float64:
		return attribute.Float64(field.Key, v)
	case bool:
		return attribute.Bool(field.Key, v)
	case error:
		return attribute.String(field.Key, v.Error())
	default:
		return attribute.String(field.Key, fmt.Sprintf("%v", v))
	}
}
