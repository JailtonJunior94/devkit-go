package otel

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
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
	cfg := observability.NewSpanConfig(opts)

	initialCap := 1
	if len(cfg.Attributes()) > 0 {
		initialCap++
	}

	otelOpts := make([]oteltrace.SpanStartOption, 0, initialCap)
	otelOpts = append(otelOpts, oteltrace.WithSpanKind(convertSpanKind(cfg.Kind())))

	attrs := convertFieldsToAttributes(cfg.Attributes())
	if attrs != nil {
		otelOpts = append(otelOpts, oteltrace.WithAttributes(attrs...))
	}

	ctx, otelSpan := t.tracer.Start(ctx, spanName, otelOpts...)
	return ctx, &otelSpanImpl{span: otelSpan}
}

// SpanFromContext returns the current span from the context.
// If no active span exists, returns a non-recording noop span.
// Operations on the returned span are safe but will have no effect if it's non-recording.
func (t *otelTracer) SpanFromContext(ctx context.Context) observability.Span {
	span := oteltrace.SpanFromContext(ctx)
	return &otelSpanImpl{span: span}
}

// ContextWithSpan returns a new context with the given span.
func (t *otelTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	otelSpan, ok := span.(*otelSpanImpl)
	if !ok {
		return ctx
	}

	return oteltrace.ContextWithSpan(ctx, otelSpan.span)
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
	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		return
	}

	s.span.SetAttributes(attrs...)
}

// SetStatus sets the status of the span.
func (s *otelSpanImpl) SetStatus(code observability.StatusCode, description string) {
	s.span.SetStatus(convertStatusCode(code), description)
}

// RecordError records an error as an event on the span.
func (s *otelSpanImpl) RecordError(err error, fields ...observability.Field) {
	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		s.span.RecordError(err)
		return
	}

	s.span.RecordError(err, oteltrace.WithAttributes(attrs...))
}

// AddEvent adds an event to the span.
func (s *otelSpanImpl) AddEvent(name string, fields ...observability.Field) {
	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		s.span.AddEvent(name)
		return
	}

	s.span.AddEvent(name, oteltrace.WithAttributes(attrs...))
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
