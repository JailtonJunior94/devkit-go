package chiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, provider *fake.Provider, opts ...Option) *Server {
	t.Helper()
	defaults := []Option{
		WithServiceName("test"),
		WithServiceVersion("0.0.0"),
		WithEnvironment("test"),
	}
	defaults = append(defaults, opts...)
	srv, err := New(provider, defaults...)
	require.NoError(t, err)
	return srv
}

func TestChiServer_AdaptHandler_PassesErrorToHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler Handler
		wantErr string
	}{
		{
			name:    "propagates non-nil error",
			handler: func(http.ResponseWriter, *http.Request) error { return errors.New("db down") },
			wantErr: "db down",
		},
		{
			name: "propagates wrapped error",
			handler: func(http.ResponseWriter, *http.Request) error {
				return errors.Join(errors.New("outer"), errors.New("inner"))
			},
			wantErr: "outer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var capturedErr error
			eh := func(_ context.Context, _ http.ResponseWriter, err error) {
				capturedErr = err
			}

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)

			adaptHandler(tc.handler, eh).ServeHTTP(rr, req)

			require.Error(t, capturedErr)
			assert.Contains(t, capturedErr.Error(), tc.wantErr)
		})
	}
}

func TestChiServer_AdaptHandler_NilErrorIsNoop(t *testing.T) {
	t.Parallel()

	called := false
	eh := func(_ context.Context, _ http.ResponseWriter, _ error) {
		called = true
	}
	h := func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	adaptHandler(h, eh).ServeHTTP(rr, req)

	assert.False(t, called, "ErrorHandler must not be invoked when handler returns nil")
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestChiServer_AdaptHandler_StashesPathInContext(t *testing.T) {
	t.Parallel()

	var got string
	eh := func(ctx context.Context, _ http.ResponseWriter, _ error) {
		got = requestPath(ctx)
	}
	h := func(http.ResponseWriter, *http.Request) error {
		return errors.New("boom")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	adaptHandler(h, eh).ServeHTTP(rr, req)

	assert.Equal(t, "/users/42", got)
}

func TestChiServer_RegisterHandler_RegistersWithRouter(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	srv.RegisterHandler(http.MethodGet, "/ping", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "pong", rr.Body.String())
}

func TestChiServer_RegisterHandler_AppliesMiddlewaresInOrder(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	var order []string

	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "before:"+name)
				next.ServeHTTP(w, r)
				order = append(order, "after:"+name)
			})
		}
	}

	srv.RegisterHandler(
		http.MethodGet,
		"/chain",
		func(w http.ResponseWriter, _ *http.Request) error {
			order = append(order, "handler")
			w.WriteHeader(http.StatusOK)
			return nil
		},
		mw("a"),
		mw("b"),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chain", nil)
	srv.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t,
		[]string{"before:a", "before:b", "handler", "after:b", "after:a"},
		order,
		"first middleware in declaration order must be the outermost wrapper",
	)
}

