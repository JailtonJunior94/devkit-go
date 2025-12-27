package observability

// Observability is the main facade interface that provides access to all observability features.
// This is the only interface that should be injected into your application layers.
type Observability interface {
	Tracer() Tracer
	Logger() Logger
	Metrics() Metrics
}

// Field represents a key-value pair for structured logging and tracing attributes.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field.
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value type.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
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

// SpanOption configures span creation.
type SpanOption interface {
	apply(*spanConfig)
}

type spanConfig struct {
	kind       SpanKind
	attributes []Field
}

type spanOptionFunc func(*spanConfig)

func (f spanOptionFunc) apply(c *spanConfig) {
	f(c)
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return spanOptionFunc(func(c *spanConfig) {
		c.kind = kind
	})
}

// WithAttributes sets initial attributes on the span.
func WithAttributes(fields ...Field) SpanOption {
	return spanOptionFunc(func(c *spanConfig) {
		c.attributes = append(c.attributes, fields...)
	})
}

// NewSpanConfig creates a span configuration from options (exported for provider implementations).
func NewSpanConfig(opts []SpanOption) SpanConfig {
	cfg := &spanConfig{
		kind:       SpanKindInternal,
		attributes: make([]Field, 0),
	}
	for _, opt := range opts {
		opt.apply(cfg)
	}
	return cfg
}

// SpanConfig provides access to span configuration (for provider implementations).
type SpanConfig interface {
	Kind() SpanKind
	Attributes() []Field
}

func (c *spanConfig) Kind() SpanKind {
	return c.kind
}

func (c *spanConfig) Attributes() []Field {
	return c.attributes
}
