package noop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
)

func TestNoopProvider(t *testing.T) {
	provider := noop.NewProvider()

	if provider.Tracer() == nil {
		t.Error("Tracer() should not return nil")
	}

	if provider.Logger() == nil {
		t.Error("Logger() should not return nil")
	}

	if provider.Metrics() == nil {
		t.Error("Metrics() should not return nil")
	}
}

func TestNoopTracer(t *testing.T) {
	provider := noop.NewProvider()
	tracer := provider.Tracer()
	ctx := context.Background()

	t.Run("Start returns valid context and span", func(t *testing.T) {
		newCtx, span := tracer.Start(ctx, "test-span")

		if newCtx == nil {
			t.Error("Start should return non-nil context")
		}

		if span == nil {
			t.Error("Start should return non-nil span")
		}

		// Should not panic
		span.SetAttributes(observability.String("key", "value"))
		span.AddEvent("event")
		span.RecordError(errors.New("test error"))
		span.SetStatus(observability.StatusCodeError, "error")
		span.End()
	})

	t.Run("SpanFromContext returns valid span", func(t *testing.T) {
		span := tracer.SpanFromContext(ctx)
		if span == nil {
			t.Error("SpanFromContext should return non-nil span")
		}

		// Should not panic
		span.End()
	})

	t.Run("span context returns empty values", func(t *testing.T) {
		_, span := tracer.Start(ctx, "test")
		spanCtx := span.Context()

		if spanCtx.TraceID() != "" {
			t.Errorf("expected empty trace ID, got %s", spanCtx.TraceID())
		}

		if spanCtx.SpanID() != "" {
			t.Errorf("expected empty span ID, got %s", spanCtx.SpanID())
		}

		if spanCtx.IsSampled() {
			t.Error("expected IsSampled to be false")
		}
	})
}

func TestNoopLogger(t *testing.T) {
	provider := noop.NewProvider()
	logger := provider.Logger()
	ctx := context.Background()

	t.Run("all log methods should not panic", func(t *testing.T) {
		// None of these should panic
		logger.Debug(ctx, "debug message", observability.String("key", "value"))
		logger.Info(ctx, "info message", observability.Int("count", 42))
		logger.Warn(ctx, "warn message", observability.Bool("flag", true))
		logger.Error(ctx, "error message", observability.Error(errors.New("test")))
	})

	t.Run("With returns valid logger", func(t *testing.T) {
		childLogger := logger.With(observability.String("service", "test"))
		if childLogger == nil {
			t.Error("With should return non-nil logger")
		}

		// Should not panic
		childLogger.Info(ctx, "message")
	})
}

func TestNoopMetrics(t *testing.T) {
	provider := noop.NewProvider()
	metrics := provider.Metrics()
	ctx := context.Background()

	t.Run("Counter operations should not panic", func(t *testing.T) {
		counter := metrics.Counter("test.counter", "description", "1")
		if counter == nil {
			t.Error("Counter should return non-nil")
		}

		// Should not panic
		counter.Add(ctx, 1)
		counter.Add(ctx, 10, observability.String("label", "value"))
	})

	t.Run("Histogram operations should not panic", func(t *testing.T) {
		histogram := metrics.Histogram("test.histogram", "description", "ms")
		if histogram == nil {
			t.Error("Histogram should return non-nil")
		}

		// Should not panic
		histogram.Record(ctx, 100.5)
		histogram.Record(ctx, 250.0, observability.String("endpoint", "/api"))
	})

	t.Run("UpDownCounter operations should not panic", func(t *testing.T) {
		upDown := metrics.UpDownCounter("test.updown", "description", "1")
		if upDown == nil {
			t.Error("UpDownCounter should return non-nil")
		}

		// Should not panic
		upDown.Add(ctx, 5)
		upDown.Add(ctx, -3)
	})

	t.Run("Gauge should not return error", func(t *testing.T) {
		err := metrics.Gauge("test.gauge", "description", "1", func(ctx context.Context) float64 {
			return 42.0
		})

		if err != nil {
			t.Errorf("Gauge should not return error, got %v", err)
		}
	})
}

// Benchmark to ensure no-op has minimal overhead.
func BenchmarkNoopTracer(b *testing.B) {
	provider := noop.NewProvider()
	tracer := provider.Tracer()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "benchmark")
		span.SetAttributes(observability.String("key", "value"))
		span.End()
	}
}

func BenchmarkNoopLogger(b *testing.B) {
	provider := noop.NewProvider()
	logger := provider.Logger()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info(ctx, "benchmark message", observability.String("key", "value"))
	}
}

func BenchmarkNoopMetrics(b *testing.B) {
	provider := noop.NewProvider()
	metrics := provider.Metrics()
	counter := metrics.Counter("bench.counter", "benchmark", "1")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(ctx, 1, observability.String("label", "value"))
	}
}
