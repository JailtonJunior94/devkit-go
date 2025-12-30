package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_DefaultSettings(t *testing.T) {
	srv := New()
	if srv == nil {
		t.Fatal("expected server, got nil")
	}

	// Check default configuration via internal struct
	s := srv.(*server)
	if s.Server.Addr != ":8080" {
		t.Errorf("expected default port 8080, got %s", s.Server.Addr)
	}
	if s.Server.ReadTimeout != 15*time.Second {
		t.Errorf("expected ReadTimeout 15s, got %v", s.Server.ReadTimeout)
	}
	if s.Server.WriteTimeout != 15*time.Second {
		t.Errorf("expected WriteTimeout 15s, got %v", s.Server.WriteTimeout)
	}
	if s.Server.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout 60s, got %v", s.Server.IdleTimeout)
	}
	if s.Server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ReadHeaderTimeout 5s, got %v", s.Server.ReadHeaderTimeout)
	}
	if s.Server.MaxHeaderBytes != 1<<20 {
		t.Errorf("expected MaxHeaderBytes 1MB, got %d", s.Server.MaxHeaderBytes)
	}
}

func TestNew_WithOptions(t *testing.T) {
	srv := New(
		WithPort("3000"),
		WithReadTimeout(30*time.Second),
		WithWriteTimeout(30*time.Second),
		WithIdleTimeout(120*time.Second),
		WithReadHeaderTimeout(10*time.Second),
		WithMaxHeaderBytes(2<<20),
	)

	s := srv.(*server)
	if s.Server.Addr != ":3000" {
		t.Errorf("expected port 3000, got %s", s.Server.Addr)
	}
	if s.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", s.Server.ReadTimeout)
	}
	if s.Server.WriteTimeout != 30*time.Second {
		t.Errorf("expected WriteTimeout 30s, got %v", s.Server.WriteTimeout)
	}
	if s.Server.IdleTimeout != 120*time.Second {
		t.Errorf("expected IdleTimeout 120s, got %v", s.Server.IdleTimeout)
	}
	if s.Server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("expected ReadHeaderTimeout 10s, got %v", s.Server.ReadHeaderTimeout)
	}
	if s.Server.MaxHeaderBytes != 2<<20 {
		t.Errorf("expected MaxHeaderBytes 2MB, got %d", s.Server.MaxHeaderBytes)
	}
}

func TestNew_WithRoutes(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	srv := New(
		WithRoutes(
			NewRoute(http.MethodGet, "/health", handler),
			NewRoute(http.MethodGet, "/users", handler),
		),
	)

	// Test health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Test users endpoint
	req = httptest.NewRequest(http.MethodGet, "/users", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestNew_WithMiddlewares(t *testing.T) {
	middlewareCalled := false
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			middlewareCalled = true
			next.ServeHTTP(w, r)
		})
	}

	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	srv := New(
		WithMiddlewares(middleware),
		WithRoutes(NewRoute(http.MethodGet, "/test", handler)),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if !middlewareCalled {
		t.Error("expected middleware to be called")
	}
}

func TestNew_WithErrorHandler(t *testing.T) {
	expectedErr := errors.New("test error")
	handlerErrorCalled := false

	customErrorHandler := func(ctx context.Context, w http.ResponseWriter, err error) {
		handlerErrorCalled = true
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
		w.WriteHeader(http.StatusBadRequest)
	}

	handler := func(w http.ResponseWriter, r *http.Request) error {
		return expectedErr
	}

	srv := New(
		WithErrorHandler(customErrorHandler),
		WithRoutes(NewRoute(http.MethodGet, "/error", handler)),
	)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if !handlerErrorCalled {
		t.Error("expected error handler to be called")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestRegisterRoute_AfterNew(t *testing.T) {
	srv := New()

	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		return nil
	}

	// Register route after New()
	srv.RegisterRoute(NewRoute(http.MethodPost, "/dynamic", handler))

	req := httptest.NewRequest(http.MethodPost, "/dynamic", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
}

func TestNewRoute(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}

	middleware := func(next http.Handler) http.Handler {
		return next
	}

	route := NewRoute(http.MethodGet, "/users/{id}", handler, middleware)

	if route.Method != http.MethodGet {
		t.Errorf("expected method GET, got %s", route.Method)
	}
	if route.Path != "/users/{id}" {
		t.Errorf("expected path /users/{id}, got %s", route.Path)
	}
	if route.Handler == nil {
		t.Error("expected handler, got nil")
	}
	if len(route.Middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(route.Middlewares))
	}
}

func TestMiddlewares_Order(t *testing.T) {
	var order []int

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 1)
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 2)
			next.ServeHTTP(w, r)
		})
	}
	m3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 3)
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, 0) // Handler is last
	})

	wrapped := Middlewares(handler, m1, m2, m3)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Middlewares should execute in order: m1 -> m2 -> m3 -> handler
	expected := []int{1, 2, 3, 0}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %d, got %d", i, v, order[i])
		}
	}
}

func TestDefaultHandleError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("test error")
	}

	srv := New(
		WithRoutes(NewRoute(http.MethodGet, "/error", handler)),
	)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Default error handler should return 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestHandler_ReturnsNil(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return nil
	}

	srv := New(
		WithRoutes(NewRoute(http.MethodGet, "/health", handler)),
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var result map[string]string
	_ = json.Unmarshal(body, &result)

	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestShutdownListener(t *testing.T) {
	srv := New()

	ch := srv.ShutdownListener()
	if ch == nil {
		t.Fatal("expected channel, got nil")
	}

	// Check it's buffered
	select {
	case ch <- nil:
		// Channel is buffered, good
		<-ch // Clean up
	default:
		t.Error("expected buffered channel")
	}
}

func TestGetShutdownTimeout(t *testing.T) {
	ctx, cancel := GetShutdownTimeout()
	defer cancel()

	if ctx == nil {
		t.Fatal("expected context, got nil")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	// Should be approximately 30 seconds from now
	remaining := time.Until(deadline)
	if remaining < 29*time.Second || remaining > 31*time.Second {
		t.Errorf("expected deadline ~30s from now, got %v", remaining)
	}
}

func TestRouteSpecificMiddleware(t *testing.T) {
	routeMiddlewareCalled := false
	routeMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeMiddlewareCalled = true
			next.ServeHTTP(w, r)
		})
	}

	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	srv := New(
		WithRoutes(
			NewRoute(http.MethodGet, "/with-middleware", handler, routeMiddleware),
			NewRoute(http.MethodGet, "/without-middleware", handler),
		),
	)

	// Request to route with middleware
	req := httptest.NewRequest(http.MethodGet, "/with-middleware", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if !routeMiddlewareCalled {
		t.Error("expected route middleware to be called")
	}

	// Reset and request to route without middleware
	routeMiddlewareCalled = false
	req = httptest.NewRequest(http.MethodGet, "/without-middleware", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if routeMiddlewareCalled {
		t.Error("route middleware should not be called for other routes")
	}
}

func TestNotFound(t *testing.T) {
	srv := New()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	srv := New(
		WithRoutes(NewRoute(http.MethodGet, "/users", handler)),
	)

	// Try POST on GET-only route
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Chi returns 405 for method not allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}
