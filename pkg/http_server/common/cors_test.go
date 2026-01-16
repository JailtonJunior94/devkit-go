package common

import (
	"testing"
)

func TestParseOrigins_Empty(t *testing.T) {
	origins, err := ParseOrigins("")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 0 {
		t.Errorf("expected empty slice, got %v", origins)
	}
}

func TestParseOrigins_Whitespace(t *testing.T) {
	origins, err := ParseOrigins("   ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 0 {
		t.Errorf("expected empty slice for whitespace, got %v", origins)
	}
}

func TestParseOrigins_Wildcard(t *testing.T) {
	origins, err := ParseOrigins("*")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 1 || origins[0] != "*" {
		t.Errorf("expected [*], got %v", origins)
	}
}

func TestParseOrigins_SingleOrigin(t *testing.T) {
	origins, err := ParseOrigins("https://example.com")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 1 || origins[0] != "https://example.com" {
		t.Errorf("expected [https://example.com], got %v", origins)
	}
}

func TestParseOrigins_MultipleOrigins(t *testing.T) {
	origins, err := ParseOrigins("https://example.com,https://app.example.com,https://api.example.com")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 3 {
		t.Errorf("expected 3 origins, got %d", len(origins))
	}

	expected := []string{
		"https://example.com",
		"https://app.example.com",
		"https://api.example.com",
	}

	for i, exp := range expected {
		if origins[i] != exp {
			t.Errorf("expected origins[%d]=%s, got %s", i, exp, origins[i])
		}
	}
}

func TestParseOrigins_MultipleOriginsWithSpaces(t *testing.T) {
	origins, err := ParseOrigins("  https://example.com  ,  https://app.example.com  ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(origins))
	}

	if origins[0] != "https://example.com" {
		t.Errorf("expected origins[0]=https://example.com, got %s", origins[0])
	}

	if origins[1] != "https://app.example.com" {
		t.Errorf("expected origins[1]=https://app.example.com, got %s", origins[1])
	}
}

func TestParseOrigins_EmptyElements(t *testing.T) {
	origins, err := ParseOrigins("https://example.com,,https://app.example.com")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Empty elements should be filtered out
	if len(origins) != 2 {
		t.Errorf("expected 2 origins (empty filtered), got %d", len(origins))
	}
}

func TestParseOrigins_WildcardWithOtherOrigins_Error(t *testing.T) {
	_, err := ParseOrigins("*,https://example.com")
	if err == nil {
		t.Error("expected error for wildcard combined with other origins")
	}

	if err.Error() != "wildcard (*) cannot be combined with other origins" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseOrigins_WildcardAtEnd_Error(t *testing.T) {
	_, err := ParseOrigins("https://example.com,*")
	if err == nil {
		t.Error("expected error for wildcard combined with other origins")
	}
}

func TestParseOrigins_WildcardInMiddle_Error(t *testing.T) {
	_, err := ParseOrigins("https://example.com,*,https://app.example.com")
	if err == nil {
		t.Error("expected error for wildcard combined with other origins")
	}
}

func TestIsOriginAllowed_Empty(t *testing.T) {
	allowed := IsOriginAllowed("https://example.com", []string{})
	if allowed {
		t.Error("expected false for empty allowed origins")
	}
}

func TestIsOriginAllowed_Wildcard(t *testing.T) {
	allowed := IsOriginAllowed("https://example.com", []string{"*"})
	if !allowed {
		t.Error("expected true for wildcard")
	}

	allowed = IsOriginAllowed("https://any-origin.com", []string{"*"})
	if !allowed {
		t.Error("expected true for wildcard with any origin")
	}
}

func TestIsOriginAllowed_ExactMatch(t *testing.T) {
	allowedOrigins := []string{"https://example.com", "https://app.example.com"}

	allowed := IsOriginAllowed("https://example.com", allowedOrigins)
	if !allowed {
		t.Error("expected true for exact match")
	}

	allowed = IsOriginAllowed("https://app.example.com", allowedOrigins)
	if !allowed {
		t.Error("expected true for exact match")
	}
}

func TestIsOriginAllowed_NoMatch(t *testing.T) {
	allowedOrigins := []string{"https://example.com", "https://app.example.com"}

	allowed := IsOriginAllowed("https://evil.com", allowedOrigins)
	if allowed {
		t.Error("expected false for non-matching origin")
	}
}

func TestIsOriginAllowed_CaseSensitive(t *testing.T) {
	allowedOrigins := []string{"https://example.com"}

	// Origins should be case-sensitive (per spec)
	allowed := IsOriginAllowed("https://Example.com", allowedOrigins)
	if allowed {
		t.Error("expected false for case mismatch (origins are case-sensitive)")
	}
}

func TestIsOriginAllowed_PartialMatch(t *testing.T) {
	allowedOrigins := []string{"https://example.com"}

	// Should not allow subdomain if not explicitly listed
	allowed := IsOriginAllowed("https://app.example.com", allowedOrigins)
	if allowed {
		t.Error("expected false for subdomain not in allowed list")
	}

	// Should not allow if origin is substring
	allowed = IsOriginAllowed("https://example.com.evil.com", allowedOrigins)
	if allowed {
		t.Error("expected false for origin that contains allowed origin as substring")
	}
}

func TestParseOrigins_Integration(t *testing.T) {
	// Test realistic scenario
	input := "https://example.com, https://app.example.com, https://api.example.com"
	origins, err := ParseOrigins(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test each origin
	testCases := []struct {
		origin  string
		allowed bool
	}{
		{"https://example.com", true},
		{"https://app.example.com", true},
		{"https://api.example.com", true},
		{"https://evil.com", false},
		{"https://admin.example.com", false}, // Not in list
	}

	for _, tc := range testCases {
		allowed := IsOriginAllowed(tc.origin, origins)
		if allowed != tc.allowed {
			t.Errorf("origin %s: expected allowed=%v, got %v", tc.origin, tc.allowed, allowed)
		}
	}
}
