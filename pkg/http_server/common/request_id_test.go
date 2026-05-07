package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRequestID_TableDriven(t *testing.T) {
	t.Parallel()

	maxBoundary := strings.Repeat("a", MaxRequestIDLength)
	overBoundary := strings.Repeat("a", MaxRequestIDLength+1)

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantOut string
	}{
		{name: "valid alphanumeric", input: "abc123", wantOK: true, wantOut: "abc123"},
		{name: "valid uuid-like", input: "0d6e91a8-2b3c-4d6e-91a8-1234567890ab", wantOK: true, wantOut: "0d6e91a8-2b3c-4d6e-91a8-1234567890ab"},
		{name: "valid with dots and underscores", input: "req.id_42-x", wantOK: true, wantOut: "req.id_42-x"},
		{name: "valid 128 char boundary", input: maxBoundary, wantOK: true, wantOut: maxBoundary},
		{name: "valid trims surrounding whitespace", input: "  abc-1  ", wantOK: true, wantOut: "abc-1"},
		{name: "rejects empty", input: "", wantOK: false},
		{name: "rejects whitespace only", input: "   ", wantOK: false},
		{name: "rejects over 128 chars", input: overBoundary, wantOK: false},
		{name: "rejects colon", input: "abc:123", wantOK: false},
		{name: "rejects slash", input: "abc/123", wantOK: false},
		{name: "rejects newline", input: "abc\n123", wantOK: false},
		{name: "rejects carriage return", input: "abc\r123", wantOK: false},
		{name: "rejects internal space", input: "abc 123", wantOK: false},
		{name: "rejects unicode", input: "abç-123", wantOK: false},
		{name: "rejects equals", input: "abc=123", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ValidateRequestID(tc.input)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.Equal(t, tc.wantOut, got)
			} else {
				require.Empty(t, got)
			}
		})
	}
}

func TestNewRequestID_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()

	const samples = 64
	seen := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		id := NewRequestID()
		require.NotEmpty(t, id)
		_, dup := seen[id]
		require.False(t, dup, "expected unique id, got duplicate %q", id)
		seen[id] = struct{}{}

		validated, ok := ValidateRequestID(id)
		require.True(t, ok, "generated id %q must satisfy ValidateRequestID", id)
		require.Equal(t, id, validated)
	}
}

func TestHeaderRequestID_CanonicalName(t *testing.T) {
	t.Parallel()
	require.Equal(t, "X-Request-ID", HeaderRequestID)
}
