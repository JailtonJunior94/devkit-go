package otel

import (
	"context"
	"log/slog"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/metric"
)

type otelMetrics struct {
	meter     metric.Meter
	namespace string
	validator *observability.CardinalityValidator
	onError   func(string, error)
}

func newOtelMetrics(
	meter metric.Meter,
	namespace string,
	validator *observability.CardinalityValidator,
	onError func(string, error),
) *otelMetrics {
	if onError == nil {
		onError = func(op string, err error) {
			slog.Default().Error("observability metrics error", "operation", op, "error", err)
		}
	}
	return &otelMetrics{
		meter:     meter,
		namespace: namespace,
		validator: validator,
		onError:   onError,
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
		m.onError("metrics.Counter", err)
		return &noopCounter{}
	}
	return &otelCounter{counter: counter, validator: m.validator, onError: m.onError}
}

func (m *otelMetrics) Histogram(name, description, unit string) observability.Histogram {
	fullName := m.addNamespace(name)
	histogram, err := m.meter.Float64Histogram(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		m.onError("metrics.Histogram", err)
		return &noopHistogram{}
	}
	return &otelHistogram{histogram: histogram, validator: m.validator, onError: m.onError}
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
		m.onError("metrics.HistogramWithBuckets", err)
		return &noopHistogram{}
	}
	return &otelHistogram{histogram: histogram, validator: m.validator, onError: m.onError}
}

func (m *otelMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	fullName := m.addNamespace(name)
	upDown, err := m.meter.Int64UpDownCounter(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		m.onError("metrics.UpDownCounter", err)
		return &noopUpDownCounter{}
	}
	return &otelUpDownCounter{counter: upDown, validator: m.validator, onError: m.onError}
}

func (m *otelMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	fullName := m.addNamespace(name)
	_, err := m.meter.Float64ObservableGauge(
		fullName,
		metric.WithDescription(description),
		metric.WithUnit(unit),
		metric.WithFloat64Callback(func(ctx context.Context, observer metric.Float64Observer) error {
			observer.Observe(callback(ctx))
			return nil
		}),
	)
	return err
}

type otelCounter struct {
	counter   metric.Int64Counter
	validator *observability.CardinalityValidator
	onError   func(string, error)
}

func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if c.validator != nil {
		if err := c.validator.Validate(fields); err != nil {
			c.onError("counter.Add", err)
			return
		}
	}
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
	validator *observability.CardinalityValidator
	onError   func(string, error)
}

func (h *otelHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	if h.validator != nil {
		if err := h.validator.Validate(fields); err != nil {
			h.onError("histogram.Record", err)
			return
		}
	}
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
	counter   metric.Int64UpDownCounter
	validator *observability.CardinalityValidator
	onError   func(string, error)
}

func (u *otelUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if u.validator != nil {
		if err := u.validator.Validate(fields); err != nil {
			u.onError("updown.Add", err)
			return
		}
	}
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

func (c *noopCounter) Add(_ context.Context, _ int64, _ ...observability.Field) {}

func (c *noopCounter) Increment(_ context.Context, _ ...observability.Field) {}

type noopHistogram struct{}

func (h *noopHistogram) Record(_ context.Context, _ float64, _ ...observability.Field) {}

type noopUpDownCounter struct{}

func (u *noopUpDownCounter) Add(_ context.Context, _ int64, _ ...observability.Field) {}
