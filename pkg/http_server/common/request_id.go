package common

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	// MaxRequestIDLength is the maximum accepted length for an incoming
	// X-Request-ID value. Values longer than this are rejected.
	MaxRequestIDLength = 128

	// HeaderRequestID is the canonical request correlation header name.
	HeaderRequestID = "X-Request-ID"
)

// requestIDPattern restricts the accepted charset to [A-Za-z0-9._-].
// Characters such as ":", "/", whitespace and unicode are rejected to
// avoid ambiguity in structured logs and to prevent log-injection.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateRequestID returns the trimmed value and true when raw is an
// acceptable request id; otherwise it returns ("", false). The caller
// decides what to do on failure (generate a new one, emit a warning, ...).
func ValidateRequestID(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if len(s) > MaxRequestIDLength {
		return "", false
	}
	if !requestIDPattern.MatchString(s) {
		return "", false
	}
	return s, true
}

// NewRequestID returns a freshly generated UUIDv4 string suitable for
// use as a request id.
func NewRequestID() string {
	return uuid.New().String()
}
