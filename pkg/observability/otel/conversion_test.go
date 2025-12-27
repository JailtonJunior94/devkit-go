package otel

import (
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

func TestConvertFieldToAttribute(t *testing.T) {
	tests := []struct {
		name  string
		field observability.Field
		want  attribute.KeyValue
	}{
		{
			name:  "string value",
			field: observability.String("key", "value"),
			want:  attribute.String("key", "value"),
		},
		{
			name:  "int value",
			field: observability.Int("count", 42),
			want:  attribute.Int("count", 42),
		},
		{
			name:  "int64 value",
			field: observability.Int64("big_count", 9223372036854775807),
			want:  attribute.Int64("big_count", 9223372036854775807),
		},
		{
			name:  "float64 value",
			field: observability.Float64("price", 99.99),
			want:  attribute.Float64("price", 99.99),
		},
		{
			name:  "bool value true",
			field: observability.Bool("enabled", true),
			want:  attribute.Bool("enabled", true),
		},
		{
			name:  "bool value false",
			field: observability.Bool("disabled", false),
			want:  attribute.Bool("disabled", false),
		},
		{
			name:  "error value",
			field: observability.Error(errors.New("test error")),
			want:  attribute.String("error", "test error"),
		},
		{
			name:  "custom type falls back to string",
			field: observability.Any("custom", struct{ Name string }{Name: "test"}),
			want:  attribute.String("custom", "{test}"),
		},
		{
			name:  "nil value",
			field: observability.Any("nil_value", nil),
			want:  attribute.String("nil_value", "<nil>"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertFieldToAttribute(tt.field)

			if got.Key != tt.want.Key {
				t.Errorf("got key %v, want %v", got.Key, tt.want.Key)
			}

			if got.Value.AsInterface() != tt.want.Value.AsInterface() {
				t.Errorf("got value %v, want %v", got.Value.AsInterface(), tt.want.Value.AsInterface())
			}
		})
	}
}

func TestConvertFieldsToAttributes(t *testing.T) {
	t.Run("empty slice returns nil", func(t *testing.T) {
		result := convertFieldsToAttributes([]observability.Field{})

		if result != nil {
			t.Errorf("expected nil for empty slice, got %v", result)
		}
	})

	t.Run("nil slice returns nil", func(t *testing.T) {
		result := convertFieldsToAttributes(nil)

		if result != nil {
			t.Errorf("expected nil for nil slice, got %v", result)
		}
	})

	t.Run("single field", func(t *testing.T) {
		fields := []observability.Field{
			observability.String("key", "value"),
		}

		result := convertFieldsToAttributes(fields)

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 attribute, got %d", len(result))
		}

		if result[0].Key != "key" {
			t.Errorf("got key %v, want %v", result[0].Key, "key")
		}

		if result[0].Value.AsString() != "value" {
			t.Errorf("got value %v, want %v", result[0].Value.AsString(), "value")
		}
	})

	t.Run("multiple fields", func(t *testing.T) {
		fields := []observability.Field{
			observability.String("name", "test"),
			observability.Int("count", 10),
			observability.Bool("enabled", true),
		}

		result := convertFieldsToAttributes(fields)

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if len(result) != 3 {
			t.Fatalf("expected 3 attributes, got %d", len(result))
		}

		// Verify first attribute
		if result[0].Key != "name" || result[0].Value.AsString() != "test" {
			t.Errorf("first attribute incorrect: key=%v, value=%v", result[0].Key, result[0].Value.AsInterface())
		}

		// Verify second attribute
		if result[1].Key != "count" || result[1].Value.AsInt64() != 10 {
			t.Errorf("second attribute incorrect: key=%v, value=%v", result[1].Key, result[1].Value.AsInterface())
		}

		// Verify third attribute
		if result[2].Key != "enabled" || result[2].Value.AsBool() != true {
			t.Errorf("third attribute incorrect: key=%v, value=%v", result[2].Key, result[2].Value.AsInterface())
		}
	})

	t.Run("mixed types", func(t *testing.T) {
		fields := []observability.Field{
			observability.String("str", "text"),
			observability.Int("int", 42),
			observability.Int64("int64", 1000000),
			observability.Float64("float", 3.14),
			observability.Bool("bool", false),
			observability.Error(errors.New("error message")),
		}

		result := convertFieldsToAttributes(fields)

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		if len(result) != 6 {
			t.Fatalf("expected 6 attributes, got %d", len(result))
		}

		// Just verify we got the right number of conversions
		// Individual conversion correctness is tested in TestConvertFieldToAttribute
	})
}

func TestConvertFieldsToAttributesNoAllocation(t *testing.T) {
	// This test verifies that we don't allocate for empty slices
	var fields []observability.Field

	result := convertFieldsToAttributes(fields)

	if result != nil {
		t.Errorf("expected nil result for zero allocation, got %v", result)
	}
}

func BenchmarkConvertFieldToAttribute(b *testing.B) {
	field := observability.String("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertFieldToAttribute(field)
	}
}

func BenchmarkConvertFieldsToAttributes(b *testing.B) {
	fields := []observability.Field{
		observability.String("name", "test"),
		observability.Int("count", 42),
		observability.Float64("latency", 1.23),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertFieldsToAttributes(fields)
	}
}

func BenchmarkConvertFieldsToAttributesEmpty(b *testing.B) {
	var fields []observability.Field

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertFieldsToAttributes(fields)
	}
}
