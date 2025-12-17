package o11y

import (
	"context"
	"testing"
	"time"
)

func TestParseLabels_Empty(t *testing.T) {
	labels := parseLabels()
	if labels != nil {
		t.Errorf("expected nil, got %v", labels)
	}
}

func TestParseLabels_ValidPairs(t *testing.T) {
	labels := parseLabels("key1", "value1", "key2", 42, "key3", true)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
}

func TestParseLabels_InvalidKey(t *testing.T) {
	// Non-string key should be skipped
	labels := parseLabels(123, "value", "valid_key", "valid_value")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if string(labels[0].Key) != "valid_key" {
		t.Errorf("expected key valid_key, got %v", labels[0].Key)
	}
}

func TestParseLabels_OddLength(t *testing.T) {
	// Odd number of arguments should skip the last unpaired element
	labels := parseLabels("key1", "value1", "key2")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
}

func TestParseLabels_AllTypes(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  any
		expect string
	}{
		{"string", "k", "v", "STRING"},
		{"int", "k", 42, "INT64"},
		{"int64", "k", int64(42), "INT64"},
		{"int32", "k", int32(42), "INT64"},
		{"int16", "k", int16(42), "INT64"},
		{"int8", "k", int8(42), "INT64"},
		{"uint", "k", uint(42), "INT64"},
		{"uint64", "k", uint64(42), "INT64"},
		{"uint32", "k", uint32(42), "INT64"},
		{"uint16", "k", uint16(42), "INT64"},
		{"uint8", "k", uint8(42), "INT64"},
		{"bool", "k", true, "BOOL"},
		{"float64", "k", 3.14, "FLOAT64"},
		{"float32", "k", float32(3.14), "FLOAT64"},
		{"[]string", "k", []string{"a"}, "STRINGSLICE"},
		{"[]int", "k", []int{1}, "INT64SLICE"},
		{"[]int64", "k", []int64{1}, "INT64SLICE"},
		{"[]float64", "k", []float64{1.1}, "FLOAT64SLICE"},
		{"[]bool", "k", []bool{true}, "BOOLSLICE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := parseLabels(tt.key, tt.value)
			if len(labels) != 1 {
				t.Fatalf("expected 1 label, got %d", len(labels))
			}
			if labels[0].Value.Type().String() != tt.expect {
				t.Errorf("expected type %s, got %s", tt.expect, labels[0].Value.Type().String())
			}
		})
	}
}

func TestNoOpMetrics_AllMethods(t *testing.T) {
	m := NewNoOpMetrics()
	ctx := context.Background()

	// These should not panic
	m.AddCounter(ctx, "counter", 1, "key", "value")
	m.RecordHistogram(ctx, "histogram", 1.5, "key", "value")
	m.SetGauge(ctx, "gauge", 2.5, "key", "value")
	m.AddUpDownCounter(ctx, "updown", -1, "key", "value")
	m.RecordDuration(ctx, "duration", time.Now(), "key", "value")
}
