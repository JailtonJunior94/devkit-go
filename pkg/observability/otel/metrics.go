package otel

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/metric"
)

// otelMetrics implements observability.Metrics using OpenTelemetry.
type otelMetrics struct {
	meter                metric.Meter
	namespace            string
	cardinalityValidator *observability.CardinalityValidator
}

// newOtelMetrics creates a new OpenTelemetry metrics recorder.
func newOtelMetrics(meter metric.Meter, namespace string, validator *observability.CardinalityValidator) *otelMetrics {
	return &otelMetrics{
		meter:                meter,
		namespace:            namespace,
		cardinalityValidator: validator,
	}
}

// addNamespace adds the namespace prefix to the metric name if configured.
func (m *otelMetrics) addNamespace(name string) string {
	if m.namespace == "" {
		return name
	}
	return m.namespace + "." + name
}

// Counter creates or returns a counter metric.
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

	return &otelCounter{
		counter:   counter,
		validator: m.cardinalityValidator,
	}
}

// Histogram creates or returns a histogram metric.
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

	return &otelHistogram{
		histogram: histogram,
		validator: m.cardinalityValidator,
	}
}

// HistogramWithBuckets creates or returns a histogram metric with custom bucket boundaries.
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

	return &otelHistogram{
		histogram: histogram,
		validator: m.cardinalityValidator,
	}
}

// UpDownCounter creates or returns an up-down counter metric.
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

	return &otelUpDownCounter{
		counter:   upDown,
		validator: m.cardinalityValidator,
	}
}

// Gauge creates an asynchronous gauge metric.
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

// otelCounter implements observability.Counter.
type otelCounter struct {
	counter   metric.Int64Counter
	validator *observability.CardinalityValidator
}

// Add increments the counter.
func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	// Validate cardinality if validator is enabled
	if c.validator != nil {
		if err := c.validator.Validate(fields); err != nil {
			// Log validation error but don't fail the operation
			// This prevents metrics from breaking the application
			return
		}
	}

	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		c.counter.Add(ctx, value)
		return
	}

	c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

// Increment increments the counter by 1.
func (c *otelCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

// otelHistogram implements observability.Histogram.
type otelHistogram struct {
	histogram metric.Float64Histogram
	validator *observability.CardinalityValidator
}

// Record adds a value to the histogram.
func (h *otelHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	// Validate cardinality if validator is enabled
	if h.validator != nil {
		if err := h.validator.Validate(fields); err != nil {
			// Log validation error but don't fail the operation
			return
		}
	}

	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		h.histogram.Record(ctx, value)
		return
	}

	h.histogram.Record(ctx, value, metric.WithAttributes(attrs...))
}

// otelUpDownCounter implements observability.UpDownCounter.
type otelUpDownCounter struct {
	counter   metric.Int64UpDownCounter
	validator *observability.CardinalityValidator
}

// Add adds a value to the up-down counter.
func (u *otelUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	// Validate cardinality if validator is enabled
	if u.validator != nil {
		if err := u.validator.Validate(fields); err != nil {
			// Log validation error but don't fail the operation
			return
		}
	}

	attrs := convertFieldsToAttributes(fields)
	if attrs == nil {
		u.counter.Add(ctx, value)
		return
	}

	u.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}


// No-op implementations for error cases.
type noopCounter struct{}

func (c *noopCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}

func (c *noopCounter) Increment(ctx context.Context, fields ...observability.Field) {}

type noopHistogram struct{}

func (h *noopHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {}

type noopUpDownCounter struct{}

func (u *noopUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}
