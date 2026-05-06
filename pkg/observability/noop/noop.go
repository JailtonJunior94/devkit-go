package noop

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

var (
	_ observability.Observability = (*Provider)(nil)
	_ observability.Tracer        = (*noopTracer)(nil)
	_ observability.Span          = noopSpan{}
	_ observability.SpanContext   = noopSpanContext{}
	_ observability.Logger        = (*noopLogger)(nil)
	_ observability.Metrics       = (*noopMetrics)(nil)
	_ observability.Counter       = noopCounter{}
	_ observability.Histogram     = noopHistogram{}
	_ observability.UpDownCounter = noopUpDownCounter{}
)

// Provider é uma implementação noop de observabilidade com overhead zero.
// Implementa noopMarker para que detectores internos possam identificá-la via
// type assertion para interface em vez de tipo concreto.
type Provider struct {
	tracer  *noopTracer
	logger  *noopLogger
	metrics *noopMetrics
}

// IsNoop satisfaz a interface noopMarker consumida por pkg/database/manager.
// Permite que wrappers ou decorators sobre Provider também se declarem noop
// implementando o mesmo método, sem precisar ser do tipo concreto *Provider.
func (p *Provider) IsNoop() bool { return true }

func NewProvider() *Provider {
	return &Provider{
		tracer:  &noopTracer{},
		logger:  &noopLogger{},
		metrics: &noopMetrics{},
	}
}

func (p *Provider) Tracer() observability.Tracer   { return p.tracer }
func (p *Provider) Logger() observability.Logger   { return p.logger }
func (p *Provider) Metrics() observability.Metrics { return p.metrics }

func (p *Provider) Shutdown(_ context.Context) error { return nil }

var (
	globalNoopSpan        = noopSpan{}
	globalNoopSpanContext = noopSpanContext{}
	globalNoopCounter     = noopCounter{}
	globalNoopHistogram   = noopHistogram{}
	globalNoopUpDown      = noopUpDownCounter{}
)

type noopTracer struct{}

func (t *noopTracer) Start(ctx context.Context, spanName string, opts ...observability.SpanOption) (context.Context, observability.Span) {
	return ctx, globalNoopSpan
}

func (t *noopTracer) SpanFromContext(ctx context.Context) observability.Span {
	return globalNoopSpan
}

func (t *noopTracer) ContextWithSpan(ctx context.Context, span observability.Span) context.Context {
	return ctx
}

type noopSpan struct{}

func (s noopSpan) End() {}

func (s noopSpan) SetAttributes(fields ...observability.Field) {}

func (s noopSpan) SetStatus(code observability.StatusCode, description string) {}

func (s noopSpan) RecordError(err error, fields ...observability.Field) {}

func (s noopSpan) AddEvent(name string, fields ...observability.Field) {}

func (s noopSpan) Context() observability.SpanContext {
	return globalNoopSpanContext
}

func (s noopSpan) TraceID() string { return "" }

func (s noopSpan) SpanID() string { return "" }

func (s noopSpan) IsSampled() bool { return false }

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

type noopLogger struct{}

func (l *noopLogger) Debug(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Info(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Warn(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) Error(ctx context.Context, msg string, fields ...observability.Field) {}

func (l *noopLogger) With(fields ...observability.Field) observability.Logger {
	return l
}

type noopMetrics struct{}

func (m *noopMetrics) Counter(name, description, unit string) observability.Counter {
	return globalNoopCounter
}

func (m *noopMetrics) Histogram(name, description, unit string) observability.Histogram {
	return globalNoopHistogram
}

func (m *noopMetrics) HistogramWithBuckets(name, description, unit string, buckets []float64) observability.Histogram {
	return globalNoopHistogram
}

func (m *noopMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	return globalNoopUpDown
}

func (m *noopMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	return nil
}

type noopCounter struct{}

func (c noopCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}

func (c noopCounter) Increment(ctx context.Context, fields ...observability.Field) {}

type noopHistogram struct{}

func (h noopHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {}

type noopUpDownCounter struct{}

func (u noopUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}
