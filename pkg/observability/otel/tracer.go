package otel

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// otelSpanContextKey is the context key used to store *otelSpanImpl wrappers.
// Using our own key (in addition to the OTel SDK's internal key) lets
// SpanFromContext retrieve the wrapper without allocating a new one.
type otelSpanContextKey struct{}

// otelSpanPool reuses otelSpanImpl wrappers to eliminate per-span heap allocation.
// Spans are acquired via acquireSpanImpl and released inside End().
// SpanFromContext results obtained via the fast path are NOT separately pooled:
// the caller receives the same pointer stored in the context.
var otelSpanPool = sync.Pool{New: func() any { return &otelSpanImpl{} }}

// spanOptsPool reuses the small []oteltrace.SpanStartOption backing arrays
// used when Start() is called with options. Capacity 2 covers the common case
// (WithSpanKind + WithAttributes) without reallocation.
var spanOptsPool = sync.Pool{New: func() any {
	s := make([]oteltrace.SpanStartOption, 0, 2)
	return &s
}}

func acquireSpanImpl() *otelSpanImpl {
	s := otelSpanPool.Get().(*otelSpanImpl)
	atomic.StoreUint32(&s.ended, 0)
	return s
}

// noopOtelSpan is the zero-size fallback returned by SpanFromContext when no
// active span exists in context. Zero-size type: interface boxing uses
// runtime.zerobase — no heap allocation.
type noopOtelSpan struct{}

var (
	globalNoopOtelSpan    observability.Span        = noopOtelSpan{}
	globalNoopOtelSpanCtx observability.SpanContext  = noopOtelSpanCtx{}
)

func (noopOtelSpan) End()                                                   {}
func (noopOtelSpan) SetAttributes(_ ...observability.Field)                 {}
func (noopOtelSpan) SetStatus(_ observability.StatusCode, _ string)         {}
func (noopOtelSpan) RecordError(_ error, _ ...observability.Field)          {}
func (noopOtelSpan) AddEvent(_ string, _ ...observability.Field)            {}
func (noopOtelSpan) Context() observability.SpanContext                     { return globalNoopOtelSpanCtx }
func (noopOtelSpan) TraceID() string                                        { return "" }
func (noopOtelSpan) SpanID() string                                         { return "" }
func (noopOtelSpan) IsSampled() bool                                        { return false }

// noopOtelSpanCtx is the zero-size SpanContext returned by noopOtelSpan.Context().
type noopOtelSpanCtx struct{}

func (noopOtelSpanCtx) TraceID() string  { return "" }
func (noopOtelSpanCtx) SpanID() string   { return "" }
func (noopOtelSpanCtx) IsSampled() bool  { return false }

// otelTracer implements observability.Tracer using OpenTelemetry.
type otelTracer struct {
	tracer oteltrace.Tracer
}

// newOtelTracer creates a new OpenTelemetry tracer.
func newOtelTracer(tracer oteltrace.Tracer) *otelTracer {
	return &otelTracer{tracer: tracer}
}

// Start creates a new span and returns a context containing the span.
//
// Fast path (len(opts) == 0 — the common case):
//   - Skips *SpanConfig + []SpanStartOption allocations entirely.
//   - Stores *otelSpanImpl in context under otelSpanContextKey so that
//     SpanFromContext can retrieve it without allocating a new wrapper.
//
// Slow path (opts provided):
//   - Borrows a []SpanStartOption from spanOptsPool to avoid a make alloc.
func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	if len(opts) == 0 {
		ctx, otelSpan := t.tracer.Start(ctx, spanName)
		s := acquireSpanImpl()
		s.span = otelSpan
		// Store our wrapper under our own key so SpanFromContext is zero-alloc.
		return context.WithValue(ctx, otelSpanContextKey{}, s), s
	}

	// Stack-allocate SpanConfig to avoid the *SpanConfig heap allocation
	// that NewSpanConfig previously incurred on every traced call with options.
	var cfg observability.SpanConfig
	observability.ApplySpanOptions(&cfg, opts)

	// Borrow a slice from pool instead of make([], 0, 2) per call.
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

	// Return opts slice to pool before acquiring span wrapper.
	*p = otelOpts[:0]
	spanOptsPool.Put(p)

	s := acquireSpanImpl()
	s.span = otelSpan
	return context.WithValue(ctx, otelSpanContextKey{}, s), s
}

