package o11y

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

type Tracer interface {
	Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
	WithAttributes(ctx context.Context, attrs ...Attribute)
}

type Span interface {
	End()
	SetAttributes(attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	SetStatus(status SpanStatus, msg string)
}

type SpanStatus int

const (
	SpanStatusOk SpanStatus = iota
	SpanStatusError
	SpanStatusUnset
)

type Attribute struct {
	Key   string
	Value any
}

const (
	defaultTracerBatchTimeout    = 5 * time.Second
	defaultTracerMaxExportBatch  = 512
	defaultTracerShutdownTimeout = 10 * time.Second
)

// TracerConfig holds configuration options for the tracer
type TracerConfig struct {
	endpoint        string
	serviceName     string
	resource        *resource.Resource
	insecure        bool
	tlsConfig       *tls.Config
	registerGlobal  bool
	batchTimeout    time.Duration
	maxExportBatch  int
	sampler         sdktrace.Sampler
	sensitiveKeys   []string
	redactSensitive bool
	strictTLS       bool
}

// TracerOption is a function that configures a TracerConfig
type TracerOption func(*TracerConfig)

// WithTracerEndpoint sets the OTLP endpoint for the tracer
func WithTracerEndpoint(endpoint string) TracerOption {
	return func(c *TracerConfig) {
		c.endpoint = endpoint
	}
}

// WithTracerServiceName sets the service name for the tracer
func WithTracerServiceName(name string) TracerOption {
	return func(c *TracerConfig) {
		c.serviceName = name
	}
}

// WithTracerResource sets the resource for the tracer
func WithTracerResource(res *resource.Resource) TracerOption {
	return func(c *TracerConfig) {
		c.resource = res
	}
}

// WithTracerInsecure enables insecure connection (not recommended for production)
func WithTracerInsecure() TracerOption {
	return func(c *TracerConfig) {
		c.insecure = true
	}
}

// WithTracerTLS sets custom TLS configuration
func WithTracerTLS(cfg *tls.Config) TracerOption {
	return func(c *TracerConfig) {
		c.tlsConfig = cfg
	}
}

// WithTracerGlobalRegistration enables/disables global tracer provider registration
func WithTracerGlobalRegistration(register bool) TracerOption {
	return func(c *TracerConfig) {
		c.registerGlobal = register
	}
}

// WithTracerBatchTimeout sets the batch timeout for span export
func WithTracerBatchTimeout(timeout time.Duration) TracerOption {
	return func(c *TracerConfig) {
		c.batchTimeout = timeout
	}
}

// WithTracerMaxExportBatch sets the maximum batch size for span export
func WithTracerMaxExportBatch(size int) TracerOption {
	return func(c *TracerConfig) {
		c.maxExportBatch = size
	}
}

// WithTracerSampler sets the sampling strategy for the tracer.
// Deprecated: Use WithTracerSamplingConfig instead to avoid exposing OpenTelemetry types.
// Common samplers include:
//   - sdktrace.AlwaysSample() - sample all traces (default, not recommended for high-volume production)
//   - sdktrace.NeverSample() - sample no traces
//   - sdktrace.TraceIDRatioBased(0.1) - sample 10% of traces
//   - sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1)) - respect parent sampling decision
func WithTracerSampler(sampler sdktrace.Sampler) TracerOption {
	return func(c *TracerConfig) {
		c.sampler = sampler
	}
}

// WithTracerSamplingConfig sets the sampling strategy using the abstracted SamplingConfig.
// This is the recommended way to configure sampling as it doesn't expose OpenTelemetry types.
// Example:
//
//	WithTracerSamplingConfig(o11y.SamplingConfig{
//	    Strategy: o11y.SamplingParentBased,
//	    Ratio:    0.1, // Sample 10% of traces
//	})
func WithTracerSamplingConfig(cfg SamplingConfig) TracerOption {
	return func(c *TracerConfig) {
		c.sampler = convertSamplingConfig(cfg)
	}
}

// WithTracerSensitiveFieldRedaction enables automatic redaction of sensitive fields in span attributes.
// When enabled, fields with keys matching sensitive patterns will have their values replaced with [REDACTED].
// This is enabled by default for security.
func WithTracerSensitiveFieldRedaction(enabled bool) TracerOption {
	return func(c *TracerConfig) {
		c.redactSensitive = enabled
	}
}

// WithTracerSensitiveKeys sets custom sensitive key patterns.
// These patterns are matched case-insensitively against attribute keys.
// If not set, DefaultSensitiveKeys will be used when redaction is enabled.
func WithTracerSensitiveKeys(keys []string) TracerOption {
	return func(c *TracerConfig) {
		c.sensitiveKeys = keys
	}
}

// WithTracerStrictTLS enables strict TLS validation mode.
// When enabled, insecure TLS configurations (InsecureSkipVerify, TLS < 1.2) will cause errors instead of warnings.
func WithTracerStrictTLS(strict bool) TracerOption {
	return func(c *TracerConfig) {
		c.strictTLS = strict
	}
}

func newTracerConfig(opts ...TracerOption) *TracerConfig {
	cfg := &TracerConfig{
		registerGlobal:  true,
		batchTimeout:    defaultTracerBatchTimeout,
		maxExportBatch:  defaultTracerMaxExportBatch,
		redactSensitive: true,
		sensitiveKeys:   DefaultSensitiveKeys,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type tracer struct {
	tracer          trace.Tracer
	sensitiveKeys   []string
	redactSensitive bool
	closed          *atomic.Bool
}

type otelSpan struct {
	span            trace.Span
	sensitiveKeys   []string
	redactSensitive bool
}

// NewTracer creates a new tracer with the given configuration
// Deprecated: Use NewTracerWithOptions instead for better control over TLS and batching.
// This function requires TLS by default. For insecure connections, use NewTracerWithOptions with WithTracerInsecure().
func NewTracer(ctx context.Context, endpoint, serviceName string, res *resource.Resource) (Tracer, func(context.Context) error, error) {
	return NewTracerWithOptions(ctx,
		WithTracerEndpoint(endpoint),
		WithTracerServiceName(serviceName),
		WithTracerResource(res),
	)
}

// NewTracerWithOptions creates a new tracer with functional options
func NewTracerWithOptions(ctx context.Context, opts ...TracerOption) (Tracer, func(context.Context) error, error) {
	cfg := newTracerConfig(opts...)

	if cfg.endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint cannot be empty")
	}
	validateEndpoint(cfg.endpoint, "tracer")
	if cfg.serviceName == "" {
		return nil, nil, fmt.Errorf("serviceName cannot be empty")
	}
	if cfg.resource == nil {
		return nil, nil, fmt.Errorf("resource cannot be nil")
	}
	if cfg.batchTimeout <= 0 {
		return nil, nil, fmt.Errorf("batchTimeout must be greater than 0")
	}
	if cfg.maxExportBatch <= 0 {
		return nil, nil, fmt.Errorf("maxExportBatch must be greater than 0")
	}

	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.endpoint),
	}
	exporterOpts, err := appendTracerTLSOptions(exporterOpts, cfg)
	if err != nil {
		return nil, nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize trace exporter grpc: %w", err)
	}

	providerOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(cfg.resource),
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(cfg.batchTimeout),
			sdktrace.WithMaxExportBatchSize(cfg.maxExportBatch),
		),
	}
	if cfg.sampler != nil {
		providerOpts = append(providerOpts, sdktrace.WithSampler(cfg.sampler))
	}

	tracerProvider := sdktrace.NewTracerProvider(providerOpts...)
	if tracerProvider == nil {
		// Clean up exporter to prevent resource leak
		if shutdownErr := traceExporter.Shutdown(ctx); shutdownErr != nil {
			log.Printf("tracer: failed to shutdown exporter after provider creation failed: %v", shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create tracer provider")
	}

	if cfg.registerGlobal {
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	}

	closed := &atomic.Bool{}
	shutdown := createTracerShutdown(tracerProvider, closed)

	return &tracer{
		tracer:          tracerProvider.Tracer(cfg.serviceName),
		sensitiveKeys:   cfg.sensitiveKeys,
		redactSensitive: cfg.redactSensitive,
		closed:          closed,
	}, shutdown, nil
}

func appendTracerTLSOptions(opts []otlptracegrpc.Option, cfg *TracerConfig) ([]otlptracegrpc.Option, error) {
	if cfg.insecure {
		if cfg.strictTLS {
			return nil, fmt.Errorf("tracer: insecure connection not allowed in strict TLS mode")
		}
		log.Printf("SECURITY WARNING: tracer using insecure connection to %s - not recommended for production", cfg.endpoint)
		return append(opts, otlptracegrpc.WithInsecure()), nil
	}

	if cfg.tlsConfig != nil {
		if err := validateTLSConfig(cfg.tlsConfig, "tracer", cfg.strictTLS); err != nil {
			return nil, err
		}
		return append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(cfg.tlsConfig))), nil
	}

	// Uses system root CAs by default
	return opts, nil
}

