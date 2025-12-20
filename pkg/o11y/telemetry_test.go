package o11y

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTracer is a test tracer
type mockTracer struct{}

func (m mockTracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (m mockTracer) WithAttributes(ctx context.Context, attrs ...Attribute) {}

// mockMetrics is a test metrics
type mockMetrics struct{}

func (m mockMetrics) AddCounter(ctx context.Context, name string, v int64, labels ...any)       {}
func (m mockMetrics) RecordHistogram(ctx context.Context, name string, v float64, labels ...any) {}

// mockLogger is a test logger
type mockLogger struct{}

func (m mockLogger) Info(ctx context.Context, msg string, fields ...Field)            {}
func (m mockLogger) Debug(ctx context.Context, msg string, fields ...Field)           {}
func (m mockLogger) Warn(ctx context.Context, msg string, fields ...Field)            {}
func (m mockLogger) Error(ctx context.Context, err error, msg string, fields ...Field) {}

func createTestTelemetry(t *testing.T) Telemetry {
	shutdownCalled := false
	shutdownFunc := func(ctx context.Context) error {
		shutdownCalled = true
		_ = shutdownCalled // suppress unused warning
		return nil
	}

	tel, err := NewTelemetry(
		mockTracer{},
		mockMetrics{},
		mockLogger{},
		shutdownFunc,
		shutdownFunc,
		shutdownFunc,
	)
	if err != nil {
		t.Fatalf("failed to create telemetry: %v", err)
	}
	return tel
}

func TestTelemetry_ShutdownIdempotent(t *testing.T) {
	tel := createTestTelemetry(t)

	// First shutdown should succeed
	err := tel.Shutdown(context.Background())
	if err != nil {
		t.Errorf("first shutdown failed: %v", err)
	}

	// Second shutdown should also succeed (idempotent)
	err = tel.Shutdown(context.Background())
	if err != nil {
		t.Errorf("second shutdown failed: %v", err)
	}
}

func TestTelemetry_ShutdownWithEmptyContext(t *testing.T) {
	tel := createTestTelemetry(t)

	// Shutdown with background context should work fine
	err := tel.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown with background context failed: %v", err)
	}
}

func TestTelemetry_ShutdownWithCancelledContext(t *testing.T) {
	tel := createTestTelemetry(t)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Shutdown with cancelled context should still work
	err := tel.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown with cancelled context failed: %v", err)
	}
}

func TestTelemetry_AccessAfterShutdown(t *testing.T) {
	tel := createTestTelemetry(t)

	// Shutdown
	err := tel.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Access after shutdown should return noop implementations
	tracer := tel.Tracer()
	if tracer == nil {
		t.Error("Tracer() returned nil after shutdown")
	}
	if _, ok := tracer.(noopTracer); !ok {
		t.Error("Tracer() should return noopTracer after shutdown")
	}

	metrics := tel.Metrics()
	if metrics == nil {
		t.Error("Metrics() returned nil after shutdown")
	}
	if _, ok := metrics.(noopMetrics); !ok {
		t.Error("Metrics() should return noopMetrics after shutdown")
	}

	logger := tel.Logger()
	if logger == nil {
		t.Error("Logger() returned nil after shutdown")
	}
	if _, ok := logger.(noopLogger); !ok {
		t.Error("Logger() should return noopLogger after shutdown")
	}
}

func TestTelemetry_IsClosed(t *testing.T) {
	tel := createTestTelemetry(t)

	if tel.IsClosed() {
		t.Error("IsClosed() should return false before shutdown")
	}

	err := tel.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if !tel.IsClosed() {
		t.Error("IsClosed() should return true after shutdown")
	}
}

func TestTelemetry_ConcurrentAccess(t *testing.T) {
	tel := createTestTelemetry(t)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Start many goroutines accessing telemetry
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = tel.Tracer()
				_ = tel.Metrics()
				_ = tel.Logger()
				_ = tel.IsClosed()
			}
		}()
	}

	// Shutdown while goroutines are accessing
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		_ = tel.Shutdown(context.Background())
	}()

	wg.Wait()
	// Test passes if no race condition or panic occurred
}

func TestTelemetry_ConcurrentShutdown(t *testing.T) {
	tel := createTestTelemetry(t)

	var wg sync.WaitGroup
	var shutdownCount atomic.Int32
	numGoroutines := 10

	// Multiple goroutines trying to shutdown simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tel.Shutdown(context.Background())
			if err == nil {
				shutdownCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All shutdowns should succeed (idempotent)
	if shutdownCount.Load() != int32(numGoroutines) {
		t.Errorf("expected all %d shutdowns to succeed, got %d", numGoroutines, shutdownCount.Load())
	}
}

func TestNewTelemetry_NilValidation(t *testing.T) {
	tests := []struct {
		name           string
		tracer         Tracer
		metrics        Metrics
		logger         Logger
		tracerShutdown func(context.Context) error
		metricsShutdown func(context.Context) error
		loggerShutdown func(context.Context) error
		wantErr        string
	}{
		{
			name:    "nil tracer",
			tracer:  nil,
			metrics: mockMetrics{},
			logger:  mockLogger{},
			tracerShutdown: func(ctx context.Context) error { return nil },
			metricsShutdown: func(ctx context.Context) error { return nil },
			loggerShutdown: func(ctx context.Context) error { return nil },
			wantErr: "tracer cannot be nil",
		},
		{
			name:    "nil metrics",
			tracer:  mockTracer{},
			metrics: nil,
			logger:  mockLogger{},
			tracerShutdown: func(ctx context.Context) error { return nil },
			metricsShutdown: func(ctx context.Context) error { return nil },
			loggerShutdown: func(ctx context.Context) error { return nil },
			wantErr: "metrics cannot be nil",
		},
		{
			name:    "nil logger",
			tracer:  mockTracer{},
			metrics: mockMetrics{},
			logger:  nil,
			tracerShutdown: func(ctx context.Context) error { return nil },
			metricsShutdown: func(ctx context.Context) error { return nil },
			loggerShutdown: func(ctx context.Context) error { return nil },
			wantErr: "logger cannot be nil",
		},
		{
			name:    "nil tracerShutdown",
			tracer:  mockTracer{},
			metrics: mockMetrics{},
			logger:  mockLogger{},
			tracerShutdown: nil,
			metricsShutdown: func(ctx context.Context) error { return nil },
			loggerShutdown: func(ctx context.Context) error { return nil },
			wantErr: "tracerShutdown function cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTelemetry(tt.tracer, tt.metrics, tt.logger, tt.tracerShutdown, tt.metricsShutdown, tt.loggerShutdown)
			if err == nil {
				t.Error("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
