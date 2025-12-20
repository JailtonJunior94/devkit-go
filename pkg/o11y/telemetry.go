package o11y

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
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
	tracer    Tracer
	metrics   Metrics
	logger    Logger
	shutdowns []shutdownFunc
	closed    atomic.Bool
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
	if t.closed.Load() {
		return noopTracer{}
	}
	return t.tracer
}

func (t *telemetry) Metrics() Metrics {
	if t.closed.Load() {
		return noopMetrics{}
	}
	return t.metrics
}

func (t *telemetry) Logger() Logger {
	if t.closed.Load() {
		return noopLogger{}
	}
	return t.logger
}

func (t *telemetry) Shutdown(ctx context.Context) error {
	// Prevent double shutdown
	if t.closed.Swap(true) {
		return nil
	}

	ctx, cancel := t.ensureValidContext(ctx)
	defer cancel()

	var errs []error

	// Shutdown em ordem reversa de dependência: logger, metrics, tracer
	// Logger precisa do tracer para correlação de trace_id
	// Fazemos logger primeiro para garantir que todos os logs finais sejam capturados
	for i := len(t.shutdowns) - 1; i >= 0; i-- {
		if err := t.shutdowns[i].fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.shutdowns[i].name, err))
			// Continuar com os outros shutdowns mesmo se um falhar
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
	return t.closed.Load()
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
