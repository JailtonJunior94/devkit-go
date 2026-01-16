package chiserver

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	requestIDKey contextKey = "requestID"
)

// recoverMiddleware recovers from panics and logs them.
// SECURITY: Uses a wrapper to track if headers were sent to prevent double write.
func recoverMiddleware(o11y observability.Observability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap ResponseWriter to track if headers were sent (thread-safe)
			rw := common.NewResponseWriter(w)

			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				stack := string(debug.Stack())
				requestID, _ := r.Context().Value(requestIDKey).(string)

				fields := []observability.Field{
					observability.String("path", r.URL.Path),
					observability.String("method", r.Method),
					observability.String("remote_addr", r.RemoteAddr),
					observability.String("stack", stack),
					observability.Any("panic", recovered),
				}

				if requestID != "" {
					fields = append(fields, observability.String("request_id", requestID))
				}

				o11y.Logger().Error(r.Context(), "panic recovered", fields...)

				// Only write error if headers haven't been sent yet
				if !rw.HeaderWritten() {
					writeErrorResponse(w, r, http.StatusInternalServerError, "Internal server error")
				} else {
					o11y.Logger().Warn(r.Context(),
						"cannot send panic error response: headers already sent",
						observability.String("request_id", requestID),
					)
				}
			}()

			next.ServeHTTP(rw, r)
		})
	}
}


// requestIDMiddleware generates or propagates a request ID.
func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")

			if strings.TrimSpace(requestID) == "" {
				requestID = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-ID", requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// timeoutMiddleware applies a timeout to requests with race-free implementation.
// IMPORTANT: Handlers MUST respect context cancellation to prevent goroutine leaks.
// When ctx.Done() is signaled, handlers should stop processing immediately.
func timeoutMiddleware(
	globalTimeout time.Duration,
	routeTimeouts map[string]time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timeout := globalTimeout

			if routeTimeout, exists := routeTimeouts[r.URL.Path]; exists {
				timeout = routeTimeout
			}

			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Wrap the ResponseWriter to detect if headers were written
			tw := &timeoutWriter{ResponseWriter: w}

			// Buffered channel prevents goroutine leak if we timeout before handler finishes
			done := make(chan struct{}, 1)

			go func() {
				defer func() {
					// Recover any panics from the handler
					if recovered := recover(); recovered != nil {
						// Re-panic will be caught by the outer recover middleware
						panic(recovered)
					}
					// Signal completion (non-blocking due to buffer)
					select {
					case done <- struct{}{}:
					default:
					}
				}()
				next.ServeHTTP(tw, r.WithContext(ctx))
			}()

			select {
			case <-done:
				// Handler completed successfully before timeout
				return
			case <-ctx.Done():
				// Timeout occurred - context cancellation signals handler to stop
				tw.mu.Lock()
				defer tw.mu.Unlock()

				// Only write timeout error if handler hasn't written anything yet
				if !tw.written {
					tw.written = true
					tw.timedOut = true
					writeErrorResponse(w, r, http.StatusRequestTimeout, "Request timeout exceeded")
				}

				// CRITICAL: Wait a short time for goroutine cleanup
				// This gives the handler time to respect context cancellation
				// Most well-behaved handlers will stop within 100ms
				cleanupTimer := time.NewTimer(100 * time.Millisecond)
				defer cleanupTimer.Stop()

				select {
				case <-done:
					// Handler finished cleanup successfully
				case <-cleanupTimer.C:
					// Handler didn't respect context cancellation
					// This is a handler bug, but we can't do much more
					// The goroutine will eventually finish but may hold resources longer
				}
			}
		})
	}
}

// timeoutWriter wraps http.ResponseWriter to track if response was written
type timeoutWriter struct {
	http.ResponseWriter
	mu       sync.Mutex
	written  bool
	timedOut bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut {
		// Timeout already occurred, don't write
		return
	}

	if !tw.written {
		tw.written = true
		tw.ResponseWriter.WriteHeader(code)
	}
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut {
		// Timeout already occurred, don't write
		return 0, http.ErrHandlerTimeout
	}

	if !tw.written {
		tw.written = true
	}

	return tw.ResponseWriter.Write(b)
}

// securityHeadersMiddleware adds comprehensive security headers to responses.
// Uses common.SecurityHeaders for centralized security configuration.
func securityHeadersMiddleware() func(http.Handler) http.Handler {
	// Initialize security headers once (reuse for all requests)
	securityHeaders := common.DefaultSecurityHeaders()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			securityHeaders.Apply(w)
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware handles CORS with proper origin validation.
func corsMiddleware(origins string) func(http.Handler) http.Handler {
	// Parse allowed origins with validation
	allowedOrigins, err := common.ParseOrigins(origins)
	if err != nil {
		// If configuration is invalid, panic during setup (fail-fast)
		panic(fmt.Sprintf("invalid CORS configuration: %v", err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// If no Origin header, skip CORS
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Validate if origin is allowed
			if !common.IsOriginAllowed(origin, allowedOrigins) {
				writeErrorResponse(w, r, http.StatusForbidden, "origin not allowed")
				return
			}

			// SECURITY: Never use wildcard (*) with credentials
			// If wildcard is needed, it must be set explicitly and credentials disabled
			if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				// Do NOT set Access-Control-Allow-Credentials with wildcard
			} else {
				// Set specific origin (not wildcard)
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Credentials can be allowed with specific origins
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "3600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}


// bodyLimitMiddleware enforces a maximum request body size.
// SECURITY: Always applies MaxBytesReader regardless of Content-Length header
// to prevent bypass via chunked encoding or missing/manipulated headers.
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ALWAYS apply MaxBytesReader to prevent DOS attacks
			// This works even with chunked encoding or missing Content-Length
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			// Check Content-Length as an early validation (optimization)
			// but don't rely on it for security
			if r.ContentLength > maxBytes {
				writeErrorResponse(w, r, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytes))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
