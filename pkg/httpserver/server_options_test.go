package httpserver

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestWithPort(t *testing.T) {
	s := defaultSettings
	s = WithPort("3000")(s)

	if s.port != "3000" {
		t.Errorf("expected port 3000, got %s", s.port)
	}
}

func TestWithReadTimeout(t *testing.T) {
	s := defaultSettings
	s = WithReadTimeout(30 * time.Second)(s)

	if s.readTimeout != 30*time.Second {
		t.Errorf("expected readTimeout 30s, got %v", s.readTimeout)
	}
}

func TestWithWriteTimeout(t *testing.T) {
	s := defaultSettings
	s = WithWriteTimeout(30 * time.Second)(s)

	if s.writeTimeout != 30*time.Second {
		t.Errorf("expected writeTimeout 30s, got %v", s.writeTimeout)
	}
}

func TestWithIdleTimeout(t *testing.T) {
	s := defaultSettings
	s = WithIdleTimeout(120 * time.Second)(s)

	if s.idleTimeout != 120*time.Second {
		t.Errorf("expected idleTimeout 120s, got %v", s.idleTimeout)
	}
}

func TestWithReadHeaderTimeout(t *testing.T) {
	s := defaultSettings
	s = WithReadHeaderTimeout(10 * time.Second)(s)

	if s.readHeaderTimeout != 10*time.Second {
		t.Errorf("expected readHeaderTimeout 10s, got %v", s.readHeaderTimeout)
	}
}

func TestWithMaxHeaderBytes(t *testing.T) {
	s := defaultSettings
	s = WithMaxHeaderBytes(2 << 20)(s)

	if s.maxHeaderBytes != 2<<20 {
		t.Errorf("expected maxHeaderBytes 2MB, got %d", s.maxHeaderBytes)
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	s := defaultSettings
	s = WithShutdownTimeout(60 * time.Second)(s)

	if s.shutdownTimeout != 60*time.Second {
		t.Errorf("expected shutdownTimeout 60s, got %v", s.shutdownTimeout)
	}
}

func TestWithRoutes(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}

	s := defaultSettings
	s = WithRoutes(
		NewRoute(http.MethodGet, "/health", handler),
		NewRoute(http.MethodGet, "/users", handler),
	)(s)

	if len(s.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(s.routes))
	}
}

func TestWithRoutes_Append(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}

	s := defaultSettings
	s = WithRoutes(NewRoute(http.MethodGet, "/health", handler))(s)
	s = WithRoutes(NewRoute(http.MethodGet, "/users", handler))(s)

	if len(s.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(s.routes))
	}
}

func TestWithMiddlewares(t *testing.T) {
	m1 := func(next http.Handler) http.Handler { return next }
	m2 := func(next http.Handler) http.Handler { return next }

	s := defaultSettings
	s = WithMiddlewares(m1, m2)(s)

	if len(s.globalMiddlewares) != 2 {
		t.Errorf("expected 2 middlewares, got %d", len(s.globalMiddlewares))
	}
}

func TestWithMiddlewares_Append(t *testing.T) {
	m1 := func(next http.Handler) http.Handler { return next }
	m2 := func(next http.Handler) http.Handler { return next }

	s := defaultSettings
	s = WithMiddlewares(m1)(s)
	s = WithMiddlewares(m2)(s)

	if len(s.globalMiddlewares) != 2 {
		t.Errorf("expected 2 middlewares, got %d", len(s.globalMiddlewares))
	}
}

func TestWithErrorHandler(t *testing.T) {
	customHandler := func(ctx context.Context, w http.ResponseWriter, err error) {
		w.WriteHeader(http.StatusBadRequest)
	}

	s := defaultSettings
	s = WithErrorHandler(customHandler)(s)

	if s.errorHandler == nil {
		t.Error("expected error handler, got nil")
	}
}

func TestDefaultSettings(t *testing.T) {
	s := defaultSettings

	if s.port != "8080" {
		t.Errorf("expected default port 8080, got %s", s.port)
	}
	if s.readTimeout != 15*time.Second {
		t.Errorf("expected default readTimeout 15s, got %v", s.readTimeout)
	}
	if s.writeTimeout != 15*time.Second {
		t.Errorf("expected default writeTimeout 15s, got %v", s.writeTimeout)
	}
	if s.idleTimeout != 60*time.Second {
		t.Errorf("expected default idleTimeout 60s, got %v", s.idleTimeout)
	}
	if s.readHeaderTimeout != 5*time.Second {
		t.Errorf("expected default readHeaderTimeout 5s, got %v", s.readHeaderTimeout)
	}
	if s.maxHeaderBytes != 1<<20 {
		t.Errorf("expected default maxHeaderBytes 1MB, got %d", s.maxHeaderBytes)
	}
	if s.shutdownTimeout != 30*time.Second {
		t.Errorf("expected default shutdownTimeout 30s, got %v", s.shutdownTimeout)
	}
	if s.errorHandler == nil {
		t.Error("expected default error handler, got nil")
	}
}

func TestMultipleOptions(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}
	middleware := func(next http.Handler) http.Handler { return next }
	errorHandler := func(ctx context.Context, w http.ResponseWriter, err error) {}

	s := defaultSettings
	s = WithPort("3000")(s)
	s = WithReadTimeout(30 * time.Second)(s)
	s = WithWriteTimeout(30 * time.Second)(s)
	s = WithIdleTimeout(120 * time.Second)(s)
	s = WithReadHeaderTimeout(10 * time.Second)(s)
	s = WithMaxHeaderBytes(2 << 20)(s)
	s = WithShutdownTimeout(60 * time.Second)(s)
	s = WithRoutes(NewRoute(http.MethodGet, "/test", handler))(s)
	s = WithMiddlewares(middleware)(s)
	s = WithErrorHandler(errorHandler)(s)

	if s.port != "3000" {
		t.Errorf("expected port 3000, got %s", s.port)
	}
	if s.readTimeout != 30*time.Second {
		t.Errorf("expected readTimeout 30s, got %v", s.readTimeout)
	}
	if s.writeTimeout != 30*time.Second {
		t.Errorf("expected writeTimeout 30s, got %v", s.writeTimeout)
	}
	if s.idleTimeout != 120*time.Second {
		t.Errorf("expected idleTimeout 120s, got %v", s.idleTimeout)
	}
	if s.readHeaderTimeout != 10*time.Second {
		t.Errorf("expected readHeaderTimeout 10s, got %v", s.readHeaderTimeout)
	}
	if s.maxHeaderBytes != 2<<20 {
		t.Errorf("expected maxHeaderBytes 2MB, got %d", s.maxHeaderBytes)
	}
	if s.shutdownTimeout != 60*time.Second {
		t.Errorf("expected shutdownTimeout 60s, got %v", s.shutdownTimeout)
	}
	if len(s.routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(s.routes))
	}
	if len(s.globalMiddlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(s.globalMiddlewares))
	}
}
