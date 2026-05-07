package chiserver

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	otelobs "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/go-chi/chi/v5"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	requestIDKey contextKey = "requestID"
)

type httpHookProvider interface {
	HTTP() otelobs.HTTPInstrumentation
}

type routePatternSetter interface {
	SetRoute(string)
}

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
					writeStatusError(w, r, http.StatusInternalServerError, "Internal server error")
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

// requestIDMiddleware validates an incoming X-Request-ID against the
// shared common.ValidateRequestID contract or generates a fresh value
// via common.NewRequestID. When a non-empty incoming value is rejected,
// it emits a structured warn log via pkg/observability with raw_length,
// remote_addr, path and method — the raw value is never logged nor
// echoed back to the client (RF-7.4, R-SEC-001).
func requestIDMiddleware(o11y observability.Observability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get(common.HeaderRequestID)
			id, ok := common.ValidateRequestID(raw)
			if !ok {
				if raw != "" {
					o11y.Logger().Warn(r.Context(), "invalid X-Request-ID rejected",
						observability.Int("raw_length", len(raw)),
						observability.String("remote_addr", r.RemoteAddr),
						observability.String("path", r.URL.Path),
						observability.String("method", r.Method),
					)
				}
				id = common.NewRequestID()
			}

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			w.Header().Set(common.HeaderRequestID, id)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func observabilityMiddleware(router chi.Routes, o11y observability.Observability) func(http.Handler) http.Handler {
	hook := httpInstrumentation(o11y)
	if hook == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID, _ := ctx.Value(requestIDKey).(string)
			correlationID := r.Header.Get("Correlation-ID")

			ctx, scope := hook.StartRequest(ctx, otelobs.HTTPRequest{
				Method:        r.Method,
				Route:         matchedChiRoutePattern(router, r),
				Target:        r.URL.Path,
				RemoteAddr:    r.RemoteAddr,
				UserAgent:     r.UserAgent(),
				RequestID:     requestID,
				CorrelationID: correlationID,
			})
			if scope == nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			rw := newObservedResponseWriter(w)
			defer func() {
				if setter, ok := scope.(routePatternSetter); ok {
					setter.SetRoute(chiRoutePattern(r.Context()))
				}
				finishObservedRequest(scope, rw)
			}()

			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}

func httpInstrumentation(o11y observability.Observability) otelobs.HTTPInstrumentation {
	provider, ok := o11y.(httpHookProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.HTTP()
}

func matchedChiRoutePattern(router chi.Routes, r *http.Request) string {
	if router == nil || r == nil {
		return "unmatched"
	}

	rctx := chi.NewRouteContext()
	if pattern := router.Find(rctx, r.Method, r.URL.Path); pattern != "" {
		return pattern
	}

	return "unmatched"
}

// chiRoutePattern returns the route pattern declared by chi for the
// current request, or the literal "unmatched" when no route has been
// matched (chi.RouteContext is nil or RoutePattern returns ""). This
// guarantees the telemetry route label has cardinality bounded by the
// number of registered patterns plus one (RF-6.2).
func chiRoutePattern(ctx context.Context) string {
	routeCtx := chi.RouteContext(ctx)
	if routeCtx == nil {
		return "unmatched"
	}
	if route := routeCtx.RoutePattern(); route != "" {
		return route
	}
	return "unmatched"
}

func finishObservedRequest(scope otelobs.HTTPRequestScope, rw *observedResponseWriter) {
	if recovered := recover(); recovered != nil {
		scope.OnError(fmt.Errorf("panic: %v", recovered))
		scope.Finish(otelobs.HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Bytes:      rw.bytesWritten(),
		})
		panic(recovered)
	}

	scope.Finish(otelobs.HTTPResponse{
		StatusCode: rw.statusCode(),
		Bytes:      rw.bytesWritten(),
	})
}

type observedResponseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func newObservedResponseWriter(w http.ResponseWriter) *observedResponseWriter {
	return &observedResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (rw *observedResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *observedResponseWriter) Write(body []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	written, err := rw.ResponseWriter.Write(body)
	rw.bytes += int64(written)
	return written, err
}

func (rw *observedResponseWriter) Flush() {
	flusher, ok := rw.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}

func (rw *observedResponseWriter) statusCode() int {
	if rw.status <= 0 {
		return http.StatusOK
	}
	return rw.status
}

func (rw *observedResponseWriter) bytesWritten() int64 {
	return rw.bytes
}

// timeoutMiddleware applies a global timeout to requests as a fallback for
// routes registered directly against the underlying chi.Router (i.e. routes
// that do NOT go through Server.RegisterHandler). Per-route timeouts are
// installed via wrapWithTimeout in Server.RegisterHandler — this top-level
// middleware deliberately does NOT consult routeTimeouts (RF-5.4) so that
// the two paths remain orthogonal and there is a single source of truth for
// the per-route value (the route registration site).
func timeoutMiddleware(globalTimeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), globalTimeout)
			defer cancel()

			tw := newTimeoutWriter(w)
			done := make(chan struct{}, 1)
			panicCh := make(chan any, 1)

			go runHandler(next, tw, r.WithContext(ctx), done, panicCh)

			select {
			case <-done:
				return
			case rec := <-panicCh:
				// Re-panic in the request goroutine so the outer
				// recoverMiddleware (deferred earlier in the same goroutine)
				// can record and respond to it.
				panic(rec)
			case <-ctx.Done():
				tw.mu.Lock()
				if !tw.written {
					tw.written = true
					tw.timedOut = true
					tw.mu.Unlock()
					// security/cors run BEFORE this middleware (see
					// registerMiddlewares ordering), so headers like
					// X-Frame-Options, CSP and HSTS are already on w.Header()
					// when we reach this branch. Writing directly to w is
					// safe because tw is now timedOut so the handler goroutine
					// can no longer touch it.
					writeStatusError(w, r, http.StatusRequestTimeout, "Request timeout exceeded")
				} else {
					tw.mu.Unlock()
				}
			}
		})
	}
}

