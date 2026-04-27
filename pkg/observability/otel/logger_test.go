package otel

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key       string
		sensitive bool
	}{
		// Sensitive keys (should be redacted)
		{"password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"api_key", true},
		{"API_KEY", true},
		{"apikey", true},
		{"token", true},
		{"access_token", true},
		{"refresh_token", true},
		{"authorization", true},
		{"Authorization", true},
		{"bearer", true},
		{"credit_card", true},
		{"creditcard", true},
		{"ssn", true},
		{"secret", true},
		{"my_secret_key", true},
		{"credential", true},
		{"credentials", true},
		{"private_key", true},
		{"session", true},
		{"cookie", true},

		// Non-sensitive keys (should NOT be redacted)
		{"username", false},
		{"email", false},
		{"name", false},
		{"id", false},
		{"user_id", false},
		{"status", false},
		{"timestamp", false},
		{"count", false},
		{"message", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			assert.Equal(t, tt.sensitive, result, "Key: %s", tt.key)
		})
	}
}

func TestSanitizeFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   []observability.Field
		expected []observability.Field
	}{
		{
			name: "redact password",
			fields: []observability.Field{
				observability.String("username", "john"),
				observability.String("password", "secret123"),
			},
			expected: []observability.Field{
				observability.String("username", "john"),
				observability.String("password", redactedValue),
			},
		},
		{
			name: "redact multiple sensitive fields",
			fields: []observability.Field{
				observability.String("api_key", "sk_live_123"),
				observability.String("user_id", "123"),
				observability.String("token", "xyz"),
			},
			expected: []observability.Field{
				observability.String("api_key", redactedValue),
				observability.String("user_id", "123"),
				observability.String("token", redactedValue),
			},
		},
		{
			name: "truncate long string",
			fields: []observability.Field{
				observability.String("data", strings.Repeat("a", maxFieldValueLength+100)),
			},
			expected: []observability.Field{
				observability.String("data", strings.Repeat("a", maxFieldValueLength)+"...[truncated]"),
			},
		},
		{
			name:     "limit number of fields",
			fields:   make([]observability.Field, maxFields+10),
			expected: make([]observability.Field, maxFields),
		},
		{
			name: "preserve non-string types",
			fields: []observability.Field{
				observability.Int("count", 42),
				observability.Bool("success", true),
				observability.Float64("latency", 0.123),
			},
			expected: []observability.Field{
				observability.Int("count", 42),
				observability.Bool("success", true),
				observability.Float64("latency", 0.123),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFields(tt.fields)

			if tt.name == "limit number of fields" {
				assert.Len(t, result, maxFields)
				return
			}

			assert.Len(t, result, len(tt.expected))

			for i := range result {
				assert.Equal(t, tt.expected[i].Key, result[i].Key)

				// For redacted fields, check value is redactedValue
				if isSensitiveKey(tt.expected[i].Key) {
					assert.Equal(t, redactedValue, result[i].AnyValue())
				} else if str, ok := tt.expected[i].AnyValue().(string); ok && len(str) > maxFieldValueLength {
					// For truncated fields
					assert.Contains(t, result[i].AnyValue().(string), "...[truncated]")
				} else {
					// For normal fields
					assert.Equal(t, tt.expected[i].AnyValue(), result[i].AnyValue())
				}
			}
		})
	}
}

func TestFormatTraceIDAndSpanIDStackEncoding(t *testing.T) {
	t.Parallel()

	traceID := trace.TraceID{
		0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6,
		0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36,
	}
	spanID := trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}

	assert.Equal(t, traceID.String(), formatTraceID(traceID))
	assert.Equal(t, spanID.String(), formatSpanID(spanID))
	assert.Len(t, formatTraceID(traceID), 32)
	assert.Len(t, formatSpanID(spanID), 16)
}

