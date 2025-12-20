package o11y

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	defaultShutdownTimeout = 15 * time.Second
)

type shutdownFunc struct {
	name string
	fn   func(context.Context) error
}

type telemetry struct {
	mu        sync.RWMutex
	tracer    Tracer
	metrics   Metrics
	logger    Logger
	shutdowns []shutdownFunc
	closed    bool
}

func NewTelemetry(tracer Tracer, metrics Metrics, logger Logger, tracerShutdown, metricsShutdown, loggerShutdown func(context.Context) error) (Telemetry, error) {
	if tracer == nil {
		return nil, fmt.Errorf("tracer cannot be nil")
	}
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if tracerShutdown == nil {
		return nil, fmt.Errorf("tracerShutdown function cannot be nil")
	}
	if metricsShutdown == nil {
		return nil, fmt.Errorf("metricsShutdown function cannot be nil")
	}
	if loggerShutdown == nil {
		return nil, fmt.Errorf("loggerShutdown function cannot be nil")
	}

	return &telemetry{
		tracer:  tracer,
		metrics: metrics,
		logger:  logger,
		shutdowns: []shutdownFunc{
			{name: "tracer", fn: tracerShutdown},
			{name: "metrics", fn: metricsShutdown},
			{name: "logger", fn: loggerShutdown},
		},
	}, nil
}

func (t *telemetry) Tracer() Tracer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return noopTracer{}
	}
	return t.tracer
}

func (t *telemetry) Metrics() Metrics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return noopMetrics{}
	}
	return t.metrics
}

func (t *telemetry) Logger() Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return noopLogger{}
	}
	return t.logger
}

func (t *telemetry) Shutdown(ctx context.Context) error {
	// Acquire write lock to prevent concurrent access during shutdown
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()

	ctx, cancel := t.ensureValidContext(ctx)
	defer cancel()

	var errs []error

	// Shutdown in reverse dependency order: logger, metrics, tracer
	// Logger needs tracer for trace_id correlation
	// We shutdown logger first to ensure all final logs are captured
	for i := len(t.shutdowns) - 1; i >= 0; i-- {
		if err := t.shutdowns[i].fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.shutdowns[i].name, err))
			// Continue with other shutdowns even if one fails
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (t *telemetry) ensureValidContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	// If context is already cancelled, create a new one
	if ctx.Err() != nil {
		return context.WithTimeout(context.Background(), defaultShutdownTimeout)
	}

	// If context has no deadline, add one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		return context.WithTimeout(ctx, defaultShutdownTimeout)
	}

	// Context is valid, return a no-op cancel
	return ctx, func() {}
}

// IsClosed returns true if telemetry has been shut down
func (t *telemetry) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// noopTracer is a no-operation tracer for use after shutdown
type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (noopTracer) WithAttributes(ctx context.Context, attrs ...Attribute) {}

// noopSpan is a no-operation span for use after shutdown
type noopSpan struct{}

func (noopSpan) End()                                     {}
func (noopSpan) SetAttributes(attrs ...Attribute)         {}
func (noopSpan) AddEvent(name string, attrs ...Attribute) {}
func (noopSpan) SetStatus(status SpanStatus, msg string)  {}

// noopMetrics is a no-operation metrics for use after shutdown
type noopMetrics struct{}

func (noopMetrics) AddCounter(ctx context.Context, name string, v int64, labels ...any)       {}
func (noopMetrics) RecordHistogram(ctx context.Context, name string, v float64, labels ...any) {}

// noopLogger is a no-operation logger for use after shutdown
type noopLogger struct{}

func (noopLogger) Info(ctx context.Context, msg string, fields ...Field)            {}
func (noopLogger) Debug(ctx context.Context, msg string, fields ...Field)           {}
func (noopLogger) Warn(ctx context.Context, msg string, fields ...Field)            {}
func (noopLogger) Error(ctx context.Context, err error, msg string, fields ...Field) {}
