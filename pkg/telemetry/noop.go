package o11y

import (
	"context"
	"time"
)

// NoOp implementations for testing and development environments.
// These implementations do nothing but satisfy the interfaces.

// noopTelemetry is a no-op implementation of Telemetry.
type noopTelemetry struct {
	tracer  Tracer
	metrics Metrics
	logger  Logger
}

// NewNoOpTelemetry creates a no-op telemetry instance.
// Useful for testing or when telemetry is disabled.
func NewNoOpTelemetry() Telemetry {
	return &noopTelemetry{
		tracer:  NewNoOpTracer(),
		metrics: NewNoOpMetrics(),
		logger:  NewNoOpLogger(),
	}
}

func (t *noopTelemetry) Tracer() Tracer   { return t.tracer }
func (t *noopTelemetry) Metrics() Metrics { return t.metrics }
func (t *noopTelemetry) Logger() Logger   { return t.logger }
func (t *noopTelemetry) Shutdown(_ context.Context) error {
	return nil
}
func (t *noopTelemetry) ShutdownWithTimeout(_ time.Duration) error {
	return nil
}

// noopTracer is a no-op implementation of Tracer.
type noopTracer struct{}

// NewNoOpTracer creates a no-op tracer instance.
func NewNoOpTracer() Tracer {
	return &noopTracer{}
}

func (t *noopTracer) Start(ctx context.Context, _ string, _ ...Attribute) (context.Context, Span) {
	return ctx, &noopSpan{}
}

func (t *noopTracer) WithAttributes(_ context.Context, _ ...Attribute) {}

// noopSpan is a no-op implementation of Span.
type noopSpan struct{}

func (s *noopSpan) End()                                  {}
func (s *noopSpan) SetAttributes(_ ...Attribute)          {}
func (s *noopSpan) AddEvent(_ string, _ ...Attribute)     {}
func (s *noopSpan) SetStatus(_ SpanStatus, _ string)      {}
func (s *noopSpan) RecordError(_ error)                   {}

// noopMetrics is a no-op implementation of Metrics.
type noopMetrics struct{}

// NewNoOpMetrics creates a no-op metrics instance.
func NewNoOpMetrics() Metrics {
	return &noopMetrics{}
}

func (m *noopMetrics) AddCounter(_ context.Context, _ string, _ int64, _ ...any)       {}
func (m *noopMetrics) RecordHistogram(_ context.Context, _ string, _ float64, _ ...any) {}
func (m *noopMetrics) SetGauge(_ context.Context, _ string, _ float64, _ ...any)        {}
func (m *noopMetrics) AddUpDownCounter(_ context.Context, _ string, _ int64, _ ...any)  {}
func (m *noopMetrics) RecordDuration(_ context.Context, _ string, _ time.Time, _ ...any) {}

// noopLogger is a no-op implementation of Logger.
type noopLogger struct{}

// NewNoOpLogger creates a no-op logger instance.
func NewNoOpLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Info(_ context.Context, _ string, _ ...Field)        {}
func (l *noopLogger) Debug(_ context.Context, _ string, _ ...Field)       {}
func (l *noopLogger) Warn(_ context.Context, _ string, _ ...Field)        {}
func (l *noopLogger) Error(_ context.Context, _ error, _ string, _ ...Field) {}
func (l *noopLogger) With(_ ...Field) Logger                              { return l }