// validateTLSConfig checks for insecure TLS configurations.
// In strict mode, it returns an error; otherwise, it logs warnings.
func validateTLSConfig(cfg *tls.Config, component string, strict bool) error {
	if cfg == nil {
		return nil
	}
	if cfg.InsecureSkipVerify {
		if strict {
			return fmt.Errorf("%s: InsecureSkipVerify=true not allowed in strict TLS mode", component)
		}
		log.Printf("SECURITY WARNING: %s TLS InsecureSkipVerify=true disables certificate validation - MITM attacks possible", component)
	}
	if cfg.MinVersion != 0 && cfg.MinVersion < tls.VersionTLS12 {
		if strict {
			return fmt.Errorf("%s: TLS version < 1.2 not allowed in strict TLS mode", component)
		}
		log.Printf("SECURITY WARNING: %s TLS version < 1.2 is deprecated and may be insecure", component)
	}
	return nil
}

// validateEndpoint validates the endpoint URL for security concerns.
// It warns about potentially dangerous endpoints like metadata services or localhost.
func validateEndpoint(endpoint, component string) {
	// Try to parse as URL first
	host := endpoint
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		host = u.Host
	}

	// Strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Check for internal/dangerous addresses
	if isInternalAddress(host) {
		log.Printf("SECURITY WARNING: %s endpoint %q appears to be internal - ensure this is intentional", component, endpoint)
	}
}

