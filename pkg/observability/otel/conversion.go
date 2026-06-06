package otel

import (
	"fmt"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

var otelAttrPool = sync.Pool{
	New: func() any {
		s := make([]attribute.KeyValue, 0, 16)
		return &s
	},
}

func acquireAttrs() *[]attribute.KeyValue {
	return otelAttrPool.Get().(*[]attribute.KeyValue)
}

func releaseAttrs(p *[]attribute.KeyValue) {
	*p = (*p)[:0]
	otelAttrPool.Put(p)
}

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

func appendFieldAttrs(dst []attribute.KeyValue, fields []observability.Field) []attribute.KeyValue {
	for _, f := range fields {
		dst = append(dst, convertFieldToAttribute(f))
	}
	return dst
}

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
