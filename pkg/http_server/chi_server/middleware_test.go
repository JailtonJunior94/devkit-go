package chiserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	otelobs "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedObservabilityMiddleware_Chi(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		path              string
		handler           func(http.ResponseWriter, *http.Request)
		wantStatus        int
		wantRoute         string
		wantRequestID     string
		wantCorrelationID string
	}{
		{
			name: "preserves correlation headers and finishes successful request",
			path: "/users/123",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("ok"))
			},
			wantStatus:        http.StatusAccepted,
			wantRoute:         "/users/{id}",
			wantRequestID:     "req-123",
			wantCorrelationID: "corr-456",
		},
		{
			name: "records server error status without correlation headers",
			path: "/users/500",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus: http.StatusInternalServerError,
			wantRoute:  "/users/{id}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hook := &recordingHTTPInstrumentation{}
			srv := newObservedChiServer(t, hook, tt.handler)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.wantRequestID != "" {
				req.Header.Set("X-Request-ID", tt.wantRequestID)
			}
			if tt.wantCorrelationID != "" {
				req.Header.Set("Correlation-ID", tt.wantCorrelationID)
			}
			rec := httptest.NewRecorder()

			srv.router.ServeHTTP(rec, req)

			require.Len(t, hook.requests, 1)
			assert.Equal(t, http.MethodGet, hook.requests[0].Method)
			assert.Equal(t, tt.wantRoute, hook.requests[0].Route)
			assert.Equal(t, tt.path, hook.requests[0].Target)
			assert.Equal(t, tt.wantCorrelationID, hook.requests[0].CorrelationID)
			if tt.wantRequestID == "" {
				assert.NotEmpty(t, hook.requests[0].RequestID)
				assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
			} else {
				assert.Equal(t, tt.wantRequestID, hook.requests[0].RequestID)
				assert.Equal(t, tt.wantRequestID, rec.Header().Get("X-Request-ID"))
			}

			require.Len(t, hook.scopes, 1)
			require.Len(t, hook.scopes[0].responses, 1)
			assert.Equal(t, tt.wantStatus, hook.scopes[0].responses[0].StatusCode)
		})
	}
}

