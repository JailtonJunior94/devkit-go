package o11y

import (
	"context"
	"fmt"
	"testing"
)

func TestConvertAttr_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		attr     Attribute
		wantType string
	}{
		{"string", Attribute{Key: "k", Value: "v"}, "STRING"},
		{"int", Attribute{Key: "k", Value: 42}, "INT64"},
		{"int64", Attribute{Key: "k", Value: int64(42)}, "INT64"},
		{"int32", Attribute{Key: "k", Value: int32(42)}, "INT64"},
		{"int16", Attribute{Key: "k", Value: int16(42)}, "INT64"},
		{"int8", Attribute{Key: "k", Value: int8(42)}, "INT64"},
		{"uint", Attribute{Key: "k", Value: uint(42)}, "INT64"},
		{"uint64", Attribute{Key: "k", Value: uint64(42)}, "INT64"},
		{"uint32", Attribute{Key: "k", Value: uint32(42)}, "INT64"},
		{"uint16", Attribute{Key: "k", Value: uint16(42)}, "INT64"},
		{"uint8", Attribute{Key: "k", Value: uint8(42)}, "INT64"},
		{"bool", Attribute{Key: "k", Value: true}, "BOOL"},
		{"float64", Attribute{Key: "k", Value: 3.14}, "FLOAT64"},
		{"float32", Attribute{Key: "k", Value: float32(3.14)}, "FLOAT64"},
		{"[]string", Attribute{Key: "k", Value: []string{"a", "b"}}, "STRINGSLICE"},
		{"[]int", Attribute{Key: "k", Value: []int{1, 2}}, "INT64SLICE"},
		{"[]int64", Attribute{Key: "k", Value: []int64{1, 2}}, "INT64SLICE"},
		{"[]float64", Attribute{Key: "k", Value: []float64{1.1, 2.2}}, "FLOAT64SLICE"},
		{"[]bool", Attribute{Key: "k", Value: []bool{true, false}}, "BOOLSLICE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv, ok := convertAttr(tt.attr)
			if !ok {
				t.Fatal("expected conversion to succeed")
			}
			if kv.Key != "k" {
				t.Errorf("expected key k, got %v", kv.Key)
			}
			if kv.Value.Type().String() != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, kv.Value.Type().String())
			}
		})
	}
}

type testStringer struct {
	value string
}

func (s testStringer) String() string {
	return s.value
}

func TestConvertAttr_Stringer(t *testing.T) {
	attr := Attribute{Key: "k", Value: testStringer{value: "test"}}
	kv, ok := convertAttr(attr)
	if !ok {
		t.Fatal("expected conversion to succeed")
	}
	if kv.Value.AsString() != "test" {
		t.Errorf("expected test, got %v", kv.Value.AsString())
	}
}

func TestConvertAttr_Uint64Overflow(t *testing.T) {
	// Test uint64 value greater than math.MaxInt64
	var maxUint64 uint64 = 18446744073709551615 // max uint64
	attr := Attribute{Key: "big_number", Value: maxUint64}
	kv, ok := convertAttr(attr)
	if !ok {
		t.Fatal("expected conversion to succeed")
	}
	// Should be converted to string to avoid overflow
	if kv.Value.Type().String() != "STRING" {
		t.Errorf("expected STRING type for overflow, got %v", kv.Value.Type().String())
	}
}

func TestConvertAttr_Uint64NoOverflow(t *testing.T) {
	// Test uint64 value that fits in int64
	var smallUint64 uint64 = 42
	attr := Attribute{Key: "small_number", Value: smallUint64}
	kv, ok := convertAttr(attr)
	if !ok {
		t.Fatal("expected conversion to succeed")
	}
	// Should remain as INT64
	if kv.Value.Type().String() != "INT64" {
		t.Errorf("expected INT64 type, got %v", kv.Value.Type().String())
	}
}

func TestConvertAttr_Unknown(t *testing.T) {
	attr := Attribute{Key: "k", Value: struct{ X int }{X: 1}}
	kv, ok := convertAttr(attr)
	if !ok {
		t.Fatal("expected conversion to succeed")
	}
	// Unknown types are converted to string via fmt.Sprintf
	if kv.Value.Type().String() != "STRING" {
		t.Errorf("expected STRING type, got %s", kv.Value.Type().String())
	}
}

func TestConvertAttrs_Multiple(t *testing.T) {
	attrs := []Attribute{
		{Key: "str", Value: "value"},
		{Key: "num", Value: 42},
		{Key: "flag", Value: true},
	}

	kvs := convertAttrs(attrs)
	if len(kvs) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(kvs))
	}
}

func TestTraceIDFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	traceID := TraceIDFromContext(ctx)
	if traceID != "" {
		t.Errorf("expected empty trace ID, got %v", traceID)
	}
}

func TestSpanIDFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	spanID := SpanIDFromContext(ctx)
	if spanID != "" {
		t.Errorf("expected empty span ID, got %v", spanID)
	}
}

func TestNoOpTracer_Start(t *testing.T) {
	tracer := NewNoOpTracer()
	ctx, span := tracer.Start(context.Background(), "test")
	if ctx == nil {
		t.Fatal("expected context, got nil")
	}
	if span == nil {
		t.Fatal("expected span, got nil")
	}

	// These should not panic
	span.SetAttributes(Attr("key", "value"))
	span.AddEvent("event", Attr("key", "value"))
	span.SetStatus(SpanStatusOk, "ok")
	span.RecordError(fmt.Errorf("test error"))
	span.End()
}

func TestNoOpTracer_WithAttributes(t *testing.T) {
	tracer := NewNoOpTracer()
	// Should not panic
	tracer.WithAttributes(context.Background(), Attr("key", "value"))
}
