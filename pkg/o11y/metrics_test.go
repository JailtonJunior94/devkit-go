package o11y

import (
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

func TestMetrics_ParseLabels_CardinalityProtection(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 3,
		maxLabelValueLen:   10,
	}

	// Test with more labels than allowed
	labels := []any{
		"key1", "value1",
		"key2", "value2",
		"key3", "value3",
		"key4", "value4", // Should be truncated
		"key5", "value5", // Should be truncated
	}

	result := m.parseLabels(labels...)

	if len(result) != 3 {
		t.Errorf("expected 3 labels, got %d", len(result))
	}
}

func TestMetrics_ParseLabels_ValueTruncation(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   10,
	}

	labels := []any{
		"key1", "this is a very long value that should be truncated",
	}

	result := m.parseLabels(labels...)

	if len(result) != 1 {
		t.Fatalf("expected 1 label, got %d", len(result))
	}

	// Check that value was truncated
	strVal := result[0].Value.AsString()
	if len(strVal) > 10 {
		t.Errorf("expected value to be truncated to 10 chars, got %d chars: %q", len(strVal), strVal)
	}
	if strVal != "this is..." {
		t.Errorf("expected truncated value to be %q, got %q", "this is...", strVal)
	}
}

func TestMetrics_ParseLabels_OddNumberOfLabels(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
	}

	// Odd number of labels - last key should be ignored
	labels := []any{"key1", "value1", "key2"}

	result := m.parseLabels(labels...)

	if len(result) != 1 {
		t.Errorf("expected 1 label (odd number), got %d", len(result))
	}
}

func TestMetrics_ParseLabels_TypeConversion(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
	}

	labels := []any{
		"string_key", "string_value",
		"int_key", 42,
		"int64_key", int64(64),
		"float_key", 3.14,
		"bool_key", true,
		"nil_key", nil,
	}

	result := m.parseLabels(labels...)

	if len(result) != 6 {
		t.Errorf("expected 6 labels, got %d", len(result))
	}

	// Verify types
	expectedTypes := map[string]attribute.Type{
		"string_key": attribute.STRING,
		"int_key":    attribute.INT64,
		"int64_key":  attribute.INT64,
		"float_key":  attribute.FLOAT64,
		"bool_key":   attribute.BOOL,
		"nil_key":    attribute.STRING, // nil becomes string "<nil>"
	}

	for _, kv := range result {
		expectedType, ok := expectedTypes[string(kv.Key)]
		if !ok {
			t.Errorf("unexpected key: %s", kv.Key)
			continue
		}
		if kv.Value.Type() != expectedType {
			t.Errorf("key %s: expected type %v, got %v", kv.Key, expectedType, kv.Value.Type())
		}
	}
}

func TestMetrics_HandleError_WithCallback(t *testing.T) {
	var callbackCalled bool
	var receivedError error

	m := &metrics{
		counters:           make(map[string]otelmetric.Int64Counter),
		histograms:         make(map[string]otelmetric.Float64Histogram),
		maxInstruments:     1000,
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		onError: func(err error) {
			callbackCalled = true
			receivedError = err
		},
	}

	testErr := &testError{msg: "test error"}
	m.handleError(testErr)

	if !callbackCalled {
		t.Error("error callback was not called")
	}

	if receivedError != testErr {
		t.Errorf("expected error %v, got %v", testErr, receivedError)
	}
}

func TestMetrics_HandleError_WithoutCallback(t *testing.T) {
	m := &metrics{
		counters:           make(map[string]otelmetric.Int64Counter),
		histograms:         make(map[string]otelmetric.Float64Histogram),
		maxInstruments:     1000,
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		onError:            nil, // No callback
	}

	// Should not panic when callback is nil
	m.handleError(&testError{msg: "test error"})
}