func TestAppendCorrelationAttrsUsesSpanContext(t *testing.T) {
	t.Parallel()

	exporter := &memoryLogExporter{}
	loggerProvider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	logger := newOtelLogger(
		observability.LogLevelInfo,
		observability.LogFormatJSON,
		"orders-api",
		"test",
		loggerProvider,
		false,
		false,
	)

	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	ctx, span := tracerProvider.Tracer("hot-path-test").Start(context.Background(), "active-operation")
	defer span.End()

	attrs := logger.appendCorrelationAttrs(ctx, nil)
	got := make(map[string]string, len(attrs))
	for _, a := range attrs {
		got[a.Key] = a.Value.String()
	}

	assert.Equal(t, span.SpanContext().TraceID().String(), got["trace_id"])
	assert.Equal(t, span.SpanContext().SpanID().String(), got["span_id"])
	assert.Len(t, got["trace_id"], 32)
	assert.Len(t, got["span_id"], 16)
}

func TestConvertLogLevel(t *testing.T) {
	tests := []struct {
		input    observability.LogLevel
		expected string
	}{
		{observability.LogLevelDebug, "DEBUG"},
		{observability.LogLevelInfo, "INFO"},
		{observability.LogLevelWarn, "WARN"},
		{observability.LogLevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			slogLevel := convertLogLevel(tt.input)
			assert.Equal(t, tt.expected, slogLevel.String())
		})
	}
}

func TestLoggerCorrelationFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		withSpan          bool
		correlation       CorrelationContext
		wantRequestID     string
		wantCorrelationID string
	}{
		{
			name:     "active span and propagated correlation context",
			withSpan: true,
			correlation: CorrelationContext{
				RequestID:     "req-123",
				CorrelationID: "corr-456",
			},
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
		},
		{
			name:        "no propagated context",
			withSpan:    false,
			correlation: CorrelationContext{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exporter := &memoryLogExporter{}
			loggerProvider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
			logger := newOtelLogger(
				observability.LogLevelInfo,
				observability.LogFormatJSON,
				"orders-api",
				"test",
				loggerProvider,
				false,
				false,
			)

			ctx := ContextWithCorrelation(context.Background(), tt.correlation)
			var wantTraceID string
			var wantSpanID string
			if tt.withSpan {
				tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
				tracer := tracerProvider.Tracer("logger-test")
				var otelSpan trace.Span
				ctx, otelSpan = tracer.Start(ctx, "active-operation")
				wantTraceID = otelSpan.SpanContext().TraceID().String()
				wantSpanID = otelSpan.SpanContext().SpanID().String()
				defer otelSpan.End()
			}

			logger.Info(ctx, "processed", observability.String("operation", "checkout"))
			require.NoError(t, loggerProvider.ForceFlush(context.Background()))

			records := exporter.Records()
			require.Len(t, records, 1)
			attrs := logRecordAttributes(records[0])

			assert.Equal(t, "orders-api", attrs["service"])
			assert.Equal(t, "test", attrs["environment"])
			assert.Equal(t, "observability.logger", attrs["component"])
			assert.Equal(t, wantTraceID, attrs["trace_id"])
			assert.Equal(t, wantSpanID, attrs["span_id"])
			assert.Equal(t, tt.wantRequestID, attrs["request_id"])
			assert.Equal(t, tt.wantCorrelationID, attrs["correlation_id"])
			assert.Equal(t, "checkout", attrs["operation"])
		})
	}
}

type memoryLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memoryLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *memoryLogExporter) Shutdown(context.Context) error {
	return nil
}

func (e *memoryLogExporter) ForceFlush(context.Context) error {
	return nil
}

func (e *memoryLogExporter) Records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()

	records := make([]sdklog.Record, len(e.records))
	copy(records, e.records)
	return records
}

func logRecordAttributes(record sdklog.Record) map[string]string {
	attrs := make(map[string]string, record.AttributesLen())
	record.WalkAttributes(func(kv otellog.KeyValue) bool {
		attrs[kv.Key] = kv.Value.AsString()
		return true
	})
	return attrs
}
