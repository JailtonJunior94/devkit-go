package o11y

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewTelemetry_ValidComponents(t *testing.T) {
	tracer := NewNoOpTracer()
	metrics := NewNoOpMetrics()
	logger := NewNoOpLogger()

	tel, err := NewTelemetry(tracer, metrics, logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if tel.Tracer() != tracer {
		t.Error("tracer mismatch")
	}
	if tel.Metrics() != metrics {
		t.Error("metrics mismatch")
	}
	if tel.Logger() != logger {
		t.Error("logger mismatch")
	}
}

func TestNewTelemetry_NilTracer(t *testing.T) {
	metrics := NewNoOpMetrics()
	logger := NewNoOpLogger()

	_, err := NewTelemetry(nil, metrics, logger, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil tracer")
	}
	if err.Error() != "tracer cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewTelemetry_NilMetrics(t *testing.T) {
	tracer := NewNoOpTracer()
	logger := NewNoOpLogger()

	_, err := NewTelemetry(tracer, nil, logger, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil metrics")
	}
	if err.Error() != "metrics cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewTelemetry_NilLogger(t *testing.T) {
	tracer := NewNoOpTracer()
	metrics := NewNoOpMetrics()

	_, err := NewTelemetry(tracer, metrics, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil logger")
	}
	if err.Error() != "logger cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTelemetry_Shutdown(t *testing.T) {
	var tracerCalled, metricsCalled, loggerCalled bool

	tracer := NewNoOpTracer()
	metrics := NewNoOpMetrics()
	logger := NewNoOpLogger()

	tel, err := NewTelemetry(
		tracer, metrics, logger,
		func(ctx context.Context) error {
			tracerCalled = true
			return nil
		},
		func(ctx context.Context) error {
			metricsCalled = true
			return nil
		},
		func(ctx context.Context) error {
			loggerCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = tel.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	if !tracerCalled {
		t.Error("tracer shutdown not called")
	}
	if !metricsCalled {
		t.Error("metrics shutdown not called")
	}
	if !loggerCalled {
		t.Error("logger shutdown not called")
	}
}

func TestTelemetry_ShutdownWithErrors(t *testing.T) {
	tracer := NewNoOpTracer()
	metrics := NewNoOpMetrics()
	logger := NewNoOpLogger()

	expectedErr1 := errors.New("tracer error")
	expectedErr2 := errors.New("metrics error")

	tel, err := NewTelemetry(
		tracer, metrics, logger,
		func(ctx context.Context) error { return expectedErr1 },
		func(ctx context.Context) error { return expectedErr2 },
		func(ctx context.Context) error { return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = tel.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected error from shutdown")
	}

	if !errors.Is(err, expectedErr1) {
		t.Errorf("expected error to contain %v", expectedErr1)
	}
	if !errors.Is(err, expectedErr2) {
		t.Errorf("expected error to contain %v", expectedErr2)
	}
}

func TestTelemetry_ShutdownWithTimeout(t *testing.T) {
	tracer := NewNoOpTracer()
	metrics := NewNoOpMetrics()
	logger := NewNoOpLogger()

	tel, err := NewTelemetry(tracer, metrics, logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = tel.ShutdownWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}
