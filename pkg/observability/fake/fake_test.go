package fake_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

func TestFakeProvider(t *testing.T) {
	provider := fake.NewProvider()

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

func TestFakeTracer(t *testing.T) {
	provider := fake.NewProvider()
	tracer := provider.Tracer().(*fake.FakeTracer)

	t.Run("captures spans", func(t *testing.T) {
		tracer.Reset()
		ctx := context.Background()

		_, span := tracer.Start(ctx, "test-span",
			observability.WithSpanKind(observability.SpanKindServer),
			observability.WithAttributes(
				observability.String("key", "value"),
			),
		)

		span.SetAttributes(observability.Int("count", 42))
		span.AddEvent("test-event", observability.Bool("success", true))
		span.SetStatus(observability.StatusCodeOK, "completed")
		span.End()

		spans := tracer.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		capturedSpan := spans[0]
		if capturedSpan.Name != "test-span" {
			t.Errorf("expected span name 'test-span', got %s", capturedSpan.Name)
		}

		if capturedSpan.EndTime == nil {
			t.Error("span should be ended")
		}

		if len(capturedSpan.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(capturedSpan.Events))
		}
	})

	t.Run("captures errors", func(t *testing.T) {
		tracer.Reset()
		ctx := context.Background()

		_, span := tracer.Start(ctx, "error-span")
		testErr := errors.New("test error")
		span.RecordError(testErr, observability.String("error_code", "TEST_ERROR"))
		span.End()

		spans := tracer.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}

		if spans[0].RecordedErr == nil {
			t.Error("expected error to be recorded")
		}

		if spans[0].RecordedErr.Error() != testErr.Error() {
			t.Errorf("expected error %v, got %v", testErr, spans[0].RecordedErr)
		}
	})
}

func TestFakeLogger(t *testing.T) {
	provider := fake.NewProvider()
	logger := provider.Logger().(*fake.FakeLogger)

	t.Run("captures all log levels", func(t *testing.T) {
		logger.Reset()
		ctx := context.Background()

		logger.Debug(ctx, "debug message", observability.String("level", "debug"))
		logger.Info(ctx, "info message", observability.String("level", "info"))
		logger.Warn(ctx, "warn message", observability.String("level", "warn"))
		logger.Error(ctx, "error message", observability.String("level", "error"))

		entries := logger.GetEntries()
		if len(entries) != 4 {
			t.Fatalf("expected 4 log entries, got %d", len(entries))
		}

		expectedLevels := []observability.LogLevel{
			observability.LogLevelDebug,
			observability.LogLevelInfo,
			observability.LogLevelWarn,
			observability.LogLevelError,
		}

		for i, entry := range entries {
			if entry.Level != expectedLevels[i] {
				t.Errorf("entry %d: expected level %s, got %s", i, expectedLevels[i], entry.Level)
			}
		}
	})

	t.Run("child logger includes parent fields", func(t *testing.T) {
		logger.Reset()
		ctx := context.Background()

		childLogger := logger.With(observability.String("service", "test"))
		childLogger.Info(ctx, "test message", observability.String("request_id", "123"))

		entries := logger.GetEntries()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}

		entry := entries[0]
		if len(entry.Fields) < 2 {
			t.Errorf("expected at least 2 fields, got %d", len(entry.Fields))
		}
	})
}

func TestFakeMetrics(t *testing.T) {
	provider := fake.NewProvider()
	metrics := provider.Metrics().(*fake.FakeMetrics)

	t.Run("counter increments", func(t *testing.T) {
		ctx := context.Background()
		counter := metrics.Counter("test.counter", "A test counter", "1")

		counter.Add(ctx, 1, observability.String("method", "GET"))
		counter.Add(ctx, 5, observability.String("method", "POST"))

		fakeCounter := metrics.GetCounter("test.counter")
		if fakeCounter == nil {
			t.Fatal("counter should exist")
		}

		values := fakeCounter.GetValues()
		if len(values) != 2 {
			t.Fatalf("expected 2 values, got %d", len(values))
		}

		if values[0].Value != 1 {
			t.Errorf("expected first value to be 1, got %d", values[0].Value)
		}

		if values[1].Value != 5 {
			t.Errorf("expected second value to be 5, got %d", values[1].Value)
		}
	})

	t.Run("histogram records values", func(t *testing.T) {
		ctx := context.Background()
		histogram := metrics.Histogram("test.histogram", "A test histogram", "ms")

		histogram.Record(ctx, 100.5, observability.String("endpoint", "/api"))
		histogram.Record(ctx, 250.3, observability.String("endpoint", "/api"))

		fakeHistogram := metrics.GetHistogram("test.histogram")
		if fakeHistogram == nil {
			t.Fatal("histogram should exist")
		}

		values := fakeHistogram.GetValues()
		if len(values) != 2 {
			t.Fatalf("expected 2 values, got %d", len(values))
		}

		if values[0].Value != 100.5 {
			t.Errorf("expected first value to be 100.5, got %f", values[0].Value)
		}
	})

	t.Run("up-down counter", func(t *testing.T) {
		ctx := context.Background()
		upDown := metrics.UpDownCounter("test.updown", "A test up-down counter", "1")

		upDown.Add(ctx, 10)
		upDown.Add(ctx, -5)
		upDown.Add(ctx, 3)

		fakeUpDown := metrics.GetUpDownCounter("test.updown")
		if fakeUpDown == nil {
			t.Fatal("up-down counter should exist")
		}

		values := fakeUpDown.GetValues()
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}

		expectedValues := []int64{10, -5, 3}
		for i, val := range values {
			if val.Value != expectedValues[i] {
				t.Errorf("value %d: expected %d, got %d", i, expectedValues[i], val.Value)
			}
		}
	})
}
