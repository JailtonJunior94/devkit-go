package otel

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type otelSpanContextKey struct{}

var otelSpanPool = sync.Pool{New: func() any { return &otelSpanImpl{} }}

var spanOptsPool = sync.Pool{New: func() any {
	s := make([]oteltrace.SpanStartOption, 0, 2)
	return &s
}}

func acquireSpanImpl() *otelSpanImpl {
	s := otelSpanPool.Get().(*otelSpanImpl)
	s.ended.Store(0)
	return s
}

type noopOtelSpan struct{}

var (
	globalNoopOtelSpan    observability.Span        = noopOtelSpan{}
	globalNoopOtelSpanCtx observability.SpanContext = noopOtelSpanCtx{}
)

func (noopOtelSpan) End()                                           {}
func (noopOtelSpan) SetAttributes(_ ...observability.Field)         {}
func (noopOtelSpan) SetStatus(_ observability.StatusCode, _ string) {}
func (noopOtelSpan) RecordError(_ error, _ ...observability.Field)  {}
func (noopOtelSpan) AddEvent(_ string, _ ...observability.Field)    {}
func (noopOtelSpan) Context() observability.SpanContext             { return globalNoopOtelSpanCtx }
func (noopOtelSpan) TraceID() string                                { return "" }
func (noopOtelSpan) SpanID() string                                 { return "" }
func (noopOtelSpan) IsSampled() bool                                { return false }

type noopOtelSpanCtx struct{}

func (noopOtelSpanCtx) TraceID() string { return "" }
func (noopOtelSpanCtx) SpanID() string  { return "" }
func (noopOtelSpanCtx) IsSampled() bool { return false }

type otelTracer struct {
	tracer oteltrace.Tracer
}

func newOtelTracer(tracer oteltrace.Tracer) *otelTracer {
	return &otelTracer{tracer: tracer}
}

func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	if len(opts) == 0 {
		ctx, otelSpan := t.tracer.Start(ctx, spanName)
		s := acquireSpanImpl()
		s.span = otelSpan
		return context.WithValue(ctx, otelSpanContextKey{}, s), s
	}

	var cfg observability.SpanConfig
	observability.ApplySpanOptions(&cfg, opts)

	p := spanOptsPool.Get().(*[]oteltrace.SpanStartOption)
	otelOpts := (*p)[:0]
	otelOpts = append(otelOpts, oteltrace.WithSpanKind(convertSpanKind(cfg.Kind())))

	if cfgAttrs := cfg.Attributes(); len(cfgAttrs) > 0 {
		ap := acquireAttrs()
		attrs := appendFieldAttrs((*ap)[:0], cfgAttrs)
		otelOpts = append(otelOpts, oteltrace.WithAttributes(attrs...))
		*ap = attrs
		releaseAttrs(ap)
	}

	ctx, otelSpan := t.tracer.Start(ctx, spanName, otelOpts...)

	*p = otelOpts[:0]
	spanOptsPool.Put(p)

	s := acquireSpanImpl()
	s.span = otelSpan
	return context.WithValue(ctx, otelSpanContextKey{}, s), s
}

func (t *otelTracer) SpanFromContext(ctx context.Context) observability.Span {
	if s, ok := ctx.Value(otelSpanContextKey{}).(*otelSpanImpl); ok {
		if s.ended.Load() == 0 {
			return s
		}
	}

	otelSpan := oteltrace.SpanFromContext(ctx)
	if !otelSpan.SpanContext().IsValid() {
		return globalNoopOtelSpan
	}
	return &otelSpanImpl{span: otelSpan}
}

func (t *otelTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	otelSpan, ok := span.(*otelSpanImpl)
	if !ok {
		return ctx
	}
	ctx = oteltrace.ContextWithSpan(ctx, otelSpan.span)
	return context.WithValue(ctx, otelSpanContextKey{}, otelSpan)
}

type otelSpanImpl struct {
	span  oteltrace.Span
	ended atomic.Uint32
}

func (s *otelSpanImpl) End() {
	if !s.ended.CompareAndSwap(0, 1) {
		return
	}
	s.span.End()
	s.span = nil
	otelSpanPool.Put(s)
}

func (s *otelSpanImpl) SetAttributes(fields ...observability.Field) {
	if len(fields) == 0 {
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	s.span.SetAttributes(attrs...)
	*p = attrs
	releaseAttrs(p)
}

func (s *otelSpanImpl) SetStatus(code observability.StatusCode, description string) {
	s.span.SetStatus(convertStatusCode(code), description)
}

func (s *otelSpanImpl) RecordError(err error, fields ...observability.Field) {
	if len(fields) == 0 {
		s.span.RecordError(err)
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	s.span.RecordError(err, oteltrace.WithAttributes(attrs...))
	*p = attrs
	releaseAttrs(p)
}

func (s *otelSpanImpl) AddEvent(name string, fields ...observability.Field) {
	if len(fields) == 0 {
		s.span.AddEvent(name)
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	s.span.AddEvent(name, oteltrace.WithAttributes(attrs...))
	*p = attrs
	releaseAttrs(p)
}

func (s *otelSpanImpl) Context() observability.SpanContext {
	return &otelSpanContext{ctx: s.span.SpanContext()}
}

func (s *otelSpanImpl) TraceID() string {
	tid := s.span.SpanContext().TraceID()
	var buf [32]byte
	encodeHex(buf[:], tid[:])
	return string(buf[:])
}

func (s *otelSpanImpl) SpanID() string {
	sid := s.span.SpanContext().SpanID()
	var buf [16]byte
	encodeHex(buf[:], sid[:])
	return string(buf[:])
}

func (s *otelSpanImpl) IsSampled() bool {
	return s.span.SpanContext().IsSampled()
}

type otelSpanContext struct {
	ctx oteltrace.SpanContext
}

func (c *otelSpanContext) TraceID() string { return c.ctx.TraceID().String() }
func (c *otelSpanContext) SpanID() string  { return c.ctx.SpanID().String() }
func (c *otelSpanContext) IsSampled() bool { return c.ctx.IsSampled() }

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
