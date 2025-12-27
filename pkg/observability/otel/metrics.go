package otel

import (
	"context"
	"fmt"

	"github.com/jailtonjunior94/order/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// otelMetrics implements observability.Metrics using OpenTelemetry.
type otelMetrics struct {
	meter metric.Meter
}

// newOtelMetrics creates a new OpenTelemetry metrics recorder.
func newOtelMetrics(meter metric.Meter) *otelMetrics {
	return &otelMetrics{meter: meter}
}

// Counter creates or returns a counter metric.
func (m *otelMetrics) Counter(name, description, unit string) observability.Counter {
	counter, err := m.meter.Int64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		// In production, you might want to handle this differently
		// For now, we return a no-op counter to prevent crashes
		return &noopCounter{}
	}

	return &otelCounter{counter: counter}
}

// Histogram creates or returns a histogram metric.
func (m *otelMetrics) Histogram(name, description, unit string) observability.Histogram {
	histogram, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return &noopHistogram{}
	}

	return &otelHistogram{histogram: histogram}
}

// UpDownCounter creates or returns an up-down counter metric.
func (m *otelMetrics) UpDownCounter(name, description, unit string) observability.UpDownCounter {
	upDown, err := m.meter.Int64UpDownCounter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return &noopUpDownCounter{}
	}

	return &otelUpDownCounter{counter: upDown}
}

// Gauge creates an asynchronous gauge metric.
func (m *otelMetrics) Gauge(name, description, unit string, callback observability.GaugeCallback) error {
	_, err := m.meter.Float64ObservableGauge(
		name,
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
	counter metric.Int64Counter
}

// Add increments the counter.
func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if len(fields) == 0 {
		c.counter.Add(ctx, value)
		return
	}

	c.counter.Add(ctx, value, metric.WithAttributes(convertFieldsToOtelAttributes(fields)...))
}

// Increment increments the counter by 1.
func (c *otelCounter) Increment(ctx context.Context, fields ...observability.Field) {
	c.Add(ctx, 1, fields...)
}

// otelHistogram implements observability.Histogram.
type otelHistogram struct {
	histogram metric.Float64Histogram
}

// Record adds a value to the histogram.
func (h *otelHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {
	if len(fields) == 0 {
		h.histogram.Record(ctx, value)
		return
	}

	h.histogram.Record(ctx, value, metric.WithAttributes(convertFieldsToOtelAttributes(fields)...))
}

// otelUpDownCounter implements observability.UpDownCounter.
type otelUpDownCounter struct {
	counter metric.Int64UpDownCounter
}

// Add adds a value to the up-down counter.
func (u *otelUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
	if len(fields) == 0 {
		u.counter.Add(ctx, value)
		return
	}

	u.counter.Add(ctx, value, metric.WithAttributes(convertFieldsToOtelAttributes(fields)...))
}

// convertFieldsToOtelAttributes converts observability fields to OTel attributes.
func convertFieldsToOtelAttributes(fields []observability.Field) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))
	for _, field := range fields {
		attrs = append(attrs, convertFieldToOtelAttribute(field))
	}
	return attrs
}

// convertFieldToOtelAttribute converts a single field to an OTel attribute.
func convertFieldToOtelAttribute(field observability.Field) attribute.KeyValue {
	switch v := field.Value.(type) {
	case string:
		return attribute.String(field.Key, v)
	case int:
		return attribute.Int(field.Key, v)
	case int64:
		return attribute.Int64(field.Key, v)
	case float64:
		return attribute.Float64(field.Key, v)
	case bool:
		return attribute.Bool(field.Key, v)
	case error:
		return attribute.String(field.Key, v.Error())
	default:
		return attribute.String(field.Key, fmt.Sprintf("%v", v))
	}
}

// No-op implementations for error cases
type noopCounter struct{}

func (c *noopCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}

func (c *noopCounter) Increment(ctx context.Context, fields ...observability.Field) {}

type noopHistogram struct{}

func (h *noopHistogram) Record(ctx context.Context, value float64, fields ...observability.Field) {}

type noopUpDownCounter struct{}

func (u *noopUpDownCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {}