func TestMetrics_TruncateString(t *testing.T) {
	m := &metrics{}

	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"hello world", 5, "he..."},
	}

	for _, tt := range tests {
		result := m.truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestMetrics_CreateAttribute(t *testing.T) {
	m := &metrics{
		maxLabelValueLen: 20,
	}

	tests := []struct {
		key      string
		value    any
		expected attribute.Type
	}{
		{"string_key", "value", attribute.STRING},
		{"int_key", 42, attribute.INT64},
		{"int64_key", int64(64), attribute.INT64},
		{"float_key", 3.14, attribute.FLOAT64},
		{"bool_key", true, attribute.BOOL},
		{"nil_key", nil, attribute.STRING},
	}

	for _, tt := range tests {
		result := m.createAttribute(tt.key, tt.value)
		if result.Value.Type() != tt.expected {
			t.Errorf("createAttribute(%q, %v): expected type %v, got %v", tt.key, tt.value, tt.expected, result.Value.Type())
		}
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestMetrics_TruncateString_EdgeCases(t *testing.T) {
	m := &metrics{}

	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		// Edge case: maxLen <= 0
		{"hello", 0, ""},
		{"hello", -1, ""},
		// Edge case: maxLen < 4 (no room for ellipsis)
		{"hello", 1, "h"},
		{"hello", 2, "he"},
		{"hello", 3, "hel"},
		// Normal truncation
		{"hello", 4, "h..."},
		{"hello", 5, "hello"},
	}

	for _, tt := range tests {
		result := m.truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestMetrics_SensitiveKeyRedaction(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		sensitiveKeys:      DefaultSensitiveKeys,
		redactSensitive:    true,
	}

	tests := []struct {
		key       string
		value     any
		expectVal string
	}{
		{"password", "secret123", redactedValue},
		{"api_key", "sk-abc123", redactedValue},
		{"user_token", "token123", redactedValue},
		{"authorization", "Bearer xyz", redactedValue},
		{"normal_field", "visible", "visible"},
		{"user_id", "12345", "12345"},
	}

	for _, tt := range tests {
		result := m.createAttribute(tt.key, tt.value)
		strVal := result.Value.AsString()
		if strVal != tt.expectVal {
			t.Errorf("createAttribute(%q, %v) = %q, expected %q", tt.key, tt.value, strVal, tt.expectVal)
		}
	}
}

func TestMetrics_SensitiveKeyRedaction_Disabled(t *testing.T) {
	m := &metrics{
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		sensitiveKeys:      DefaultSensitiveKeys,
		redactSensitive:    false, // Disabled
	}

	// When redaction is disabled, sensitive values should be visible
	result := m.createAttribute("password", "secret123")
	strVal := result.Value.AsString()
	if strVal != "secret123" {
		t.Errorf("expected value to be visible when redaction disabled, got %q", strVal)
	}
}

func TestMetrics_ConcurrentParseLabels(t *testing.T) {
	m := &metrics{
		counters:           make(map[string]otelmetric.Int64Counter),
		histograms:         make(map[string]otelmetric.Float64Histogram),
		maxInstruments:     1000,
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		sensitiveKeys:      DefaultSensitiveKeys,
		redactSensitive:    true,
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				labels := []any{
					"key1", "value1",
					"key2", idx,
					"key3", float64(j),
				}
				_ = m.parseLabels(labels...)
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race condition or panic occurred
}

func TestMetrics_ConcurrentAttributeCreation(t *testing.T) {
	m := &metrics{
		counters:           make(map[string]otelmetric.Int64Counter),
		histograms:         make(map[string]otelmetric.Float64Histogram),
		maxInstruments:     1000,
		maxLabelsPerMetric: 10,
		maxLabelValueLen:   256,
		sensitiveKeys:      DefaultSensitiveKeys,
		redactSensitive:    true,
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent attribute creation (no meter needed)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.createAttribute("key", "value")
				_ = m.createAttribute("password", "secret")
				_ = m.isSensitiveKey("api_key")
				_ = m.truncateString("this is a long string that needs truncation", 20)
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race condition or panic occurred
}
