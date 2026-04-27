package otel

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/metric"
)

type otelMetrics struct {
	meter     metric.Meter
	namespace string
}

func newOtelMetrics(meter metric.Meter, namespace string) *otelMetrics {
	return &otelMetrics{
		meter:     meter,
		namespace: namespace,
	}
}

func (m *otelMetrics) addNamespace(name string) string {
	if m.namespace == "" {
		return name
	}
	return m.namespace + "." + name
}

func (m *otelMetrics) Counter(name, description, unit string) observability.Counter {
	fullName := m.addNamespace(name)
	counter, err := m.meter.Int64Counter(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return &noopCounter{}
	}
	return &otelCounter{counter: counter}
}

func (m *otelMetrics) Histogram(name, description, unit string) observability.Histogram {
	fullName := m.addNamespace(name)
	histogram, err := m.meter.Float64Histogram(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return &noopHistogram{}
	}
	return &otelHistogram{histogram: histogram}
}

func (m *otelMetrics) HistogramWithBuckets(name, description, unit string, buckets []float64) observability.Histogram {
	fullName := m.addNamespace(name)
	histogram, err := m.meter.Float64Histogram(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
	if err != nil {
		return &noopHistogram{}
	}
	return &otelHistogram{histogram: histogram}
}

func (m *otelMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	fullName := m.addNamespace(name)
	upDown, err := m.meter.Int64UpDownCounter(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return &noopUpDownCounter{}
	}
	return &otelUpDownCounter{counter: upDown}
}

func (m *otelMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	fullName := m.addNamespace(name)
	_, err := m.meter.Float64ObservableGauge(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
		metric.WithFloat64Callback(func(ctx context.Context, observer metric.Float64Observer) error {
			value := callback(ctx)
			observer.Observe(value)
			return nil
		}),
	)
	return err
}

// otelCounter: CardinalityValidator foi removido do caminho de gravação.
// Validação de labels deve ser feita no call site ou na criação do instrumento.
type otelCounter struct {
	counter metric.Int64Counter
}

func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if len(fields) == 0 {
		c.counter.Add(ctx, value)
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
	*p = attrs
	releaseAttrs(p)
}

func (c *otelCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

type otelHistogram struct {
	histogram metric.Float64Histogram
}

func (h *otelHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	if len(fields) == 0 {
		h.histogram.Record(ctx, value)
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	h.histogram.Record(ctx, value, metric.WithAttributes(attrs...))
	*p = attrs
	releaseAttrs(p)
}

type otelUpDownCounter struct {
	counter metric.Int64UpDownCounter
}

func (u *otelUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if len(fields) == 0 {
		u.counter.Add(ctx, value)
		return
	}
	p := acquireAttrs()
	attrs := appendFieldAttrs((*p)[:0], fields)
	u.counter.Add(ctx, value, metric.WithAttributes(attrs...))
	*p = attrs
	releaseAttrs(p)
}

type noopCounter struct{}

func (c *noopCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}

func (c *noopCounter) Increment(ctx context.Context, fields ...observability.Field) {}

type noopHistogram struct{}

func (h *noopHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {}

type noopUpDownCounter struct{}

func (u *noopUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}
