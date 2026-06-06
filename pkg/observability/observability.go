package observability

import (
	"context"
	"math"
)

type Observability interface {
	Tracer() Tracer
	Logger() Logger
	Metrics() Metrics
	Shutdown(ctx context.Context) error
}

type FieldKind uint8

const (
	FieldKindString FieldKind = iota
	FieldKindInt
	FieldKindInt64
	FieldKindFloat64
	FieldKindBool
	FieldKindError
	FieldKindAny
)

type Field struct {
	Key    string
	numVal uint64
	strVal string
	anyVal any
	kind   FieldKind
}

func (f Field) Kind() FieldKind       { return f.kind }
func (f Field) StringValue() string   { return f.strVal }
func (f Field) Int64Value() int64     { return int64(f.numVal) }
func (f Field) Float64Value() float64 { return math.Float64frombits(f.numVal) }
func (f Field) BoolValue() bool       { return f.numVal != 0 }

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

func String(key, value string) Field {
	return Field{Key: key, strVal: value, kind: FieldKindString}
}

func Int(key string, value int) Field {
	return Field{Key: key, numVal: uint64(int64(value)), kind: FieldKindInt}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, numVal: uint64(value), kind: FieldKindInt64}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, numVal: math.Float64bits(value), kind: FieldKindFloat64}
}

func Bool(key string, value bool) Field {
	var n uint64
	if value {
		n = 1
	}
	return Field{Key: key, numVal: n, kind: FieldKindBool}
}

func Error(err error) Field {
	return Field{Key: "error", anyVal: err, kind: FieldKindError}
}

func Any(key string, value any) Field {
	return Field{Key: key, anyVal: value, kind: FieldKindAny}
}

type SpanContext interface {
	TraceID() string
	SpanID() string
	IsSampled() bool
}

type Span interface {
	End()
	SetAttributes(fields ...Field)
	SetStatus(code StatusCode, description string)
	RecordError(err error, fields ...Field)
	AddEvent(name string, fields ...Field)
	Context() SpanContext
	TraceID() string
	SpanID() string
	IsSampled() bool
}

type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

type SpanConfig struct {
	kind       SpanKind
	attributes []Field
}

func (c SpanConfig) Kind() SpanKind      { return c.kind }
func (c SpanConfig) Attributes() []Field { return c.attributes }

type SpanOption interface {
	apply(*SpanConfig)
}

type spanKindOpt struct{ kind SpanKind }

func (o spanKindOpt) apply(c *SpanConfig) { c.kind = o.kind }

type spanAttrsOpt struct{ attrs []Field }

func (o spanAttrsOpt) apply(c *SpanConfig) { c.attributes = append(c.attributes, o.attrs...) }

func WithSpanKind(kind SpanKind) SpanOption { return spanKindOpt{kind: kind} }

func WithAttributes(fields ...Field) SpanOption { return spanAttrsOpt{attrs: fields} }

func ApplySpanOptions(cfg *SpanConfig, opts []SpanOption) {
	cfg.kind = SpanKindInternal
	for _, opt := range opts {
		opt.apply(cfg)
	}
}

func NewSpanConfig(opts []SpanOption) SpanConfig {
	var cfg SpanConfig
	ApplySpanOptions(&cfg, opts)
	return cfg
}
