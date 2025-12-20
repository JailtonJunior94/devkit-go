package o11y

import (
	"testing"
)

func TestLogger_SensitiveKeyDetection(t *testing.T) {
	l := &logger{
		sensitiveKeys:   DefaultSensitiveKeys,
		redactSensitive: true,
	}

	tests := []struct {
		key       string
		sensitive bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"api_key", true},
		{"API_KEY", true},
		{"apikey", true},
		{"token", true},
		{"access_token", true},
		{"authorization", true},
		{"credit_card", true},
		{"ssn", true},
		{"secret", true},
		{"credential", true},
		// Non-sensitive keys
		{"username", false},
		{"email", false},
		{"name", false},
		{"id", false},
		{"status", false},
		{"timestamp", false},
	}

	for _, tt := range tests {
		result := l.isSensitiveKey(tt.key)
		if result != tt.sensitive {
			t.Errorf("isSensitiveKey(%q) = %v, expected %v", tt.key, result, tt.sensitive)
		}
	}
}

func TestLogger_SensitiveKeyDetection_Disabled(t *testing.T) {
	l := &logger{
		sensitiveKeys:   DefaultSensitiveKeys,
		redactSensitive: false, // Disabled
	}

	// Even if the key is sensitive, when redaction is disabled,
	// the key itself is still detected, but redaction won't happen in log()
	if !l.isSensitiveKey("password") {
		t.Error("isSensitiveKey should still detect sensitive keys even when redaction is disabled")
	}
}

func TestLogger_SensitiveKeyDetection_CustomKeys(t *testing.T) {
	l := &logger{
		sensitiveKeys:   []string{"custom_secret", "my_token"},
		redactSensitive: true,
	}

	tests := []struct {
		key       string
		sensitive bool
	}{
		{"custom_secret", true},
		{"my_custom_secret_field", true},
		{"my_token", true},
		{"password", false}, // Not in custom list
		{"api_key", false},  // Not in custom list
	}

	for _, tt := range tests {
		result := l.isSensitiveKey(tt.key)
		if result != tt.sensitive {
			t.Errorf("isSensitiveKey(%q) = %v, expected %v", tt.key, result, tt.sensitive)
		}
	}
}

func TestLogger_SensitiveKeyDetection_EmptyKeys(t *testing.T) {
	l := &logger{
		sensitiveKeys:   []string{},
		redactSensitive: true,
	}

	// With empty sensitive keys, nothing should be detected
	if l.isSensitiveKey("password") {
		t.Error("isSensitiveKey should return false when sensitiveKeys is empty")
	}
}

func TestDefaultSensitiveKeys(t *testing.T) {
	// Verify default sensitive keys are populated
	if len(DefaultSensitiveKeys) == 0 {
		t.Error("DefaultSensitiveKeys should not be empty")
	}

	expectedKeys := []string{
		"password",
		"token",
		"api_key",
		"secret",
		"authorization",
	}

	for _, expected := range expectedKeys {
		found := false
		for _, key := range DefaultSensitiveKeys {
			if key == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultSensitiveKeys should contain %q", expected)
		}
	}
}
