package o11y

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"
)

// Metrics provides metrics capabilities.
type Metrics interface {
	// AddCounter increments a counter by the given value.
	AddCounter(ctx context.Context, name string, v int64, labels ...any)
	// RecordHistogram records a value in a histogram.
	RecordHistogram(ctx context.Context, name string, v float64, labels ...any)
	// SetGauge sets a gauge to the given value.
	SetGauge(ctx context.Context, name string, v float64, labels ...any)
	// AddUpDownCounter adds a value to an up-down counter (can be negative).
	AddUpDownCounter(ctx context.Context, name string, v int64, labels ...any)
	// RecordDuration is a helper to record the duration of an operation.
	RecordDuration(ctx context.Context, name string, start time.Time, labels ...any)
}

type metrics struct {
	meter          otelmetric.Meter
	counters       sync.Map
	histograms     sync.Map
	gauges         sync.Map
	upDownCounters sync.Map
	onError        func(err error)
}

// NewMetrics creates a new metrics instance with the given configuration.
// By default, TLS is enabled using system certificates. Use WithMetricsInsecure() for development.
//
// Errors during metric recording are logged to stderr by default. Use WithMetricsErrorHandler
// to customize error handling.
func NewMetrics(ctx context.Context, endpoint, serviceName string, resource *resource.Resource, opts ...MetricsOption) (Metrics, func(context.Context) error, error) {
	cfg := defaultMetricsConfig(endpoint)
	for _, opt := range opts {
		opt(cfg)
	}

	grpcOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}

	switch {
	case cfg.Insecure:
		grpcOpts = append(grpcOpts, otlpmetricgrpc.WithInsecure())
	case cfg.TLSConfig != nil:
		grpcOpts = append(grpcOpts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(cfg.TLSConfig)))
	default:
		grpcOpts = append(grpcOpts, otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	metricExporter, err := otlpmetricgrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize metric exporter grpc: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(resource),
		metric.WithReader(metric.NewPeriodicReader(
			metricExporter,
			metric.WithInterval(cfg.ExportInterval),
		)),
	)

	shutdown := func(ctx context.Context) error {
		return meterProvider.Shutdown(ctx)
	}

	errorHandler := cfg.ErrorHandler
	if errorHandler == nil {
		errorHandler = func(err error) {
			fmt.Fprintf(os.Stderr, "telemetry metrics error: %v\n", err)
		}
	}

	return &metrics{
		meter:   meterProvider.Meter(serviceName),
		onError: errorHandler,
	}, shutdown, nil
}

func (m *metrics) AddCounter(ctx context.Context, name string, v int64, labels ...any) {
	ctr, err := m.getOrCreateCounter(name)
	if err != nil {
		m.handleError(fmt.Errorf("AddCounter(%s): %w", name, err))
		return
	}
	attrs := parseLabels(labels...)
	ctr.Add(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) RecordHistogram(ctx context.Context, name string, v float64, labels ...any) {
	h, err := m.getOrCreateHistogram(name)
	if err != nil {
		m.handleError(fmt.Errorf("RecordHistogram(%s): %w", name, err))
		return
	}
	attrs := parseLabels(labels...)
	h.Record(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) SetGauge(ctx context.Context, name string, v float64, labels ...any) {
	g, err := m.getOrCreateGauge(name)
	if err != nil {
		m.handleError(fmt.Errorf("SetGauge(%s): %w", name, err))
		return
	}
	attrs := parseLabels(labels...)
	g.Record(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) AddUpDownCounter(ctx context.Context, name string, v int64, labels ...any) {
	ctr, err := m.getOrCreateUpDownCounter(name)
	if err != nil {
		m.handleError(fmt.Errorf("AddUpDownCounter(%s): %w", name, err))
		return
	}
	attrs := parseLabels(labels...)
	ctr.Add(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) handleError(err error) {
	if m.onError != nil {
		m.onError(err)
	}
}

func (m *metrics) RecordDuration(ctx context.Context, name string, start time.Time, labels ...any) {
	duration := time.Since(start).Seconds()
	m.RecordHistogram(ctx, name, duration, labels...)
}

func (m *metrics) getOrCreateCounter(name string) (otelmetric.Int64Counter, error) {
	if v, ok := m.counters.Load(name); ok {
		return v.(otelmetric.Int64Counter), nil
	}

	ctr, err := m.meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}

	actual, _ := m.counters.LoadOrStore(name, ctr)
	return actual.(otelmetric.Int64Counter), nil
}

func (m *metrics) getOrCreateHistogram(name string) (otelmetric.Float64Histogram, error) {
	if v, ok := m.histograms.Load(name); ok {
		return v.(otelmetric.Float64Histogram), nil
	}

	h, err := m.meter.Float64Histogram(name)
	if err != nil {
		return nil, err
	}

	actual, _ := m.histograms.LoadOrStore(name, h)
	return actual.(otelmetric.Float64Histogram), nil
}

func (m *metrics) getOrCreateGauge(name string) (otelmetric.Float64Gauge, error) {
	if v, ok := m.gauges.Load(name); ok {
		return v.(otelmetric.Float64Gauge), nil
	}

	g, err := m.meter.Float64Gauge(name)
	if err != nil {
		return nil, err
	}

	actual, _ := m.gauges.LoadOrStore(name, g)
	return actual.(otelmetric.Float64Gauge), nil
}

func (m *metrics) getOrCreateUpDownCounter(name string) (otelmetric.Int64UpDownCounter, error) {
	if v, ok := m.upDownCounters.Load(name); ok {
		return v.(otelmetric.Int64UpDownCounter), nil
	}

	ctr, err := m.meter.Int64UpDownCounter(name)
	if err != nil {
		return nil, err
	}

	actual, _ := m.upDownCounters.LoadOrStore(name, ctr)
	return actual.(otelmetric.Int64UpDownCounter), nil
}

func parseLabels(labels ...any) []attribute.KeyValue {
	if len(labels) == 0 {
		return nil
	}

	kv := make([]attribute.KeyValue, 0, len(labels)/2)
	for i := 0; i+1 < len(labels); i += 2 {
		k, ok := labels[i].(string)
		if !ok {
			continue
		}
		val := labels[i+1]
		switch tv := val.(type) {
		case string:
			kv = append(kv, attribute.String(k, tv))
		case int64:
			kv = append(kv, attribute.Int64(k, tv))
		case int:
			kv = append(kv, attribute.Int(k, tv))
		case int32:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case int16:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case int8:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case uint:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case uint64:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case uint32:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case uint16:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case uint8:
			kv = append(kv, attribute.Int64(k, int64(tv)))
		case bool:
			kv = append(kv, attribute.Bool(k, tv))
		case float64:
			kv = append(kv, attribute.Float64(k, tv))
		case float32:
			kv = append(kv, attribute.Float64(k, float64(tv)))
		case []string:
			kv = append(kv, attribute.StringSlice(k, tv))
		case []int:
			kv = append(kv, attribute.IntSlice(k, tv))
		case []int64:
			kv = append(kv, attribute.Int64Slice(k, tv))
		case []float64:
			kv = append(kv, attribute.Float64Slice(k, tv))
		case []bool:
			kv = append(kv, attribute.BoolSlice(k, tv))
		case fmt.Stringer:
			kv = append(kv, attribute.String(k, tv.String()))
		default:
			kv = append(kv, attribute.String(k, fmt.Sprintf("%v", tv)))
		}
	}
	return kv
}
