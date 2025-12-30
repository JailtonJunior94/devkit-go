package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// ContextKeyRequestID is the context key for request ID.
	ContextKeyRequestID ContextKey = "request-id"
)

// RequestID is a middleware that adds a unique request ID to each request.
// The request ID is stored in the context and can be retrieved using
// GetRequestID or directly from the context with ContextKeyRequestID.
//
// If UUID generation fails, a fallback ID based on timestamp and random bytes
// is generated to ensure every request has an ID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID generates a unique request ID using UUID v7.
// Falls back to timestamp-based ID if UUID generation fails.
func generateRequestID() string {
	id, err := vos.NewUUID()
	if err == nil {
		return id.String()
	}
	log.Printf("Failed to generate UUID for request ID, using fallback: %v", err)
	return generateFallbackID()
}

// GetRequestID retrieves the request ID from the context.
// Returns an empty string if no request ID is found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(ContextKeyRequestID).(string)
	return requestID
}

// generateFallbackID generates a fallback ID when UUID generation fails.
// Format: timestamp (hex) + random bytes = 8 + 8 = 16 hex chars.
func generateFallbackID() string {
	// Use timestamp for ordering
	ts := time.Now().UnixNano()
	tsHex := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		tsHex[i] = byte(ts)
		ts >>= 8
	}

	// Add random bytes for uniqueness
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// If even random fails, use a simple counter-based fallback
		// This should basically never happen
		return hex.EncodeToString(tsHex) + "00000000"
	}

	return hex.EncodeToString(tsHex) + hex.EncodeToString(randomBytes)
}

// Recovery is a middleware that recovers from panics and returns a 500 error.
// It logs the panic and stack trace for debugging.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err == nil {
				return
			}
			logPanic(r.Context(), err)
			w.WriteHeader(http.StatusInternalServerError)
		}()

		next.ServeHTTP(w, r)
	})
}

// logPanic logs a panic with request ID if available.
func logPanic(ctx context.Context, err any) {
	requestID := GetRequestID(ctx)
	if requestID == "" {
		log.Printf("PANIC recovered: %v", err)
		return
	}
	log.Printf("[%s] PANIC recovered: %v", requestID, err)
}

// sanitizeHeaderValue removes CR and LF characters to prevent HTTP header injection.
func sanitizeHeaderValue(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

// CORS is a middleware that adds CORS headers to responses.
// For production, consider using a more configurable CORS middleware.
// Note: All header values are sanitized to prevent CRLF injection attacks.
func CORS(allowedOrigins, allowedMethods, allowedHeaders string) Middleware {
	// Sanitize at creation time for efficiency
	origins := sanitizeHeaderValue(allowedOrigins)
	methods := sanitizeHeaderValue(allowedMethods)
	headers := sanitizeHeaderValue(allowedHeaders)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContentType is a middleware that sets the Content-Type header for responses.
func ContentType(contentType string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			next.ServeHTTP(w, r)
		})
	}
}

// JSONContentType is a middleware that sets Content-Type to application/json.
func JSONContentType(next http.Handler) http.Handler {
	return ContentType("application/json")(next)
}

// SecurityHeaders is a middleware that adds common security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// Enable XSS filter
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// Timeout is a middleware that sets a timeout for request processing.
// It uses a thread-safe response writer to prevent race conditions
// when timeout occurs while the handler is still writing.
func Timeout(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			tw := &timeoutWriter{
				ResponseWriter: w,
				req:            r,
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				next.ServeHTTP(tw, r.WithContext(ctx))
			}()

			select {
			case <-done:
				tw.mu.Lock()
				defer tw.mu.Unlock()
				if tw.timedOut {
					return
				}
			case <-ctx.Done():
				tw.mu.Lock()
				defer tw.mu.Unlock()
				if tw.wroteHeader {
					return
				}
				tw.timedOut = true
				logTimeout(r.Context())
				w.WriteHeader(http.StatusGatewayTimeout)
			}
		})
	}
}

// timeoutWriter is a thread-safe ResponseWriter that prevents writes after timeout.
type timeoutWriter struct {
	http.ResponseWriter
	req         *http.Request
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, context.DeadlineExceeded
	}
	if !tw.wroteHeader {
		tw.wroteHeader = true
		tw.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return tw.ResponseWriter.Write(b)
}

// logTimeout logs a timeout with request ID if available.
func logTimeout(ctx context.Context) {
	requestID := GetRequestID(ctx)
	if requestID == "" {
		log.Printf("Request timeout exceeded")
		return
	}
	log.Printf("[%s] Request timeout exceeded", requestID)
}
