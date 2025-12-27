package otel

import (
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/assert"
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
					assert.Equal(t, redactedValue, result[i].Value)
				} else if str, ok := tt.expected[i].Value.(string); ok && len(str) > maxFieldValueLength {
					// For truncated fields
					assert.Contains(t, result[i].Value, "...[truncated]")
				} else {
					// For normal fields
					assert.Equal(t, tt.expected[i].Value, result[i].Value)
				}
			}
		})
	}
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
