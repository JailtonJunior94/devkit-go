package otel

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
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

func BenchmarkLoggerInfoNoFields(b *testing.B) {
	loggerProvider := sdklog.NewLoggerProvider()
	logger := newOtelLogger(
		observability.LogLevelInfo,
		observability.LogFormatJSON,
		"bench-service",
		"benchmark",
		loggerProvider,
		false,
		false,
	)
	ctx := contextWithBenchTraceAndCorrelation(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info(ctx, "bench message")
	}
}

func BenchmarkLoggerInfoWithFields(b *testing.B) {
	loggerProvider := sdklog.NewLoggerProvider()
	logger := newOtelLogger(
		observability.LogLevelInfo,
		observability.LogFormatJSON,
		"bench-service",
		"benchmark",
		loggerProvider,
		false,
		false,
	)
	ctx := contextWithBenchTraceAndCorrelation(b)
	fields := []observability.Field{
		observability.String("operation", "checkout"),
		observability.Int("attempt", 2),
		observability.Bool("cached", false),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info(ctx, "bench message", fields...)
	}
}

func BenchmarkPropagationExtract(b *testing.B) {
	runtime := newBenchPropagationRuntime()
	carrier := propagation.MapCarrier{
		"traceparent":    "00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01",
		"baggage":        "tenant=acme",
		"x-request-id":   "req-123",
		"correlation-id": "corr-456",
	}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		extractedCtx, correlation := runtime.Extract(ctx, carrier)
		_, _ = extractedCtx, correlation
	}
}

func BenchmarkPropagationInject(b *testing.B) {
	runtime := newBenchPropagationRuntime()
	ctx := contextWithBenchTraceAndCorrelation(b)
	member, err := baggage.NewMember("tenant", "acme")
	if err != nil {
		b.Fatalf("new baggage member: %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		b.Fatalf("new baggage: %v", err)
	}
	ctx = baggage.ContextWithBaggage(ctx, bag)
	carrier := propagation.MapCarrier{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := runtime.Inject(ctx, carrier); err != nil {
			b.Fatalf("inject propagation: %v", err)
		}
	}
}

func newBenchPropagationRuntime() *propagationRuntime {
	return newPropagationRuntime(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		observability.DefaultPropagationHeaders(),
	)
}

func contextWithBenchTraceAndCorrelation(b *testing.B) context.Context {
	b.Helper()

	traceID := oteltrace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := oteltrace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	ctx := oteltrace.ContextWithSpanContext(context.Background(), oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	}))
	return ContextWithCorrelation(ctx, CorrelationContext{
		RequestID:     "req-123",
		CorrelationID: "corr-456",
	})
}
