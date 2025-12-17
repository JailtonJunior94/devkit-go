package fiberserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestNew_DefaultSettings(t *testing.T) {
	srv := New()
	if srv == nil {
		t.Fatal("expected server, got nil")
	}

	s := srv.(*server)
	if s.port != "8080" {
		t.Errorf("expected default port 8080, got %s", s.port)
	}
}

func TestNew_WithOptions(t *testing.T) {
	srv := New(
		WithPort("3000"),
		WithReadTimeout(30*time.Second),
		WithWriteTimeout(30*time.Second),
		WithIdleTimeout(120*time.Second),
		WithBodyLimit(8*1024*1024),
	)

	s := srv.(*server)
	if s.port != "3000" {
		t.Errorf("expected port 3000, got %s", s.port)
	}
}

func TestNew_WithRoutes(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithRoutes(
			NewRoute(fiber.MethodGet, "/health", handler),
			NewRoute(fiber.MethodGet, "/users", handler),
		),
	)

	app := srv.App()

	// Test health endpoint
	req := httptest.NewRequest(fiber.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Test users endpoint
	req = httptest.NewRequest(fiber.MethodGet, "/users", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNew_WithMiddlewares(t *testing.T) {
	middlewareCalled := false
	middleware := func(c *fiber.Ctx) error {
		middlewareCalled = true
		return c.Next()
	}

	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithMiddlewares(middleware),
		WithRoutes(NewRoute(fiber.MethodGet, "/test", handler)),
	)

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !middlewareCalled {
		t.Error("expected middleware to be called")
	}
}

func TestNew_WithErrorHandler(t *testing.T) {
	expectedErr := errors.New("test error")
	handlerErrorCalled := false

	customErrorHandler := func(c *fiber.Ctx, err error) error {
		handlerErrorCalled = true
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	handler := func(c *fiber.Ctx) error {
		return expectedErr
	}

	srv := New(
		WithErrorHandler(customErrorHandler),
		WithRoutes(NewRoute(fiber.MethodGet, "/error", handler)),
	)

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !handlerErrorCalled {
		t.Error("expected error handler to be called")
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestRegisterRoute_AfterNew(t *testing.T) {
	srv := New()

	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	}

	// Register route after New()
	srv.RegisterRoute(NewRoute(fiber.MethodPost, "/dynamic", handler))

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodPost, "/dynamic", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestNewRoute(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return nil
	}

	middleware := func(c *fiber.Ctx) error {
		return c.Next()
	}

	route := NewRoute(fiber.MethodGet, "/users/:id", handler, middleware)

	if route.Method != fiber.MethodGet {
		t.Errorf("expected method GET, got %s", route.Method)
	}
	if route.Path != "/users/:id" {
		t.Errorf("expected path /users/:id, got %s", route.Path)
	}
	if route.Handler == nil {
		t.Error("expected handler, got nil")
	}
	if len(route.Middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(route.Middlewares))
	}
}

func TestDefaultHandleError(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return errors.New("test error")
	}

	srv := New(
		WithRoutes(NewRoute(fiber.MethodGet, "/error", handler)),
	)

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default error handler should return 500
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestHandler_ReturnsNil(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	}

	srv := New(
		WithRoutes(NewRoute(fiber.MethodGet, "/health", handler)),
	)

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(body, &result)

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
	routeMiddleware := func(c *fiber.Ctx) error {
		routeMiddlewareCalled = true
		return c.Next()
	}

	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithRoutes(
			NewRoute(fiber.MethodGet, "/with-middleware", handler, routeMiddleware),
			NewRoute(fiber.MethodGet, "/without-middleware", handler),
		),
	)

	app := srv.App()

	// Request to route with middleware
	req := httptest.NewRequest(fiber.MethodGet, "/with-middleware", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !routeMiddlewareCalled {
		t.Error("expected route middleware to be called")
	}

	// Reset and request to route without middleware
	routeMiddlewareCalled = false
	req = httptest.NewRequest(fiber.MethodGet, "/without-middleware", nil)
	_, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if routeMiddlewareCalled {
		t.Error("route middleware should not be called for other routes")
	}
}

func TestNotFound(t *testing.T) {
	srv := New()

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithRoutes(NewRoute(fiber.MethodGet, "/users", handler)),
	)

	app := srv.App()

	// Try POST on GET-only route
	req := httptest.NewRequest(fiber.MethodPost, "/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fiber returns 404 for method not matched (different from chi which returns 405)
	// This is expected Fiber behavior
	if resp.StatusCode != fiber.StatusNotFound && resp.StatusCode != fiber.StatusMethodNotAllowed {
		t.Errorf("expected status 404 or 405, got %d", resp.StatusCode)
	}
}

func TestAllHTTPMethods(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithRoutes(
			NewRoute(fiber.MethodGet, "/get", handler),
			NewRoute(fiber.MethodPost, "/post", handler),
			NewRoute(fiber.MethodPut, "/put", handler),
			NewRoute(fiber.MethodDelete, "/delete", handler),
			NewRoute(fiber.MethodPatch, "/patch", handler),
			NewRoute(fiber.MethodHead, "/head", handler),
			NewRoute(fiber.MethodOptions, "/options", handler),
		),
	)

	app := srv.App()

	tests := []struct {
		method string
		path   string
	}{
		{fiber.MethodGet, "/get"},
		{fiber.MethodPost, "/post"},
		{fiber.MethodPut, "/put"},
		{fiber.MethodDelete, "/delete"},
		{fiber.MethodPatch, "/patch"},
		{fiber.MethodHead, "/head"},
		{fiber.MethodOptions, "/options"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error for %s %s: %v", tt.method, tt.path, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("expected status 200 for %s %s, got %d", tt.method, tt.path, resp.StatusCode)
		}
	}
}

func TestWithPrefork(t *testing.T) {
	srv := New(WithPrefork(true))
	if srv == nil {
		t.Fatal("expected server, got nil")
	}
}

func TestWithStrictRouting(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithStrictRouting(true),
		WithRoutes(NewRoute(fiber.MethodGet, "/test", handler)),
	)

	app := srv.App()

	// /test should work
	req := httptest.NewRequest(fiber.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// /test/ should NOT work with strict routing
	req = httptest.NewRequest(fiber.MethodGet, "/test/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404 for /test/ with strict routing, got %d", resp.StatusCode)
	}
}

func TestWithCaseSensitive(t *testing.T) {
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	srv := New(
		WithCaseSensitive(true),
		WithRoutes(NewRoute(fiber.MethodGet, "/Test", handler)),
	)

	app := srv.App()

	// /Test should work
	req := httptest.NewRequest(fiber.MethodGet, "/Test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// /test should NOT work with case sensitive routing
	req = httptest.NewRequest(fiber.MethodGet, "/test", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404 for /test with case sensitive routing, got %d", resp.StatusCode)
	}
}
