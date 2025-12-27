package observability

import "context"

// Metrics provides application metrics capabilities.
type Metrics interface {
	// Counter returns a counter metric instrument.
	Counter(name, description, unit string) Counter

	// Histogram returns a histogram metric instrument.
	Histogram(name, description, unit string) Histogram

	// UpDownCounter returns an up-down counter metric instrument.
	UpDownCounter(name, description, unit string) UpDownCounter

	// Gauge returns a gauge metric instrument (asynchronous).
	Gauge(name, description, unit string, callback GaugeCallback) error
}

// Counter is a monotonically increasing metric.
type Counter interface {
	// Add increments the counter by the given value with optional attributes.
	Add(ctx context.Context, value int64, fields ...Field)

	// Increment increments the counter by 1 with optional attributes.
	Increment(ctx context.Context, fields ...Field)
}

// Histogram records a distribution of values.
type Histogram interface {
	// Record adds a value to the histogram with optional attributes.
	Record(ctx context.Context, value float64, fields ...Field)
}

// UpDownCounter is a metric that can increase and decrease.
type UpDownCounter interface {
	// Add adds the given value (can be positive or negative) with optional attributes.
	Add(ctx context.Context, value int64, fields ...Field)
}

// GaugeCallback is a function that returns the current value for a gauge metric.
type GaugeCallback func(ctx context.Context) float64