// isInternalAddress checks if an address is internal/localhost/metadata service
func isInternalAddress(host string) bool {
	hostLower := strings.ToLower(host)

	// Check localhost variants
	if hostLower == "localhost" || hostLower == "127.0.0.1" || hostLower == "::1" {
		return true
	}

	// Check cloud metadata services
	metadataAddresses := []string{
		"169.254.169.254", // AWS/GCP/Azure metadata
		"metadata.google.internal",
		"metadata",
	}
	if slices.Contains(metadataAddresses, hostLower) {
		return true
	}

	// Check private IP ranges (basic check)
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}

	return false
}

func createTracerShutdown(provider *sdktrace.TracerProvider, closed *atomic.Bool) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTracerShutdownTimeout)
			defer cancel()
		}

		if err := provider.ForceFlush(ctx); err != nil {
			log.Printf("tracer: flush failed during shutdown: %v", err)
		}

		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("tracer: shutdown failed: %w", err)
		}

		// Mark tracer as closed to invalidate old references
		closed.Store(true)
		return nil
	}
}

func (t *tracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Return noop span if tracer has been shut down
	if t.closed != nil && t.closed.Load() {
		return ctx, noopSpan{}
	}
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(convertAttrs(attrs, t.sensitiveKeys, t.redactSensitive)...))
	return ctx, &otelSpan{
		span:            span,
		sensitiveKeys:   t.sensitiveKeys,
		redactSensitive: t.redactSensitive,
	}
}

