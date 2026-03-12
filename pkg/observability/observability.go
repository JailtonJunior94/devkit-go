package observability

import (
	"context"
	"math"
)

// Observability is the main facade interface that provides access to all observability features.
// This is the only interface that should be injected into your application layers.
type Observability interface {
	Tracer() Tracer
	Logger() Logger
	Metrics() Metrics
	// Shutdown gracefully flushes pending telemetry and releases resources.
	// Always call with a timeout context to avoid blocking indefinitely.
	Shutdown(ctx context.Context) error
}

// FieldKind identifies the type of value stored in a Field.
// Using a discriminated union instead of interface{} eliminates boxing allocations
// for the five most common field types: string, int, int64, float64, and bool.
type FieldKind uint8

const (
	// FieldKindString stores the value in the internal string field. Zero allocations.
	FieldKindString FieldKind = iota
	// FieldKindInt stores an int value in the internal numeric field. Zero allocations.
	FieldKindInt
	// FieldKindInt64 stores an int64 value in the internal numeric field. Zero allocations.
	FieldKindInt64
	// FieldKindFloat64 stores a float64 (as IEEE 754 bits) in the numeric field. Zero allocations.
	FieldKindFloat64
	// FieldKindBool stores a bool (0=false, 1=true) in the numeric field. Zero allocations.
	FieldKindBool
	// FieldKindError stores an error interface in anyVal. One boxing allocation at construction.
	// The original error type is preserved to support errors.Is and errors.As.
	FieldKindError
	// FieldKindAny stores any value in anyVal. One boxing allocation for non-pointer types.
	FieldKindAny
)

// Field represents a key-value pair for structured logging and tracing attributes.
//
// It uses a discriminated union to avoid interface{} boxing for the five most
// common types (string, int, int64, float64, bool), reducing heap allocations
// and GC pressure in hot paths. Memory layout: 64 bytes (one cache line).
type Field struct {
	Key    string    // 16 bytes on 64-bit
	numVal uint64    // stores int, int64, float64 bits, bool — zero boxing
	strVal string    // stores string values — zero boxing
	anyVal any       // stores error and any — boxing unavoidable for interfaces
	kind   FieldKind // type discriminator: uint8
}

// Kind returns the type discriminator for this field.
func (f Field) Kind() FieldKind { return f.kind }

// StringValue returns the string value. Valid when Kind() == FieldKindString.
func (f Field) StringValue() string { return f.strVal }

// Int64Value returns the numeric value as int64.
// Valid when Kind() is FieldKindInt or FieldKindInt64.
func (f Field) Int64Value() int64 { return int64(f.numVal) }

// Float64Value returns the float64 value. Valid when Kind() == FieldKindFloat64.
func (f Field) Float64Value() float64 { return math.Float64frombits(f.numVal) }

// BoolValue returns the bool value. Valid when Kind() == FieldKindBool.
func (f Field) BoolValue() bool { return f.numVal != 0 }

// AnyValue returns the field value as interface{} for introspection and testing.
// For typed fields (String, Int, Float64, Bool), the value is boxed into interface{} here —
// use the typed accessors in hot paths to avoid the allocation.
// For Error and Any fields, returns the already-boxed anyVal directly (no extra alloc).
func (f Field) AnyValue() any {
	switch f.kind {
	case FieldKindString:
		return f.strVal
	case FieldKindInt:
		return int(f.numVal)
	case FieldKindInt64:
		return int64(f.numVal)
	case FieldKindFloat64:
		return math.Float64frombits(f.numVal)
	case FieldKindBool:
		return f.numVal != 0
	case FieldKindError, FieldKindAny:
		return f.anyVal
	default:
		return nil
	}
}

// String creates a string field. Zero allocations.
func String(key, value string) Field {
	return Field{Key: key, strVal: value, kind: FieldKindString}
}

// Int creates an integer field. Zero allocations.
func Int(key string, value int) Field {
	return Field{Key: key, numVal: uint64(int64(value)), kind: FieldKindInt}
}

// Int64 creates an int64 field. Zero allocations.
func Int64(key string, value int64) Field {
	return Field{Key: key, numVal: uint64(value), kind: FieldKindInt64}
}

