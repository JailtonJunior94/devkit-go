package common

import (
	"net/http/httptest"
	"testing"
)

func TestDefaultSecurityHeaders(t *testing.T) {
	headers := DefaultSecurityHeaders()
	headersMap := headers.ToMap()

	// Check all required headers are present
	requiredHeaders := []string{
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Content-Security-Policy",
		"Referrer-Policy",
		"Permissions-Policy",
	}

	for _, header := range requiredHeaders {
		if _, ok := headersMap[header]; !ok {
			t.Errorf("expected header %s to be present", header)
		}
	}
}

func TestSecurityHeaders_Apply(t *testing.T) {
	headers := DefaultSecurityHeaders()

	w := httptest.NewRecorder()
	headers.Apply(w)

	// Verify X-Frame-Options
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %s", got)
	}

	// Verify X-Content-Type-Options
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options=nosniff, got %s", got)
	}

	// Verify HSTS
	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected Strict-Transport-Security header to be set")
	}
	if hsts != "max-age=31536000; includeSubDomains; preload" {
		t.Errorf("unexpected HSTS value: %s", hsts)
	}

	// Verify CSP is present
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}

	// Verify Referrer-Policy
	if got := w.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy=strict-origin-when-cross-origin, got %s", got)
	}

	// Verify Permissions-Policy is present
	permPolicy := w.Header().Get("Permissions-Policy")
	if permPolicy == "" {
		t.Error("expected Permissions-Policy header to be set")
	}
}

func TestSecurityHeaders_With(t *testing.T) {
	headers := DefaultSecurityHeaders()
	headers = headers.With("X-Custom-Header", "custom-value")

	headersMap := headers.ToMap()

	if got := headersMap["X-Custom-Header"]; got != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got %s", got)
	}
}

func TestSecurityHeaders_WithOverride(t *testing.T) {
	headers := DefaultSecurityHeaders()
	originalCSP := headers.ToMap()["Content-Security-Policy"]

	headers = headers.With("Content-Security-Policy", "default-src 'self' https://cdn.example.com")

	if got := headers.ToMap()["Content-Security-Policy"]; got == originalCSP {
		t.Error("expected CSP to be overridden")
	}

	if got := headers.ToMap()["Content-Security-Policy"]; got != "default-src 'self' https://cdn.example.com" {
		t.Errorf("expected CSP to be overridden with custom value, got %s", got)
	}
}

func TestSecurityHeaders_Without(t *testing.T) {
	headers := DefaultSecurityHeaders()
	headers = headers.Without("X-XSS-Protection")

	headersMap := headers.ToMap()

	if _, ok := headersMap["X-XSS-Protection"]; ok {
		t.Error("expected X-XSS-Protection to be removed")
	}

	// Verify other headers are still present
	if _, ok := headersMap["X-Frame-Options"]; !ok {
		t.Error("expected X-Frame-Options to still be present")
	}
}

func TestSecurityHeaders_ToMap(t *testing.T) {
	headers := DefaultSecurityHeaders()
	headersMap1 := headers.ToMap()
	headersMap2 := headers.ToMap()

	// Verify it returns a copy (modifying one doesn't affect the other)
	headersMap1["X-Test"] = "test"

	if _, ok := headersMap2["X-Test"]; ok {
		t.Error("expected headersMap2 to not be affected by modifications to headersMap1")
	}
}

func TestSecurityHeaders_EmptyValues(t *testing.T) {
	headers := DefaultSecurityHeaders()

	w := httptest.NewRecorder()
	headers.Apply(w)

	// X-Powered-By and Server should be set to empty string
	if got := w.Header().Get("X-Powered-By"); got != "" {
		t.Errorf("expected X-Powered-By to be empty, got %s", got)
	}

	if got := w.Header().Get("Server"); got != "" {
		t.Errorf("expected Server to be empty, got %s", got)
	}
}

func TestSecurityHeaders_CSPDefaultIsRestrictive(t *testing.T) {
	headers := DefaultSecurityHeaders()
	csp := headers.ToMap()["Content-Security-Policy"]

	// Verify restrictive defaults
	requiredDirectives := []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	for _, directive := range requiredDirectives {
		if !containsString(csp, directive) {
			t.Errorf("expected CSP to contain directive: %s", directive)
		}
	}
}

