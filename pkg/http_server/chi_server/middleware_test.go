package chiserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
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
			wantRoute:         "/users/{param}",
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
			wantRoute:  "/users/{param}",
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
