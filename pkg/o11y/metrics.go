package o11y

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"
)

const (
	// DefaultMaxMetricInstruments is the default maximum number of metric instruments allowed
	DefaultMaxMetricInstruments = 1000
	// DefaultMetricExportInterval is the default interval for exporting metrics
	DefaultMetricExportInterval  = 2 * time.Second
	defaultMetricsShutdownTimeout = 10 * time.Second
)

type Metrics interface {
	AddCounter(ctx context.Context, name string, v int64, labels ...any)
	RecordHistogram(ctx context.Context, name string, v float64, labels ...any)
}

// MetricsConfig holds configuration options for metrics
type MetricsConfig struct {
	endpoint       string
	serviceName    string
	resource       *resource.Resource
	insecure       bool
	tlsConfig      *tls.Config
	exportInterval time.Duration
	maxInstruments int
}

// MetricsOption is a function that configures a MetricsConfig
type MetricsOption func(*MetricsConfig)

// WithMetricsEndpoint sets the OTLP endpoint for metrics
func WithMetricsEndpoint(endpoint string) MetricsOption {
	return func(c *MetricsConfig) {
		c.endpoint = endpoint
	}
}

// WithMetricsServiceName sets the service name for metrics
func WithMetricsServiceName(name string) MetricsOption {
	return func(c *MetricsConfig) {
		c.serviceName = name
	}
}

// WithMetricsResource sets the resource for metrics
func WithMetricsResource(res *resource.Resource) MetricsOption {
	return func(c *MetricsConfig) {
		c.resource = res
	}
}

// WithMetricsInsecure enables insecure connection (not recommended for production)
func WithMetricsInsecure() MetricsOption {
	return func(c *MetricsConfig) {
		c.insecure = true
	}
}

// WithMetricsTLS sets custom TLS configuration
func WithMetricsTLS(cfg *tls.Config) MetricsOption {
	return func(c *MetricsConfig) {
		c.tlsConfig = cfg
	}
}

// WithMetricsExportInterval sets the export interval for metrics
func WithMetricsExportInterval(interval time.Duration) MetricsOption {
	return func(c *MetricsConfig) {
		c.exportInterval = interval
	}
}

// WithMetricsMaxInstruments sets the maximum number of metric instruments
func WithMetricsMaxInstruments(max int) MetricsOption {
	return func(c *MetricsConfig) {
		c.maxInstruments = max
	}
}

func newMetricsConfig(opts ...MetricsOption) *MetricsConfig {
	cfg := &MetricsConfig{
		exportInterval: DefaultMetricExportInterval,
		maxInstruments: DefaultMaxMetricInstruments,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type metrics struct {
	meter          otelmetric.Meter
	mu             sync.RWMutex
	counters       map[string]otelmetric.Int64Counter
	histograms     map[string]otelmetric.Float64Histogram
	maxInstruments int
}

// NewMetrics creates a new metrics instance with the given configuration
// Deprecated: Use NewMetricsWithOptions instead for better control over TLS and limits
func NewMetrics(ctx context.Context, endpoint, serviceName string, res *resource.Resource) (Metrics, func(context.Context) error, error) {
	return NewMetricsWithOptions(ctx,
		WithMetricsEndpoint(endpoint),
		WithMetricsServiceName(serviceName),
		WithMetricsResource(res),
		WithMetricsInsecure(), // Maintain backward compatibility
	)
}

// NewMetricsWithOptions creates a new metrics instance with functional options
func NewMetricsWithOptions(ctx context.Context, opts ...MetricsOption) (Metrics, func(context.Context) error, error) {
	cfg := newMetricsConfig(opts...)

	if cfg.endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint cannot be empty")
	}
	if cfg.serviceName == "" {
		return nil, nil, fmt.Errorf("serviceName cannot be empty")
	}
	if cfg.resource == nil {
		return nil, nil, fmt.Errorf("resource cannot be nil")
	}
	if cfg.maxInstruments <= 0 {
		return nil, nil, fmt.Errorf("maxInstruments must be greater than 0")
	}
	if cfg.exportInterval <= 0 {
		return nil, nil, fmt.Errorf("exportInterval must be greater than 0")
	}

	exporterOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.endpoint),
	}
	exporterOpts = appendMetricsTLSOptions(exporterOpts, cfg)

	metricExporter, err := otlpmetricgrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize metric exporter grpc: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(cfg.resource),
		metric.WithReader(metric.NewPeriodicReader(
			metricExporter,
			metric.WithInterval(cfg.exportInterval)),
		),
	)
	if meterProvider == nil {
		// Clean up exporter to prevent resource leak
		if shutdownErr := metricExporter.Shutdown(ctx); shutdownErr != nil {
			log.Printf("metrics: failed to shutdown exporter after provider creation failed: %v", shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create meter provider")
	}

	shutdown := createMetricsShutdown(meterProvider)

	return &metrics{
		meter:          meterProvider.Meter(cfg.serviceName),
		counters:       make(map[string]otelmetric.Int64Counter),
		histograms:     make(map[string]otelmetric.Float64Histogram),
		maxInstruments: cfg.maxInstruments,
	}, shutdown, nil
}

func appendMetricsTLSOptions(opts []otlpmetricgrpc.Option, cfg *MetricsConfig) []otlpmetricgrpc.Option {
	if cfg.insecure {
		log.Printf("WARNING: metrics using insecure connection to %s - not recommended for production", cfg.endpoint)
		return append(opts, otlpmetricgrpc.WithInsecure())
	}

	if cfg.tlsConfig != nil {
		return append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(cfg.tlsConfig)))
	}

	// Uses system root CAs by default
	return opts
}