// runHandler runs next.ServeHTTP and translates panics into a value
// delivered through panicCh so the parent goroutine can re-raise them
// in a frame protected by the outer recoverMiddleware.
func runHandler(next http.Handler, w http.ResponseWriter, r *http.Request, done chan<- struct{}, panicCh chan<- any) {
	defer func() {
		if rec := recover(); rec != nil {
			select {
			case panicCh <- rec:
			default:
			}
			return
		}
		select {
		case done <- struct{}{}:
		default:
		}
	}()
	next.ServeHTTP(w, r)
}

// timeoutWriter wraps http.ResponseWriter to coordinate writes between the
// handler goroutine and the timeout goroutine. The wrapper buffers user
// headers in a private http.Header map so concurrent calls to
// w.Header().Set from the timeout goroutine and the handler goroutine never
// touch the same underlying map (RF-4.2). Once timedOut is observed under
// mu, WriteHeader/Write become no-ops and the buffered headers are dropped
// so the timeout response written by the middleware is the only payload
// that reaches the wire.
type timeoutWriter struct {
	http.ResponseWriter
	mu       sync.Mutex
	h        http.Header
	written  bool
	timedOut bool
}

func newTimeoutWriter(w http.ResponseWriter) *timeoutWriter {
	return &timeoutWriter{
		ResponseWriter: w,
		h:              make(http.Header),
	}
}

// Header returns the timeoutWriter's private header map. Headers set on it
// are flushed to the underlying ResponseWriter on the first Write or
// WriteHeader call as long as the request has not already timed out. After
// a timeout, the buffered map is discarded and never leaks to the wire.
func (tw *timeoutWriter) Header() http.Header {
	return tw.h
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut || tw.written {
		return
	}

	tw.written = true
	tw.flushHeadersLocked()
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}

	if !tw.written {
		tw.written = true
		tw.flushHeadersLocked()
	}

	return tw.ResponseWriter.Write(b)
}

// flushHeadersLocked copies the buffered headers to the underlying
// ResponseWriter. Caller MUST hold tw.mu.
func (tw *timeoutWriter) flushHeadersLocked() {
	dst := tw.ResponseWriter.Header()
	for k, v := range tw.h {
		dst[k] = v
	}
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
				writeStatusError(w, r, http.StatusForbidden, "origin not allowed")
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
				writeStatusError(w, r, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytes))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// wrapWithTimeout installs a per-route timeout decorator around next.
// It is invoked from Server.RegisterHandler when the path has a timeout
// configured via WithRouteTimeout. The middleware:
//   - derives a child context with the per-route deadline (RF-5.4);
//   - serves the handler in a child goroutine so the timeout path can run
//     concurrently with the handler write path;
//   - serializes 408 emission with handler writes via timeoutWriter.mu so
//     the wire only ever sees a single response (RF-4.2);
//   - waits up to s.timeoutCleanup for the handler goroutine to drain and
//     records http_handler_timeout_leak_total when it does not (RF-4.3);
//   - skips the leak counter entirely when s.timeoutCleanup <= 0 (RF-4.6).
//
// Panics inside the handler goroutine are re-panicked so the outer
// recoverMiddleware can record them and emit the canonical 500 response.
func wrapWithTimeout(s *Server, d time.Duration, next http.Handler, route string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()

		tw := newTimeoutWriter(w)
		done := make(chan struct{}, 1)
		panicCh := make(chan any, 1)

		go runHandler(next, tw, r.WithContext(ctx), done, panicCh)

		select {
		case <-done:
			return
		case rec := <-panicCh:
			panic(rec)
		case <-ctx.Done():
			tw.mu.Lock()
			if !tw.written {
				tw.written = true
				tw.timedOut = true
				tw.mu.Unlock()
				// security/cors run on the request goroutine BEFORE the chain
				// reaches this per-route wrap (see registerMiddlewares
				// ordering), so the underlying ResponseWriter already carries
				// those headers. Writing through w here is safe: tw is now
				// timedOut so the handler goroutine can no longer mutate the
				// shared underlying writer.
				writeTimeoutResponse(w, r)
			} else {
				tw.mu.Unlock()
			}

			if s.timeoutCleanup <= 0 {
				return
			}

			timer := time.NewTimer(s.timeoutCleanup)
			defer timer.Stop()
			select {
			case <-done:
				return
			case rec := <-panicCh:
				// Late panic during cleanup — re-raise for outer recover.
				panic(rec)
			case <-timer.C:
				s.recordTimeoutLeak(r.Context(), route, r.URL.Path)
			}
		}
	})
}

// writeTimeoutResponse emits an RFC 7807 408 Request Timeout response.
// chi_server is plain net/http; we deliberately do NOT route through
// common.ProblemFromError (which understands *fiber.Error) so the chi
// adapter stays free of fiber coupling.
func writeTimeoutResponse(w http.ResponseWriter, r *http.Request) {
	writeStatusError(w, r, http.StatusRequestTimeout, "Request timeout exceeded")
}
