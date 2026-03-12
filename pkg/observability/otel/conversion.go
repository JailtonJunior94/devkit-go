package otel

import (
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

// otelAttrPool pools slices of attribute.KeyValue to reduce per-call GC pressure.
// Slices are reset to length 0 but retain their capacity between reuses.
// Capacity is 16 to match slogAttrPool and avoid reallocation for typical field counts.
var otelAttrPool = sync.Pool{
	New: func() any {
		s := make([]attribute.KeyValue, 0, 16)
		return &s
	},
}

// acquireAttrs returns a pooled *[]attribute.KeyValue with length reset to 0.
// Always pair with releaseAttrs.
func acquireAttrs() *[]attribute.KeyValue {
	return otelAttrPool.Get().(*[]attribute.KeyValue)
}

// releaseAttrs returns a slice to the pool.
// Set *p = attrs before calling so the pool retains any reallocation from append.
func releaseAttrs(p *[]attribute.KeyValue) {
	*p = (*p)[:0]
	otelAttrPool.Put(p)
}

// convertFieldToAttribute converts an observability.Field to an OpenTelemetry attribute.
// Uses the Field discriminated union (Kind + typed accessors) — zero boxing for common types.
func convertFieldToAttribute(field observability.Field) attribute.KeyValue {
	switch field.Kind() {
	case observability.FieldKindString:
		return attribute.String(field.Key, field.StringValue())
	case observability.FieldKindInt:
		return attribute.Int(field.Key, int(field.Int64Value()))
	case observability.FieldKindInt64:
		return attribute.Int64(field.Key, field.Int64Value())
	case observability.FieldKindFloat64:
		return attribute.Float64(field.Key, field.Float64Value())
	case observability.FieldKindBool:
		return attribute.Bool(field.Key, field.BoolValue())
	case observability.FieldKindError:
		if err, ok := field.AnyValue().(error); ok {
			return attribute.String(field.Key, err.Error())
		}
		return attribute.String(field.Key, "")
	default:
		return attribute.String(field.Key, fmt.Sprintf("%v", field.AnyValue()))
	}
}

// appendFieldAttrs appends OTel attribute conversions of fields to dst and returns the result.
// Use with acquireAttrs/releaseAttrs in hot paths to avoid per-call heap allocation.
func appendFieldAttrs(dst []attribute.KeyValue, fields []observability.Field) []attribute.KeyValue {
	for _, f := range fields {
		dst = append(dst, convertFieldToAttribute(f))
	}
	return dst
}

// convertFieldsToAttributes converts multiple observability.Field to OpenTelemetry attributes.
// Returns nil for empty slices. Prefer acquireAttrs/appendFieldAttrs/releaseAttrs in hot paths.
func convertFieldsToAttributes(fields []observability.Field) []attribute.KeyValue {
	if len(fields) == 0 {
		return nil
	}
	attrs := make([]attribute.KeyValue, len(fields))
	for i, field := range fields {
		attrs[i] = convertFieldToAttribute(field)
	}
	return attrs
}