func createMetricsShutdown(provider *metric.MeterProvider) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultMetricsShutdownTimeout)
			defer cancel()
		}

		if err := provider.ForceFlush(ctx); err != nil {
			log.Printf("metrics: flush failed during shutdown: %v", err)
		}

		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("metrics: shutdown failed: %w", err)
		}
		return nil
	}
}

func (m *metrics) getOrCreateCounter(name string) (otelmetric.Int64Counter, error) {
	// Tentar read lock primeiro (caso comum - instrumento existe)
	m.mu.RLock()
	if ctr, ok := m.counters[name]; ok {
		m.mu.RUnlock()
		return ctr, nil
	}
	m.mu.RUnlock()

	// Adquirir write lock para criar
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check - outro goroutine pode ter criado
	if ctr, ok := m.counters[name]; ok {
		return ctr, nil
	}

	// Check map size limit to prevent memory exhaustion
	totalInstruments := len(m.counters) + len(m.histograms)
	if totalInstruments >= m.maxInstruments {
		return nil, fmt.Errorf("metrics: maximum instrument limit (%d) reached, rejecting new counter %q", m.maxInstruments, name)
	}

	// Criar novo instrumento
	ctr, err := m.meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}
	m.counters[name] = ctr
	return ctr, nil
}

func (m *metrics) AddCounter(ctx context.Context, name string, v int64, labels ...any) {
	ctr, err := m.getOrCreateCounter(name)
	if err != nil {
		log.Printf("metrics: failed to create counter %q: %v", name, err)
		return
	}
	attrs := m.parseLabels(labels...)
	ctr.Add(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) getOrCreateHistogram(name string) (otelmetric.Float64Histogram, error) {
	// Tentar read lock primeiro (caso comum - instrumento existe)
	m.mu.RLock()
	if h, ok := m.histograms[name]; ok {
		m.mu.RUnlock()
		return h, nil
	}
	m.mu.RUnlock()

	// Adquirir write lock para criar
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check - outro goroutine pode ter criado
	if h, ok := m.histograms[name]; ok {
		return h, nil
	}

	// Check map size limit to prevent memory exhaustion
	totalInstruments := len(m.counters) + len(m.histograms)
	if totalInstruments >= m.maxInstruments {
		return nil, fmt.Errorf("metrics: maximum instrument limit (%d) reached, rejecting new histogram %q", m.maxInstruments, name)
	}

	// Criar novo instrumento
	h, err := m.meter.Float64Histogram(name)
	if err != nil {
		return nil, err
	}
	m.histograms[name] = h
	return h, nil
}

func (m *metrics) RecordHistogram(ctx context.Context, name string, v float64, labels ...any) {
	h, err := m.getOrCreateHistogram(name)
	if err != nil {
		log.Printf("metrics: failed to create histogram %q: %v", name, err)
		return
	}
	attrs := m.parseLabels(labels...)
	h.Record(ctx, v, otelmetric.WithAttributes(attrs...))
}

func (m *metrics) parseLabels(labels ...any) []attribute.KeyValue {
	if len(labels) == 0 {
		return nil
	}

	// Warn about odd number of labels (missing value for last key)
	if len(labels)%2 != 0 {
		log.Printf("metrics: odd number of labels provided (%d), last key will be ignored", len(labels))
	}

	kv := make([]attribute.KeyValue, 0, len(labels)/2)
	for i := 0; i+1 < len(labels); i += 2 {
		k, ok := labels[i].(string)
		if !ok {
			log.Printf("metrics: label key at position %d is not a string (type %T), skipping", i, labels[i])
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
		case float64:
			kv = append(kv, attribute.Float64(k, tv))
		case bool:
			kv = append(kv, attribute.Bool(k, tv))
		case nil:
			kv = append(kv, attribute.String(k, "<nil>"))
		default:
			kv = append(kv, attribute.String(k, fmt.Sprintf("%v", tv)))
		}
	}
	return kv
}
