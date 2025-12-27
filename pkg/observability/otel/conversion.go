package otel

import (
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

// convertFieldToAttribute converts an observability.Field to an OpenTelemetry attribute.
// This centralizes the conversion logic used by tracer, metrics, and logger.
func convertFieldToAttribute(field observability.Field) attribute.KeyValue {
	switch v := field.Value.(type) {
	case string:
		return attribute.String(field.Key, v)
	case int:
		return attribute.Int(field.Key, v)
	case int64:
		return attribute.Int64(field.Key, v)
	case float64:
		return attribute.Float64(field.Key, v)
	case bool:
		return attribute.Bool(field.Key, v)
	case error:
		return attribute.String(field.Key, v.Error())
	default:
		return attribute.String(field.Key, fmt.Sprintf("%v", v))
	}
}

// convertFieldsToAttributes converts multiple observability.Field to OpenTelemetry attributes.
// Returns nil for empty slices to avoid unnecessary allocations.
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