func TestChiServer_RegisterHandler_UsesRouteTimeoutHook(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	// Stub wrapWithTimeout is a no-op in 4.0; the real behaviour is
	// validated in 5.0. Here we just verify that a per-route timeout
	// configuration does not break registration through RegisterHandler.
	srv := newTestServer(t, provider, WithRouteTimeout("/slow", 5*time.Second))

	called := false
	srv.RegisterHandler(http.MethodGet, "/slow", func(w http.ResponseWriter, _ *http.Request) error {
		called = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	srv.router.ServeHTTP(rr, req)

	assert.True(t, called, "route timeout stub must not block the handler in 4.0")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestChiServer_WithErrorHandler_OverridesDefault(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()

	customCalled := false
	custom := func(_ context.Context, w http.ResponseWriter, err error) {
		customCalled = true
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(err.Error()))
	}

	srv := newTestServer(t, provider, WithErrorHandler(custom))
	srv.RegisterHandler(http.MethodGet, "/x", func(http.ResponseWriter, *http.Request) error {
		return errors.New("custom path")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	srv.router.ServeHTTP(rr, req)

	assert.True(t, customCalled, "custom ErrorHandler must replace the default")
	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.Equal(t, "custom path", rr.Body.String())
}

func TestChiServer_DefaultErrorHandler_UsesProblemFromError(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)
	srv.RegisterHandler(http.MethodGet, "/boom", func(http.ResponseWriter, *http.Request) error {
		return errors.New("db down")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))

	var body common.ProblemDetail
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, http.StatusInternalServerError, body.Status)
	assert.Equal(t, "internal server error", body.Detail)
	assert.Equal(t, "/boom", body.Instance)
	assert.NotEmpty(t, body.Type)
}

func TestChiServer_DefaultErrorHandler_DoesNotLeakRawError(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	const secret = "user=alice password=hunter2"

	srv.RegisterHandler(http.MethodGet, "/leak", func(http.ResponseWriter, *http.Request) error {
		return errors.New(secret)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/leak", nil)
	srv.router.ServeHTTP(rr, req)

	assert.NotContains(t, rr.Body.String(), secret, "raw err.Error() must not be reflected to the client")
}

func TestChiServer_DefaultErrorHandler_LogsOriginalErrorWithRequestID(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	const errMsg = "downstream timeout"
	srv.RegisterHandler(http.MethodGet, "/log", func(http.ResponseWriter, *http.Request) error {
		return errors.New(errMsg)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	req.Header.Set("X-Request-ID", "req-abc")
	srv.router.ServeHTTP(rr, req)

	logger, ok := provider.Logger().(*fake.FakeLogger)
	require.True(t, ok)

	entries := logger.GetEntries()
	var found *fake.LogEntry
	for i := range entries {
		if entries[i].Level == observability.LogLevelError && entries[i].Message == "http handler error" {
			found = &entries[i]
			break
		}
	}
	require.NotNil(t, found, "default ErrorHandler must emit an error-level log")

	fields := indexFields(found.Fields)

	gotErr, ok := fields["error"]
	require.True(t, ok, "log must include the original error field")
	if e, ok := gotErr.(error); ok {
		assert.EqualError(t, e, errMsg)
	} else {
		assert.Equal(t, errMsg, gotErr)
	}

	assert.Equal(t, "req-abc", fields["request_id"])
	assert.Equal(t, "/log", fields["path"])
}

func TestChiServer_WriteErrorResponse_UsesProblemFromError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey, "rid-1"))

	writeErrorResponse(rr, req, errors.New("internal blew up"))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))
	assert.NotContains(t, rr.Body.String(), "internal blew up", "raw err must not leak in body")

	var body common.ProblemDetail
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, http.StatusInternalServerError, body.Status)
	assert.Equal(t, "internal server error", body.Detail)
	assert.Equal(t, "/foo", body.Instance)
	assert.Equal(t, "rid-1", body.RequestID)
}

// TestChiServer_RegisterHandler_DuplicateRoute_LastWins documents chi's
// idempotency behavior (RF-1.5 idempotency scenario). chi.Router.Method does
// NOT panic on duplicate registration — it silently replaces the handler and
// the last registration wins. This differs from frameworks that panic on
// conflicts; consumers must be aware that registering the same route twice
// overwrites the first handler without an error.
func TestChiServer_RegisterHandler_DuplicateRoute_LastWins(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	// First handler returns 200.
	srv.RegisterHandler(http.MethodGet, "/dup", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// Second registration for the same route returns 201.
	srv.RegisterHandler(http.MethodGet, "/dup", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		return nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dup", nil)
	srv.router.ServeHTTP(rr, req)

	// chi uses last-wins semantics: the second handler (201) takes effect.
	assert.Equal(t, http.StatusCreated, rr.Code,
		"duplicate route registration must silently replace the previous handler (last-wins)")
}

func TestChiServer_RegisterRouters_StillWorks(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	srv.RegisterRouters(stubRouter{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/legacy", nil)
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "legacy", strings.TrimSpace(rr.Body.String()))
}

type stubRouter struct{}

func (stubRouter) Register(r chi.Router) {
	r.Get("/legacy", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("legacy"))
	})
}

func indexFields(fields []observability.Field) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		out[f.Key] = f.AnyValue()
	}
	return out
}
