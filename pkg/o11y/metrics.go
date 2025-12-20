package o11y

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
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
	DefaultMetricExportInterval   = 2 * time.Second
	defaultMetricsShutdownTimeout = 10 * time.Second
	// DefaultMaxLabelsPerMetric is the maximum number of labels allowed per metric call
	DefaultMaxLabelsPerMetric = 10
	// DefaultMaxLabelValueLength is the maximum length of a label value
	DefaultMaxLabelValueLength = 256
)

type Metrics interface {
	AddCounter(ctx context.Context, name string, v int64, labels ...any)
	RecordHistogram(ctx context.Context, name string, v float64, labels ...any)
}

// MetricsErrorCallback is called when a metrics operation fails
type MetricsErrorCallback func(err error)

// MetricsConfig holds configuration options for metrics
type MetricsConfig struct {
	endpoint           string
	serviceName        string
	resource           *resource.Resource
	insecure           bool
	tlsConfig          *tls.Config
	exportInterval     time.Duration
	maxInstruments     int
	maxLabelsPerMetric int
	maxLabelValueLen   int
	onError            MetricsErrorCallback
	sensitiveKeys      []string
	redactSensitive    bool
	strictTLS          bool
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

// WithMetricsMaxLabelsPerMetric sets the maximum number of labels per metric call.
// Labels exceeding this limit will be truncated to prevent cardinality explosion.
func WithMetricsMaxLabelsPerMetric(max int) MetricsOption {
	return func(c *MetricsConfig) {
		c.maxLabelsPerMetric = max
	}
}

// WithMetricsMaxLabelValueLength sets the maximum length for label values.
// Values exceeding this length will be truncated.
func WithMetricsMaxLabelValueLength(max int) MetricsOption {
	return func(c *MetricsConfig) {
		c.maxLabelValueLen = max
	}
}

// WithMetricsOnError sets a callback function that is called when a metrics operation fails.
// This allows consumers to be notified of metric errors instead of them being silently logged.
func WithMetricsOnError(callback MetricsErrorCallback) MetricsOption {
	return func(c *MetricsConfig) {
		c.onError = callback
	}
}

// WithMetricsSensitiveFieldRedaction enables automatic redaction of sensitive fields in metric labels.
// When enabled, labels with keys matching sensitive patterns will have their values replaced with [REDACTED].
// This is enabled by default for security.
func WithMetricsSensitiveFieldRedaction(enabled bool) MetricsOption {
	return func(c *MetricsConfig) {
		c.redactSensitive = enabled
	}
}

// WithMetricsSensitiveKeys sets custom sensitive key patterns.
// These patterns are matched case-insensitively against label keys.
// If not set, DefaultSensitiveKeys will be used when redaction is enabled.
func WithMetricsSensitiveKeys(keys []string) MetricsOption {
	return func(c *MetricsConfig) {
		c.sensitiveKeys = keys
	}
}

// WithMetricsStrictTLS enables strict TLS validation mode.
// When enabled, insecure TLS configurations (InsecureSkipVerify, TLS < 1.2) will cause errors instead of warnings.
func WithMetricsStrictTLS(strict bool) MetricsOption {
	return func(c *MetricsConfig) {
		c.strictTLS = strict
	}
}

func newMetricsConfig(opts ...MetricsOption) *MetricsConfig {
	cfg := &MetricsConfig{
		exportInterval:     DefaultMetricExportInterval,
		maxInstruments:     DefaultMaxMetricInstruments,
		maxLabelsPerMetric: DefaultMaxLabelsPerMetric,
		maxLabelValueLen:   DefaultMaxLabelValueLength,
		redactSensitive:    true,
		sensitiveKeys:      DefaultSensitiveKeys,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type metrics struct {
	meter              otelmetric.Meter
	mu                 sync.RWMutex
	counters           map[string]otelmetric.Int64Counter
	histograms         map[string]otelmetric.Float64Histogram
	maxInstruments     int
	maxLabelsPerMetric int
	maxLabelValueLen   int
	onError            MetricsErrorCallback
	sensitiveKeys      []string
	redactSensitive    bool
}

// NewMetrics creates a new metrics instance with the given configuration
// Deprecated: Use NewMetricsWithOptions instead for better control over TLS and limits.
// This function requires TLS by default. For insecure connections, use NewMetricsWithOptions with WithMetricsInsecure().
func NewMetrics(ctx context.Context, endpoint, serviceName string, res *resource.Resource) (Metrics, func(context.Context) error, error) {
	return NewMetricsWithOptions(ctx,
		WithMetricsEndpoint(endpoint),
		WithMetricsServiceName(serviceName),
		WithMetricsResource(res),
	)
}

// NewMetricsWithOptions creates a new metrics instance with functional options
func NewMetricsWithOptions(ctx context.Context, opts ...MetricsOption) (Metrics, func(context.Context) error, error) {
	cfg := newMetricsConfig(opts...)

	if cfg.endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint cannot be empty")
	}
	validateEndpoint(cfg.endpoint, "metrics")
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
	if cfg.maxLabelsPerMetric <= 0 {
		return nil, nil, fmt.Errorf("maxLabelsPerMetric must be greater than 0")
	}
	if cfg.maxLabelValueLen < 4 {
		return nil, nil, fmt.Errorf("maxLabelValueLen must be at least 4")
	}

	exporterOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.endpoint),
	}
	exporterOpts, err := appendMetricsTLSOptions(exporterOpts, cfg)
	if err != nil {
		return nil, nil, err
	}

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
		meter:              meterProvider.Meter(cfg.serviceName),
		counters:           make(map[string]otelmetric.Int64Counter),
		histograms:         make(map[string]otelmetric.Float64Histogram),
		maxInstruments:     cfg.maxInstruments,
		maxLabelsPerMetric: cfg.maxLabelsPerMetric,
		maxLabelValueLen:   cfg.maxLabelValueLen,
		onError:            cfg.onError,
		sensitiveKeys:      cfg.sensitiveKeys,
		redactSensitive:    cfg.redactSensitive,
	}, shutdown, nil
}

