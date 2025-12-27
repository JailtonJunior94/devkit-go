package noop

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Provider returns a no-op implementation of observability that has zero runtime overhead.
// Use this when you want to disable observability completely.
type Provider struct {
	tracer  *noopTracer
	logger  *noopLogger
	metrics *noopMetrics
}

// NewProvider creates a new no-op observability provider.
func NewProvider() *Provider {
	return &Provider{
		tracer:  &noopTracer{},
		logger:  &noopLogger{},
		metrics: &noopMetrics{},
	}
}

// Tracer returns a no-op tracer.
func (p *Provider) Tracer() observability.Tracer {
	return p.tracer
}

// Logger returns a no-op logger.
func (p *Provider) Logger() observability.Logger {
	return p.logger
}

// Metrics returns a no-op metrics recorder.
func (p *Provider) Metrics() observability.Metrics {
	return p.metrics
}

// noopTracer implements observability.Tracer with no-op operations.
type noopTracer struct{}

func (t *noopTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	return ctx, noopSpan{}
}

func (t *noopTracer) SpanFromContext(ctx context.Context) observability.Span {
	return noopSpan{}
}

func (t *noopTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	return ctx
}

// noopSpan implements observability.Span with no-op operations.
type noopSpan struct{}

func (s noopSpan) End() {}

func (s noopSpan) SetAttributes(fields ...observability.Field) {}

func (s noopSpan) SetStatus(code observability.StatusCode, description string) {}

func (s noopSpan) RecordError(err error, fields ...observability.Field) {}

func (s noopSpan) AddEvent(name string, fields ...observability.Field) {}

func (s noopSpan) Context() observability.SpanContext {
	return noopSpanContext{}
}

// noopSpanContext implements observability.SpanContext with no-op operations.
type noopSpanContext struct{}

func (c noopSpanContext) TraceID() string {
	return ""
}

func (c noopSpanContext) SpanID() string {
	return ""
}

func (c noopSpanContext) IsSampled() bool {
	return false
}

// noopLogger implements observability.Logger with no-op operations.
type noopLogger struct{}

func (l *noopLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) With(fields ...observability.Field) observability.Logger {
	return l
}

// noopMetrics implements observability.Metrics with no-op operations.
type noopMetrics struct{}

func (m *noopMetrics) Counter(name, description, unit string) observability.Counter {
	return noopCounter{}
}

func (m *noopMetrics) Histogram(name, description, unit string) observability.Histogram {
	return noopHistogram{}
}

func (m *noopMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	return noopUpDownCounter{}
}

func (m *noopMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	return nil
}

// noopCounter implements observability.Counter with no-op operations.
type noopCounter struct{}

func (c noopCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}

func (c noopCounter) Increment(ctx context.Context, fields ...observability.Field) {}

// noopHistogram implements observability.Histogram with no-op operations.
type noopHistogram struct{}

func (h noopHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {}

// noopUpDownCounter implements observability.UpDownCounter with no-op operations.
type noopUpDownCounter struct{}

func (u noopUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}
