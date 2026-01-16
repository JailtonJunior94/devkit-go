package common

import (
	"net/http"
	"sync"
)

// ResponseWriter wraps http.ResponseWriter to track if headers were written.
// This wrapper is thread-safe and prevents race conditions when checking
// whether headers have been sent, which is critical in panic recovery
// scenarios where multiple goroutines might attempt to write to the response.
//
// Thread-safety: All methods use mutex protection to ensure safe concurrent access.
type ResponseWriter struct {
	http.ResponseWriter
	mu            sync.Mutex
	headerWritten bool
}

// NewResponseWriter creates a new thread-safe ResponseWriter wrapper.
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
	}
}

// WriteHeader writes the HTTP status code to the response.
// It ensures headers are written only once to prevent http: superfluous response.WriteHeader calls.
//
// Thread-safe: Uses mutex to protect concurrent access to headerWritten flag.
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.headerWritten {
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write writes the data to the response body.
// If headers haven't been written yet, Write will trigger an implicit WriteHeader(200).
//
// Thread-safe: Uses mutex to protect concurrent access to headerWritten flag and the underlying Write call.
// NOTE: The mutex is held during I/O to ensure thread-safety of the underlying ResponseWriter.
// In practice, each HTTP request has its own ResponseWriter, so concurrent writes are rare.
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.headerWritten {
		rw.headerWritten = true
	}

	return rw.ResponseWriter.Write(b)
}

// HeaderWritten returns true if headers have been written to the response.
// This is useful for panic recovery to determine if an error response can still be sent.
//
// Thread-safe: Uses mutex to protect concurrent access to headerWritten flag.
func (rw *ResponseWriter) HeaderWritten() bool {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.headerWritten
}