func appendMetricsTLSOptions(opts []otlpmetricgrpc.Option, cfg *MetricsConfig) ([]otlpmetricgrpc.Option, error) {
	if cfg.insecure {
		if cfg.strictTLS {
			return nil, fmt.Errorf("metrics: insecure connection not allowed in strict TLS mode")
		}
		log.Printf("SECURITY WARNING: metrics using insecure connection to %s - not recommended for production", cfg.endpoint)
		return append(opts, otlpmetricgrpc.WithInsecure()), nil
	}

	if cfg.tlsConfig != nil {
		if err := validateTLSConfig(cfg.tlsConfig, "metrics", cfg.strictTLS); err != nil {
			return nil, err
		}
		return append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(cfg.tlsConfig))), nil
	}

	// Uses system root CAs by default
	return opts, nil
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
	m.mu.RLock()
	if ctr, ok := m.counters[name]; ok {
		m.mu.RUnlock()
		return ctr, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if ctr, ok := m.counters[name]; ok {
		return ctr, nil
	}

	totalInstruments := len(m.counters) + len(m.histograms)
	if totalInstruments >= m.maxInstruments {
		return nil, fmt.Errorf("metrics: maximum instrument limit (%d) reached, rejecting new counter %q", m.maxInstruments, name)
	}

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
		m.handleError(err)
		return
	}
	attrs := m.parseLabels(labels...)
	ctr.Add(ctx, v, otelmetric.WithAttributes(attrs...))
}

// handleError logs the error and calls the error callback if configured
func (m *metrics) handleError(err error) {
	log.Printf("metrics: %v", err)
	if m.onError != nil {
		m.onError(err)
	}
}

func (m *metrics) getOrCreateHistogram(name string) (otelmetric.Float64Histogram, error) {
	m.mu.RLock()
	if h, ok := m.histograms[name]; ok {
		m.mu.RUnlock()
		return h, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.histograms[name]; ok {
		return h, nil
	}

	totalInstruments := len(m.counters) + len(m.histograms)
	if totalInstruments >= m.maxInstruments {
		return nil, fmt.Errorf("metrics: maximum instrument limit (%d) reached, rejecting new histogram %q", m.maxInstruments, name)
	}

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
		m.handleError(err)
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

	// Apply cardinality protection - limit number of labels
	maxPairs := m.maxLabelsPerMetric
	if maxPairs <= 0 {
		maxPairs = DefaultMaxLabelsPerMetric
	}
	labelPairs := len(labels) / 2
	if labelPairs > maxPairs {
		log.Printf("metrics: too many labels (%d), truncating to %d to prevent cardinality explosion", labelPairs, maxPairs)
		labels = labels[:maxPairs*2]
	}

	kv := make([]attribute.KeyValue, 0, len(labels)/2)
	for i := 0; i+1 < len(labels); i += 2 {
		k, ok := labels[i].(string)
		if !ok {
			log.Printf("metrics: label key at position %d is not a string (type %T), skipping", i, labels[i])
			continue
		}
		val := labels[i+1]
		kv = append(kv, m.createAttribute(k, val))
	}
	return kv
}

// createAttribute creates an attribute with value length protection and sensitive field redaction
func (m *metrics) createAttribute(key string, val any) attribute.KeyValue {
	// Check for sensitive keys first
	if m.redactSensitive && m.isSensitiveKey(key) {
		return attribute.String(key, redactedValue)
	}

	maxLen := m.maxLabelValueLen
	if maxLen <= 0 {
		maxLen = DefaultMaxLabelValueLength
	}

	switch tv := val.(type) {
	case string:
		return attribute.String(key, m.truncateString(tv, maxLen))
	case int64:
		return attribute.Int64(key, tv)
	case int:
		return attribute.Int(key, tv)
	case float64:
		return attribute.Float64(key, tv)
	case bool:
		return attribute.Bool(key, tv)
	case nil:
		return attribute.String(key, "<nil>")
	default:
		s := fmt.Sprintf("%v", tv)
		return attribute.String(key, m.truncateString(s, maxLen))
	}
}

// isSensitiveKey checks if a label key matches any sensitive key pattern
func (m *metrics) isSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, sensitive := range m.sensitiveKeys {
		if strings.Contains(keyLower, strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}

// truncateString truncates a string to maxLen characters
func (m *metrics) truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
