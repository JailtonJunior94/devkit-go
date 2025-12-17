package o11y

import (
	"errors"
	"testing"
)

func TestSanitizeError_Nil(t *testing.T) {
	result := sanitizeError(nil)
	if result != "" {
		t.Errorf("expected empty string, got %v", result)
	}
}

func TestSanitizeError_NoSensitiveData(t *testing.T) {
	err := errors.New("simple error message")
	result := sanitizeError(err)
	if result != "simple error message" {
		t.Errorf("expected 'simple error message', got %v", result)
	}
}

func TestSanitizeError_ConnectionString(t *testing.T) {
	err := errors.New("failed to connect: postgres://user:secretpassword@localhost:5432/db")
	result := sanitizeError(err)

	if result == err.Error() {
		t.Error("expected credentials to be redacted")
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected redacted marker, got %v", result)
	}
}

func TestSanitizeError_BearerToken(t *testing.T) {
	err := errors.New("auth failed: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U")
	result := sanitizeError(err)

	if result == err.Error() {
		t.Error("expected token to be redacted")
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected redacted marker, got %v", result)
	}
}

func TestSanitizeError_APIKey(t *testing.T) {
	err := errors.New("API error: api_key=sk_live_1234567890abcdefghij")
	result := sanitizeError(err)

	if result == err.Error() {
		t.Error("expected API key to be redacted")
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected redacted marker, got %v", result)
	}
}

func TestSanitizeError_Password(t *testing.T) {
	err := errors.New("config error: password=mysecretpassword")
	result := sanitizeError(err)

	if result == err.Error() {
		t.Error("expected password to be redacted")
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected redacted marker, got %v", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
