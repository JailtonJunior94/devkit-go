package chiserver

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	requestIDKey contextKey = "requestID"
)

// recoverMiddleware recovers from panics and logs them.
func recoverMiddleware(o11y observability.Observability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

				writeErrorResponse(w, r, http.StatusInternalServerError, "Internal server error")
			}()

			next.ServeHTTP(w, r)
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

// timeoutMiddleware applies a timeout to requests.
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

			done := make(chan struct{})

			go func() {
				defer close(done)
				next.ServeHTTP(w, r.WithContext(ctx))
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				writeErrorResponse(w, r, http.StatusRequestTimeout, "Request timeout")
			}
		})
	}
}

// securityHeadersMiddleware adds security headers to responses.
func securityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("X-Powered-By", "")

			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware handles CORS.
func corsMiddleware(origins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
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
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeErrorResponse(w, r, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("Request body too large. Maximum size is %d bytes", maxBytes))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}