func TestSecurityHeaders_PermissionsPolicyRestrictive(t *testing.T) {
	headers := DefaultSecurityHeaders()
	permPolicy := headers.ToMap()["Permissions-Policy"]

	// Verify dangerous features are disabled
	dangerousFeatures := []string{
		"geolocation=()",
		"camera=()",
		"microphone=()",
		"payment=()",
		"usb=()",
	}

	for _, feature := range dangerousFeatures {
		if !containsString(permPolicy, feature) {
			t.Errorf("expected Permissions-Policy to restrict feature: %s", feature)
		}
	}
}

func TestSecurityHeaders_ApplyDoesNotMutateOriginal(t *testing.T) {
	headers := DefaultSecurityHeaders()
	originalMap := headers.ToMap()

	w := httptest.NewRecorder()
	headers.Apply(w)

	// Modify headers on ResponseWriter
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")

	// Verify original SecurityHeaders are not affected
	if got := headers.ToMap()["X-Frame-Options"]; got != "DENY" {
		t.Errorf("expected original headers to remain unchanged, got %s", got)
	}

	// Verify originalMap is not affected
	if got := originalMap["X-Frame-Options"]; got != "DENY" {
		t.Error("expected originalMap to remain unchanged")
	}
}

func TestSecurityHeaders_With_DoesNotMutateOriginal(t *testing.T) {
	original := DefaultSecurityHeaders()
	originalSnapshot := original.ToMap()

	derived := original.With("X-Frame-Options", "SAMEORIGIN")
	derived = derived.With("X-Custom-Header", "value")

	originalAfter := original.ToMap()

	if got := originalAfter["X-Frame-Options"]; got != "DENY" {
		t.Errorf("expected original X-Frame-Options to remain DENY, got %s", got)
	}
	if _, ok := originalAfter["X-Custom-Header"]; ok {
		t.Error("expected original to not contain X-Custom-Header")
	}
	if len(originalAfter) != len(originalSnapshot) {
		t.Errorf("expected original size %d, got %d", len(originalSnapshot), len(originalAfter))
	}

	derivedMap := derived.ToMap()
	if got := derivedMap["X-Frame-Options"]; got != "SAMEORIGIN" {
		t.Errorf("expected derived X-Frame-Options=SAMEORIGIN, got %s", got)
	}
	if got := derivedMap["X-Custom-Header"]; got != "value" {
		t.Errorf("expected derived X-Custom-Header=value, got %s", got)
	}
}

func TestSecurityHeaders_Without_DoesNotMutateOriginal(t *testing.T) {
	original := DefaultSecurityHeaders()
	derived := original.Without("X-Frame-Options")

	if _, ok := original.ToMap()["X-Frame-Options"]; !ok {
		t.Error("expected original to still contain X-Frame-Options")
	}
	if _, ok := derived.ToMap()["X-Frame-Options"]; ok {
		t.Error("expected derived to not contain X-Frame-Options")
	}
}

func TestSecurityHeaders_Apply_DeletesEmptyValueHeaders(t *testing.T) {
	headers := DefaultSecurityHeaders()
	w := httptest.NewRecorder()

	// Pre-populate headers Apply should strip.
	w.Header().Set("Server", "nginx/1.25")
	w.Header().Set("X-Powered-By", "Express")

	headers.Apply(w)

	if got := w.Header().Get("Server"); got != "" {
		t.Errorf("expected Server to be deleted, got %s", got)
	}
	if got := w.Header().Get("X-Powered-By"); got != "" {
		t.Errorf("expected X-Powered-By to be deleted, got %s", got)
	}

	if _, ok := w.Header()["Server"]; ok {
		t.Error("expected Server key to be absent from header map after Del")
	}
	if _, ok := w.Header()["X-Powered-By"]; ok {
		t.Error("expected X-Powered-By key to be absent from header map after Del")
	}
}

func TestSecurityHeaders_Apply_RestoresUserHeaderToEmptyViaDel(t *testing.T) {
	custom := DefaultSecurityHeaders().With("X-Custom-Header", "")
	w := httptest.NewRecorder()
	w.Header().Set("X-Custom-Header", "previous-value")

	custom.Apply(w)

	if _, ok := w.Header()["X-Custom-Header"]; ok {
		t.Error("expected X-Custom-Header to be deleted when configured value is empty")
	}
}

// Helper function to check if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