// SpanFromContext returns the current span from the context.
//
// Fast path (span created by our Start):
//   - Retrieves the *otelSpanImpl stored under otelSpanContextKey.
//   - Zero allocations.
//
// Slow path (span created externally, e.g. by OTel middleware):
//   - Falls back to OTel SDK's SpanFromContext.
//   - Allocates 1 *otelSpanImpl wrapper (unavoidable for foreign spans).
//
// If no active span exists at all, returns globalNoopOtelSpan (zero alloc).
//
// NOTE: the returned span must not have End() called on it unless the caller
// owns the span's lifecycle (i.e., received it from Start()).
func (t *otelTracer) SpanFromContext(ctx context.Context) observability.Span {
	// Fast path: our wrapper is already in context.
	if s, ok := ctx.Value(otelSpanContextKey{}).(*otelSpanImpl); ok {
		if atomic.LoadUint32(&s.ended) == 0 {
			return s
		}
	}

	// Slow path: check OTel SDK context (spans injected by external middleware).
	otelSpan := oteltrace.SpanFromContext(ctx)
	if !otelSpan.SpanContext().IsValid() {
		return globalNoopOtelSpan
	}
	// Foreign span: must wrap (not pooled — caller does not own lifecycle).
	return &otelSpanImpl{span: otelSpan}
}

// ContextWithSpan returns a new context carrying the given span under both
// the OTel SDK key and our own otelSpanContextKey, so that SpanFromContext
// can use the zero-alloc fast path for spans stored via this method.
func (t *otelTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	otelSpan, ok := span.(*otelSpanImpl)
	if !ok {
		return ctx
	}
	ctx = oteltrace.ContextWithSpan(ctx, otelSpan.span)
	return context.WithValue(ctx, otelSpanContextKey{}, otelSpan)
}

// otelSpanImpl implements observability.Span using OpenTelemetry.
// Instances are managed by otelSpanPool; End() is the release point.
// ended is accessed exclusively via sync/atomic to guard against double-End
// and pool corruption without a mutex in the hot path.
type otelSpanImpl struct {
	span  oteltrace.Span
	ended uint32 // 0 = active, 1 = ended; access via sync/atomic only
}

// End finishes the span and returns the wrapper to the pool.
// Safe to call from multiple goroutines: the first call wins via CAS;
// subsequent calls are no-ops. Per the OTel spec, no operations should
// be performed on a span after End.
func (s *otelSpanImpl) End() {
	if !atomic.CompareAndSwapUint32(&s.ended, 0, 1) {
		return
	}
	s.span.End()
	s.span = nil
	otelSpanPool.Put(s)
}

// SetAttributes sets additional attributes on the span.
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

// SetStatus sets the status of the span.
func (s *otelSpanImpl) SetStatus(code observability.StatusCode, description string) {
	s.span.SetStatus(convertStatusCode(code), description)
}

// RecordError records an error as an event on the span.
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

// AddEvent adds an event to the span.
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

// Context returns the span context.
func (s *otelSpanImpl) Context() observability.SpanContext {
	return &otelSpanContext{ctx: s.span.SpanContext()}
}

// TraceID returns the trace ID as a lowercase hex string.
// Uses stack-allocated [32]byte buffer: 1 alloc (string copy) instead of the
// 2 allocs incurred by Context().TraceID() (*otelSpanContext + TraceID.String()).
func (s *otelSpanImpl) TraceID() string {
	tid := s.span.SpanContext().TraceID()
	var buf [32]byte
	encodeHex(buf[:], tid[:])
	return string(buf[:])
}

// SpanID returns the span ID as a lowercase hex string.
// Uses stack-allocated [16]byte buffer: 1 alloc (string copy) instead of 2.
func (s *otelSpanImpl) SpanID() string {
	sid := s.span.SpanContext().SpanID()
	var buf [16]byte
	encodeHex(buf[:], sid[:])
	return string(buf[:])
}

// IsSampled reports whether the span is sampled.
func (s *otelSpanImpl) IsSampled() bool {
	return s.span.SpanContext().IsSampled()
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