func (t *tracer) WithAttributes(ctx context.Context, attrs ...Attribute) {
	if ctx == nil {
		return
	}
	// Do nothing if tracer has been shut down
	if t.closed != nil && t.closed.Load() {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.SetAttributes(convertAttrs(attrs, t.sensitiveKeys, t.redactSensitive)...)
	}
}

func (s *otelSpan) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(convertAttrs(attrs, s.sensitiveKeys, s.redactSensitive)...)
}

func (s *otelSpan) AddEvent(name string, attrs ...Attribute) {
	if s == nil || s.span == nil {
		return
	}
	s.span.AddEvent(name, trace.WithAttributes(convertAttrs(attrs, s.sensitiveKeys, s.redactSensitive)...))
}

var statusMap = map[SpanStatus]codes.Code{
	SpanStatusOk:    codes.Ok,
	SpanStatusError: codes.Error,
	SpanStatusUnset: codes.Unset,
}

func (s *otelSpan) SetStatus(status SpanStatus, msg string) {
	if s == nil || s.span == nil {
		return
	}
	if code, ok := statusMap[status]; ok {
		s.span.SetStatus(code, msg)
		return
	}
	s.span.SetStatus(codes.Unset, msg)
}

func convertAttrs(attrs []Attribute, sensitiveKeys []string, redact bool) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	kv := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if redact && isSensitiveKey(a.Key, sensitiveKeys) {
			kv = append(kv, attribute.String(a.Key, redactedValue))
			continue
		}
		switch v := a.Value.(type) {
		case string:
			kv = append(kv, attribute.String(a.Key, v))
		case int:
			kv = append(kv, attribute.Int(a.Key, v))
		case int64:
			kv = append(kv, attribute.Int64(a.Key, v))
		case int32:
			kv = append(kv, attribute.Int64(a.Key, int64(v)))
		case float64:
			kv = append(kv, attribute.Float64(a.Key, v))
		case float32:
			kv = append(kv, attribute.Float64(a.Key, float64(v)))
		case bool:
			kv = append(kv, attribute.Bool(a.Key, v))
		case []string:
			kv = append(kv, attribute.StringSlice(a.Key, v))
		case []int:
			kv = append(kv, attribute.IntSlice(a.Key, v))
		case []int64:
			kv = append(kv, attribute.Int64Slice(a.Key, v))
		case []float64:
			kv = append(kv, attribute.Float64Slice(a.Key, v))
		case []bool:
			kv = append(kv, attribute.BoolSlice(a.Key, v))
		case fmt.Stringer:
			s := v.String()
			kv = append(kv, attribute.String(a.Key, truncateAttrValue(s)))
		case nil:
			kv = append(kv, attribute.String(a.Key, "<nil>"))
		default:
			// Limit size of string representation to prevent memory issues
			s := fmt.Sprintf("%+v", v)
			const maxAttrSize = 2048
			if len(s) > maxAttrSize {
				s = s[:maxAttrSize-3] + "..."
				log.Printf("o11y: attribute %q truncated (type %T value too large: %d bytes)", a.Key, v, len(s))
			}
			kv = append(kv, attribute.String(a.Key, s))
		}
	}
	return kv
}

// truncateAttrValue truncates attribute values to prevent excessive memory usage
func truncateAttrValue(s string) string {
	const maxAttrSize = 2048
	if len(s) > maxAttrSize {
		return s[:maxAttrSize-3] + "..."
	}
	return s
}

// isSensitiveKey checks if a field key matches any sensitive key pattern
func isSensitiveKey(key string, sensitiveKeys []string) bool {
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}