// Float64 creates a float64 field. Zero allocations.
func Float64(key string, value float64) Field {
	return Field{Key: key, numVal: math.Float64bits(value), kind: FieldKindFloat64}
}

// Bool creates a boolean field. Zero allocations.
func Bool(key string, value bool) Field {
	var n uint64
	if value {
		n = 1
	}
	return Field{Key: key, numVal: n, kind: FieldKindBool}
}

// Error creates an error field. Stores the original error interface (one boxing allocation).
// The original error type is preserved to support errors.Is and errors.As.
func Error(err error) Field {
	return Field{Key: "error", anyVal: err, kind: FieldKindError}
}

// Any creates a field with any value. One boxing allocation for non-pointer types.
func Any(key string, value any) Field {
	return Field{Key: key, anyVal: value, kind: FieldKindAny}
}

// SpanContext represents the context needed to propagate trace information.
type SpanContext interface {
	TraceID() string
	SpanID() string
	IsSampled() bool
}

// Span represents an active trace span.
type Span interface {
	// End finishes the span. No further operations should be performed on the span after calling End.
	End()
	// SetAttributes sets additional attributes on the span.
	SetAttributes(fields ...Field)
	// SetStatus sets the status of the span.
	SetStatus(code StatusCode, description string)
	// RecordError records an error as an event on the span.
	RecordError(err error, fields ...Field)
	// AddEvent adds an event to the span.
	AddEvent(name string, fields ...Field)
	// Context returns the span context.
	Context() SpanContext
	// TraceID returns the trace ID as a lowercase hex string.
	// Zero-allocation fast path — prefer this over Context().TraceID() in hot paths.
	// Returns "" for unsampled or noop spans.
	TraceID() string
	// SpanID returns the span ID as a lowercase hex string.
	// Zero-allocation fast path — prefer this over Context().SpanID() in hot paths.
	// Returns "" for unsampled or noop spans.
	SpanID() string
	// IsSampled reports whether the span is being sampled.
	IsSampled() bool
}

// StatusCode represents the canonical status code of a span.
type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

// SpanKind represents the role of a span in a trace.
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// SpanConfig holds the resolved span configuration for provider implementations.
// Concrete struct (not interface) so callers can stack-allocate via ApplySpanOptions,
// eliminating the *spanConfig heap allocation present in the old NewSpanConfig design.
type SpanConfig struct {
	kind       SpanKind
	attributes []Field
}

// Kind returns the span kind.
func (c SpanConfig) Kind() SpanKind { return c.kind }

// Attributes returns the span attributes.
func (c SpanConfig) Attributes() []Field { return c.attributes }

// SpanOption configures span creation.
type SpanOption interface {
	apply(*SpanConfig)
}

// spanKindOpt is a concrete value type for WithSpanKind.
// Replaces the previous spanOptionFunc closure: a concrete struct boxed into
// a SpanOption interface is smaller and more inlineable than a function closure.
type spanKindOpt struct{ kind SpanKind }

func (o spanKindOpt) apply(c *SpanConfig) { c.kind = o.kind }

// spanAttrsOpt is a concrete value type for WithAttributes.
type spanAttrsOpt struct{ attrs []Field }

func (o spanAttrsOpt) apply(c *SpanConfig) { c.attributes = append(c.attributes, o.attrs...) }

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption { return spanKindOpt{kind: kind} }

// WithAttributes sets initial attributes on the span.
func WithAttributes(fields ...Field) SpanOption { return spanAttrsOpt{attrs: fields} }

// ApplySpanOptions applies opts to a caller-allocated SpanConfig.
// Enables stack allocation in the caller, eliminating the *SpanConfig heap
// allocation incurred by NewSpanConfig on every traced call with options.
//
//	var cfg observability.SpanConfig
//	observability.ApplySpanOptions(&cfg, opts)
func ApplySpanOptions(cfg *SpanConfig, opts []SpanOption) {
	cfg.kind = SpanKindInternal
	for _, opt := range opts {
		opt.apply(cfg)
	}
}

// NewSpanConfig creates a span configuration from options (exported for provider implementations).
// For hot paths, prefer ApplySpanOptions with a stack-allocated SpanConfig.
func NewSpanConfig(opts []SpanOption) SpanConfig {
	var cfg SpanConfig
	ApplySpanOptions(&cfg, opts)
	return cfg
}
