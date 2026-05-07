package chiserver

// server_legacy_test.go ports server-level and options scenarios from
// the removed httpserver package that had no direct equivalent in chi_server/
// after tasks 4.0–6.0. All tests use the chi_server API (RegisterHandler,
// WithMiddleware, etc.) and do not import the deleted legacy package (RF-1.5).

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Options (ported from server_options_test.go) ---

func TestChiServer_WithPort_SetsAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		port string
		want string
	}{
		{name: "plain port", port: "3000", want: ":3000"},
		{name: "port with colon prefix", port: ":4000", want: ":4000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, err := New(fake.NewProvider(),
				WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
				WithPort(tt.port),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want, srv.config.Address)
		})
	}
}

func TestChiServer_WithReadTimeout_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
		WithReadTimeout(30*time.Second),
	)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, srv.config.ReadTimeout)
}

func TestChiServer_WithWriteTimeout_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
		WithWriteTimeout(45*time.Second),
	)
	require.NoError(t, err)
	assert.Equal(t, 45*time.Second, srv.config.WriteTimeout)
}

func TestChiServer_WithIdleTimeout_PropagatesToConfig(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
		WithIdleTimeout(120*time.Second),
	)
	require.NoError(t, err)
	assert.Equal(t, 120*time.Second, srv.config.IdleTimeout)
}

func TestChiServer_WithMiddleware_IsApplied(t *testing.T) {
	t.Parallel()

	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
		WithMiddleware(mw),
	)
	require.NoError(t, err)

	srv.RegisterHandler(http.MethodGet, "/test", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.True(t, called, "custom middleware must be invoked on every request")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChiServer_WithMiddleware_AppendMultiple(t *testing.T) {
	t.Parallel()

	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}

	srv, err := New(fake.NewProvider(),
		WithServiceName("test"), WithServiceVersion("0"), WithEnvironment("test"),
		WithMiddleware(mw1),
		WithMiddleware(mw2),
	)
	require.NoError(t, err)

	srv.RegisterHandler(http.MethodGet, "/chain", func(w http.ResponseWriter, _ *http.Request) error {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/chain", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"mw1", "mw2", "handler"}, order)
}

// --- Default config (ported from TestDefaultSettings / TestNew_DefaultSettings) ---

func TestChiServer_DefaultConfig_HasExpectedValues(t *testing.T) {
	t.Parallel()

	cfg := common.DefaultConfig()

	assert.Equal(t, ":8080", cfg.Address)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.IdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
	assert.Positive(t, cfg.BodyLimit)
}

func TestChiServer_New_DefaultConfig(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("svc"), WithServiceVersion("1.0"), WithEnvironment("prod"),
	)
	require.NoError(t, err)
	require.NotNil(t, srv)

	assert.Equal(t, ":8080", srv.config.Address)
	assert.Equal(t, 30*time.Second, srv.config.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.config.WriteTimeout)
	assert.Equal(t, 120*time.Second, srv.config.IdleTimeout)
	assert.Equal(t, 30*time.Second, srv.config.ShutdownTimeout)
}

func TestChiServer_MultipleOptions_CombinedEffect(t *testing.T) {
	t.Parallel()

	srv, err := New(fake.NewProvider(),
		WithServiceName("my-service"),
		WithServiceVersion("2.0"),
		WithEnvironment("staging"),
		WithPort("9090"),
		WithReadTimeout(10*time.Second),
		WithWriteTimeout(20*time.Second),
		WithIdleTimeout(60*time.Second),
		WithShutdownTimeout(15*time.Second),
	)
	require.NoError(t, err)

	assert.Equal(t, ":9090", srv.config.Address)
	assert.Equal(t, 10*time.Second, srv.config.ReadTimeout)
	assert.Equal(t, 20*time.Second, srv.config.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.config.IdleTimeout)
	assert.Equal(t, 15*time.Second, srv.config.ShutdownTimeout)
}

// --- Server routing behavior (ported from server_test.go) ---

func TestChiServer_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChiServer_MethodNotAllowed_Returns405(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	srv.RegisterHandler(http.MethodGet, "/users", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestChiServer_RouteSpecificMiddleware_IsApplied(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	srv := newTestServer(t, provider)

	routeMiddlewareCalled := false
	routeMW := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeMiddlewareCalled = true
			next.ServeHTTP(w, r)
		})
	})

	h := func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	srv.RegisterHandler(http.MethodGet, "/with-middleware", h, routeMW)
	srv.RegisterHandler(http.MethodGet, "/without-middleware", h)

	// Route with middleware: expect middleware to be called.
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/with-middleware", nil))
	assert.True(t, routeMiddlewareCalled)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Route without middleware: middleware must NOT be called.
	routeMiddlewareCalled = false
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/without-middleware", nil))
	assert.False(t, routeMiddlewareCalled, "route middleware must not spill to other routes")
}