func TestSharedObservabilityMiddleware_ChiNoHookIsNoop(t *testing.T) {
	t.Parallel()

	srv, err := New(noop.NewProvider(), WithTracing(), WithOTelMetrics())
	require.NoError(t, err)
	srv.RegisterRouters(chiTestRouter{handler: func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestFinishObservedRequest_ChiRecordsPanicAndRethrows(t *testing.T) {
	t.Parallel()

	scope := &recordingHTTPRequestScope{}
	rw := newObservedResponseWriter(httptest.NewRecorder())

	assert.Panics(t, func() {
		func() {
			defer finishObservedRequest(scope, rw)
			panic("boom")
		}()
	})

	require.Len(t, scope.errors, 1)
	assert.Contains(t, scope.errors[0].Error(), "panic: boom")
	require.Len(t, scope.responses, 1)
	assert.Equal(t, http.StatusInternalServerError, scope.responses[0].StatusCode)
}

func newObservedChiServer(
	t *testing.T,
	hook *recordingHTTPInstrumentation,
	handler func(http.ResponseWriter, *http.Request),
) *Server {
	t.Helper()

	cfg := common.DefaultConfig()
	cfg.EnableHealthChecks = false
	cfg.EnableTracing = true
	cfg.EnableOTelMetrics = true

	srv, err := New(&observabilityWithHTTP{Provider: noop.NewProvider(), hook: hook}, WithConfig(cfg))
	require.NoError(t, err)
	srv.RegisterRouters(chiTestRouter{handler: handler})
	return srv
}

type chiTestRouter struct {
	handler func(http.ResponseWriter, *http.Request)
}

func (r chiTestRouter) Register(router chi.Router) {
	router.Get("/users/{id}", r.handler)
}

type observabilityWithHTTP struct {
	*noop.Provider
	hook otelobs.HTTPInstrumentation
}

func (o *observabilityWithHTTP) HTTP() otelobs.HTTPInstrumentation {
	return o.hook
}

type recordingHTTPInstrumentation struct {
	requests []otelobs.HTTPRequest
	scopes   []*recordingHTTPRequestScope
}

func (r *recordingHTTPInstrumentation) StartRequest(
	ctx context.Context,
	req otelobs.HTTPRequest,
) (context.Context, otelobs.HTTPRequestScope) {
	scope := &recordingHTTPRequestScope{}
	r.requests = append(r.requests, req)
	r.scopes = append(r.scopes, scope)
	return ctx, scope
}

type recordingHTTPRequestScope struct {
	errors    []error
	responses []otelobs.HTTPResponse
}

func (r *recordingHTTPRequestScope) OnError(err error) {
	if err != nil {
		r.errors = append(r.errors, err)
	}
}

func (r *recordingHTTPRequestScope) Finish(resp otelobs.HTTPResponse) {
	r.responses = append(r.responses, resp)
}

func TestRecordingHTTPRequestScope_RecordsOnlyNonNilErrors(t *testing.T) {
	t.Parallel()

	expected := errors.New("expected")
	scope := &recordingHTTPRequestScope{}
	scope.OnError(nil)
	scope.OnError(expected)

	require.Len(t, scope.errors, 1)
	assert.ErrorIs(t, scope.errors[0], expected)
}

// --- Recover middleware (ported from legacy httpserver/middlewares_test.go) ---

func TestChiServer_RecoverMiddleware_NoPanic(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := recoverMiddleware(provider)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

// --- CORS middleware (ported from legacy httpserver/middlewares_test.go) ---

func TestChiServer_CorsMiddleware_SetsBasicHeaders(t *testing.T) {
	t.Parallel()

	handler := corsMiddleware("*")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Wildcard CORS: Access-Control-Allow-Origin must be "*"
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestChiServer_CorsMiddleware_HandlesPreflight(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := corsMiddleware("https://example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.False(t, handlerCalled, "handler must not be called for preflight")
}

func TestChiServer_CorsMiddleware_SpecificOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		requestOrigin string
		wantStatus    int
		wantOrigin    string
	}{
		{
			name:          "allowed specific origin is echoed back",
			requestOrigin: "https://example.com",
			wantStatus:    http.StatusOK,
			wantOrigin:    "https://example.com",
		},
		{
			name:          "disallowed origin is rejected with 403",
			requestOrigin: "https://evil.com",
			wantStatus:    http.StatusForbidden,
			wantOrigin:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := corsMiddleware("https://example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantOrigin != "" {
				assert.Equal(t, tt.wantOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

// TestChiServer_CorsMiddleware_RejectsCRLFInOrigin covers RF-1.5 mandatory
// scenario: an Origin header with CRLF injection must not be echoed to the
// client. Because the injected origin does not match any allowed origin, the
// middleware returns 403 and never reflects the raw value (runtime path).
func TestChiServer_CorsMiddleware_RejectsCRLFInOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		origin string
	}{
		{name: "CRLF inject via \\r\\n", origin: "https://example.com\r\nX-Injected: evil"},
		{name: "LF inject via \\n", origin: "https://example.com\nX-Injected: evil"},
		{name: "CR inject via \\r", origin: "https://example.com\rX-Injected: evil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := corsMiddleware("https://example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			// Set origin directly on the header map to bypass net/http sanitization
			// that strips CRLF before the middleware sees it via r.Header.Get.
			req.Header["Origin"] = []string{tt.origin}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// The CRLF-injected origin must NOT match the allowed origin,
			// so the middleware must reject it with 403.
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"origin with CRLF must be rejected: %q", tt.origin)

			// The raw injected value must not appear in any response header.
			for k, vals := range rec.Header() {
				for _, v := range vals {
					assert.NotContains(t, v, tt.origin,
						"raw CRLF origin must not be echoed in header %s", k)
				}
			}
		})
	}
}

// --- Security headers middleware (ported from legacy httpserver/middlewares_test.go) ---

func TestChiServer_SecurityHeadersMiddleware_SetsExpectedHeaders(t *testing.T) {
	t.Parallel()

	handler := securityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.expected, rec.Header().Get(tt.header))
		})
	}
}

// --- Body limit middleware (ported from legacy httpserver coverage gap) ---

func TestChiServer_BodyLimitMiddleware_AcceptsWithinLimit(t *testing.T) {
	t.Parallel()

	const limit = 1024
	handler := bodyLimitMiddleware(limit)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.Repeat("a", limit-1)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChiServer_BodyLimitMiddleware_RejectsOversizedBody(t *testing.T) {
	t.Parallel()

	const limit = 64
	handler := bodyLimitMiddleware(limit)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Content-Length exceeds limit — early rejection path.
	body := strings.Repeat("b", limit+1)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
