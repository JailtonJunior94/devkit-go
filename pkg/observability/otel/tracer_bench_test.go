package otel

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newBenchTracer returns an otelTracer backed by a no-export, always-sample provider.
func newBenchTracer() *otelTracer {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	return newOtelTracer(tp.Tracer("bench"))
}

// BenchmarkStart measures the fast path: Start with no options.
func BenchmarkStart(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newCtx, span := t.Start(ctx, "op")
		span.End()
		_ = newCtx
	}
}

// BenchmarkSpanFromContext_FastPath measures SpanFromContext when the span was
// created by our Start() — wrapper already in context, zero allocations expected.
func BenchmarkSpanFromContext_FastPath(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	ctx, span := t.Start(ctx, "parent")
	defer span.End()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := t.SpanFromContext(ctx)
		_ = s
	}
}

// BenchmarkSpanFromContext_NoSpan measures SpanFromContext with no active span.
// Expected: 0 allocs (returns globalNoopOtelSpan singleton).
func BenchmarkSpanFromContext_NoSpan(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := t.SpanFromContext(ctx)
		_ = s
	}
}

// BenchmarkTraceID measures span.TraceID() — 1 alloc (string copy from stack buffer).
func BenchmarkTraceID(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	_, span := t.Start(ctx, "op")
	defer span.End()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = span.TraceID()
	}
}

// BenchmarkContextTraceID measures the legacy path span.Context().TraceID().
// Expected: 2 allocs (*otelSpanContext struct + TraceID.String()).
func BenchmarkContextTraceID(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	_, span := t.Start(ctx, "op")
	defer span.End()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = span.Context().TraceID()
	}
}

// BenchmarkStartWithKind measures Start with WithSpanKind — exercises the slow path
// and verifies spanOptsPool avoids the make([]SpanStartOption) alloc per call.
func BenchmarkStartWithKind(b *testing.B) {
	t := newBenchTracer()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newCtx, span := t.Start(ctx, "op",
			observability.WithSpanKind(observability.SpanKindServer),
		)
		span.End()
		_ = newCtx
	}
}
