package o11y

import (
	"reflect"
	"testing"
)

func TestAttr(t *testing.T) {
	attr := Attr("key", "value")
	if attr.Key != "key" {
		t.Errorf("expected key key, got %v", attr.Key)
	}
	if attr.Value != "value" {
		t.Errorf("expected value value, got %v", attr.Value)
	}
}

func TestLogField(t *testing.T) {
	field := LogField("key", 42)
	if field.Key != "key" {
		t.Errorf("expected key key, got %v", field.Key)
	}
	if field.Value != 42 {
		t.Errorf("expected value 42, got %v", field.Value)
	}
}

func TestAttr_DifferentTypes(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"string", "key", "value"},
		{"int", "key", 42},
		{"int64", "key", int64(42)},
		{"float64", "key", 3.14},
		{"bool", "key", true},
		{"slice", "key", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := Attr(tt.key, tt.value)
			if attr.Key != tt.key {
				t.Errorf("expected key %v, got %v", tt.key, attr.Key)
			}
			if !reflect.DeepEqual(attr.Value, tt.value) {
				t.Errorf("expected value %v, got %v", tt.value, attr.Value)
			}
		})
	}
}

func TestLogField_DifferentTypes(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"string", "key", "value"},
		{"int", "key", 42},
		{"int64", "key", int64(42)},
		{"float64", "key", 3.14},
		{"bool", "key", true},
		{"struct", "key", struct{ X int }{X: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := LogField(tt.key, tt.value)
			if field.Key != tt.key {
				t.Errorf("expected key %v, got %v", tt.key, field.Key)
			}
			if !reflect.DeepEqual(field.Value, tt.value) {
				t.Errorf("expected value %v, got %v", tt.value, field.Value)
			}
		})
	}
}
