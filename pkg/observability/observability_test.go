package observability_test

import (
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func TestStringField(t *testing.T) {
	field := observability.String("key", "value")

	if field.Key != "key" {
		t.Errorf("got key %q, want %q", field.Key, "key")
	}

	value, ok := field.Value.(string)
	if !ok {
		t.Fatalf("field.Value is not a string, got %T", field.Value)
	}

	if value != "value" {
		t.Errorf("got value %q, want %q", value, "value")
	}
}

func TestIntField(t *testing.T) {
	field := observability.Int("count", 42)

	if field.Key != "count" {
		t.Errorf("got key %q, want %q", field.Key, "count")
	}

	value, ok := field.Value.(int)
	if !ok {
		t.Fatalf("field.Value is not an int, got %T", field.Value)
	}

	if value != 42 {
		t.Errorf("got value %d, want %d", value, 42)
	}
}

func TestInt64Field(t *testing.T) {
	field := observability.Int64("count", 9223372036854775807)

	if field.Key != "count" {
		t.Errorf("got key %q, want %q", field.Key, "count")
	}

	value, ok := field.Value.(int64)
	if !ok {
		t.Fatalf("field.Value is not an int64, got %T", field.Value)
	}

	if value != 9223372036854775807 {
		t.Errorf("got value %d, want %d", value, 9223372036854775807)
	}
}

func TestFloat64Field(t *testing.T) {
	field := observability.Float64("latency", 3.14159)

	if field.Key != "latency" {
		t.Errorf("got key %q, want %q", field.Key, "latency")
	}

	value, ok := field.Value.(float64)
	if !ok {
		t.Fatalf("field.Value is not a float64, got %T", field.Value)
	}

	if value != 3.14159 {
		t.Errorf("got value %f, want %f", value, 3.14159)
	}
}

func TestBoolField(t *testing.T) {
	field := observability.Bool("success", true)

	if field.Key != "success" {
		t.Errorf("got key %q, want %q", field.Key, "success")
	}

	value, ok := field.Value.(bool)
	if !ok {
		t.Fatalf("field.Value is not a bool, got %T", field.Value)
	}

	if value != true {
		t.Errorf("got value %v, want %v", value, true)
	}
}

func TestErrorField(t *testing.T) {
	testErr := errors.New("test error")
	field := observability.Error(testErr)

	if field.Key != "error" {
		t.Errorf("got key %q, want %q", field.Key, "error")
	}

	value, ok := field.Value.(error)
	if !ok {
		t.Fatalf("field.Value is not an error, got %T", field.Value)
	}

	if value.Error() != "test error" {
		t.Errorf("got error %q, want %q", value.Error(), "test error")
	}
}

func TestAnyField(t *testing.T) {
	type customStruct struct {
		Name string
		Age  int
	}

	custom := customStruct{Name: "John", Age: 30}
	field := observability.Any("custom", custom)

	if field.Key != "custom" {
		t.Errorf("got key %q, want %q", field.Key, "custom")
	}

	value, ok := field.Value.(customStruct)
	if !ok {
		t.Fatalf("field.Value is not customStruct, got %T", field.Value)
	}

	if value.Name != "John" || value.Age != 30 {
		t.Errorf("got value %+v, want {Name:John Age:30}", value)
	}
}

func TestFieldHelpers(t *testing.T) {
	tests := []struct {
		name      string
		field     observability.Field
		wantKey   string
		wantValue interface{}
	}{
		{
			name:      "String field",
			field:     observability.String("name", "test"),
			wantKey:   "name",
			wantValue: "test",
		},
		{
			name:      "Int field",
			field:     observability.Int("count", 10),
			wantKey:   "count",
			wantValue: 10,
		},
		{
			name:      "Int64 field",
			field:     observability.Int64("big_count", 1000000),
			wantKey:   "big_count",
			wantValue: int64(1000000),
		},
		{
			name:      "Float64 field",
			field:     observability.Float64("price", 99.99),
			wantKey:   "price",
			wantValue: 99.99,
		},
		{
			name:      "Bool field",
			field:     observability.Bool("enabled", false),
			wantKey:   "enabled",
			wantValue: false,
		},
		{
			name:      "Error field",
			field:     observability.Error(errors.New("oops")),
			wantKey:   "error",
			wantValue: errors.New("oops"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.wantKey {
				t.Errorf("got key %q, want %q", tt.field.Key, tt.wantKey)
			}

			// For error type, compare error messages
			if wantErr, ok := tt.wantValue.(error); ok {
				gotErr, ok := tt.field.Value.(error)
				if !ok {
					t.Fatalf("field.Value is not an error, got %T", tt.field.Value)
				}
				if gotErr.Error() != wantErr.Error() {
					t.Errorf("got error %q, want %q", gotErr.Error(), wantErr.Error())
				}
				return
			}

			if tt.field.Value != tt.wantValue {
				t.Errorf("got value %v, want %v", tt.field.Value, tt.wantValue)
			}
		})
	}
}
