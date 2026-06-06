package observability

import "context"

type Metrics interface {
	Counter(name, description, unit string) Counter
	Histogram(name, description, unit string) Histogram
	HistogramWithBuckets(name, description, unit string, buckets []float64) Histogram
	UpDownCounter(name, description, unit string) UpDownCounter
	Gauge(name, description, unit string, callback GaugeCallback) error
}

type Counter interface {
	Add(ctx context.Context, value int64, fields ...Field)
	Increment(ctx context.Context, fields ...Field)
}

type Histogram interface {
	Record(ctx context.Context, value float64, fields ...Field)
}

type UpDownCounter interface {
	Add(ctx context.Context, value int64, fields ...Field)
}

type GaugeCallback func(ctx context.Context) float64
