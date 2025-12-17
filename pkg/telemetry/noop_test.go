package o11y

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNoOpTelemetry(t *testing.T) {
	tel := NewNoOpTelemetry()

	if tel.Tracer() == nil {
		t.Error("expected non-nil tracer")
	}
	if tel.Metrics() == nil {
		t.Error("expected non-nil metrics")
	}
	if tel.Logger() == nil {
		t.Error("expected non-nil logger")
	}

	// Test shutdown
	if err := tel.Shutdown(context.Background()); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if err := tel.ShutdownWithTimeout(time.Second); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestNoOpLogger_AllMethods(t *testing.T) {
	logger := NewNoOpLogger()
	ctx := context.Background()

	// These should not panic
	logger.Debug(ctx, "debug message", LogField("key", "value"))
	logger.Info(ctx, "info message", LogField("key", "value"))
	logger.Warn(ctx, "warn message", LogField("key", "value"))
	logger.Error(ctx, errors.New("test"), "error message", LogField("key", "value"))

	// Test With
	newLogger := logger.With(LogField("base", "field"))
	if newLogger == nil {
		t.Error("expected non-nil logger from With")
	}
	newLogger.Info(ctx, "message with base field")
}

func TestNoOpSpan_AllMethods(t *testing.T) {
	tracer := NewNoOpTracer()
	_, span := tracer.Start(context.Background(), "test")

	// All these should not panic
	span.SetAttributes(Attr("key1", "value1"), Attr("key2", 42))
	span.AddEvent("event_name", Attr("event_key", "event_value"))
	span.SetStatus(SpanStatusOk, "success")
	span.SetStatus(SpanStatusError, "failure")
	span.SetStatus(SpanStatusUnset, "")
	span.RecordError(errors.New("test error"))
	span.RecordError(nil) // nil error should be safe
	span.End()
}
