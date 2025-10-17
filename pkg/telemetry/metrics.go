package o11y

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Metrics interface {
	AddCounter(ctx context.Context, name string, v int64, labels ...any)
	RecordHistogram(ctx context.Context, name string, v float64, labels ...any)
}

type metrics struct {
	meter otelmetric.Meter
}

func NewMetrics(ctx context.Context, endpoint, serviceName string, resource *resource.Resource) (Metrics, func(context.Context) error, error) {
	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize metric exporter grpc: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(resource),
		metric.WithReader(metric.NewPeriodicReader(
			metricExporter,
			metric.WithInterval(2*time.Second)),
		),
	)

	shutdown := func(ctx context.Context) error {
		if err := meterProvider.Shutdown(ctx); err != nil {
			return err
		}
		return nil
	}

	return &metrics{meter: meterProvider.Meter(serviceName)}, shutdown, nil
}

func (m *metrics) AddCounter(ctx context.Context, name string, v int64, labels ...any) {
	ctr, err := m.meter.Int64Counter(name)
	if err != nil {
		return
	}
	attrs := m.parseLabels(labels...)
	ctr.Add(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) RecordHistogram(ctx context.Context, name string, v float64, labels ...any) {
	h, err := m.meter.Float64Histogram(name)
	if err != nil {
		return
	}
	attrs := m.parseLabels(labels...)
	h.Record(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) parseLabels(labels ...any) []attribute.KeyValue {
	var kv []attribute.KeyValue
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
		case bool:
			kv = append(kv, attribute.Bool(k, tv))
		default:
			kv = append(kv, attribute.String(k, fmt.Sprintf("%v", tv)))
		}
	}
	return kv
}
