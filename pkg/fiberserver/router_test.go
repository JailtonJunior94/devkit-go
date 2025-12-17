package fiberserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRouteGroup_Basic(t *testing.T) {
	srv := New()

	api := srv.Group("/api")
	api.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "ok"})
	})

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/api/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRouteGroup_NestedGroups(t *testing.T) {
	srv := New()

	api := srv.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	users.Get("", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"users": []string{"john", "jane"}})
	})

	users.Get("/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": c.Params("id")})
	})

	app := srv.App()

	// Test list users
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Test get user by id
	req = httptest.NewRequest(fiber.MethodGet, "/api/v1/users/123", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(body, &result)

	if result["id"] != "123" {
		t.Errorf("expected id 123, got %s", result["id"])
	}
}

func TestRouteGroup_AllMethods(t *testing.T) {
	srv := New()

	api := srv.Group("/api")

	api.Get("/resource", func(c *fiber.Ctx) error {
		return c.SendString("GET")
	})
	api.Post("/resource", func(c *fiber.Ctx) error {
		return c.SendString("POST")
	})
	api.Put("/resource", func(c *fiber.Ctx) error {
		return c.SendString("PUT")
	})
	api.Delete("/resource", func(c *fiber.Ctx) error {
		return c.SendString("DELETE")
	})
	api.Patch("/resource", func(c *fiber.Ctx) error {
		return c.SendString("PATCH")
	})
	api.Head("/resource", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	api.Options("/resource", func(c *fiber.Ctx) error {
		return c.SendString("OPTIONS")
	})

	app := srv.App()

	tests := []struct {
		method   string
		expected string
	}{
		{fiber.MethodGet, "GET"},
		{fiber.MethodPost, "POST"},
		{fiber.MethodPut, "PUT"},
		{fiber.MethodDelete, "DELETE"},
		{fiber.MethodPatch, "PATCH"},
		{fiber.MethodOptions, "OPTIONS"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, "/api/resource", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.method, err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("expected status 200 for %s, got %d", tt.method, resp.StatusCode)
		}

		if tt.method != fiber.MethodHead {
			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.expected {
				t.Errorf("expected body %s for %s, got %s", tt.expected, tt.method, string(body))
			}
		}
	}
}

func TestRouteGroup_WithMiddleware(t *testing.T) {
	srv := New()

	middlewareCalled := false
	authMiddleware := func(c *fiber.Ctx) error {
		middlewareCalled = true
		c.Set("X-Auth", "authenticated")
		return c.Next()
	}

	api := srv.Group("/api", authMiddleware)
	api.Get("/protected", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "protected"})
	})

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/api/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !middlewareCalled {
		t.Error("expected middleware to be called")
	}

	if resp.Header.Get("X-Auth") != "authenticated" {
		t.Errorf("expected X-Auth header, got %s", resp.Header.Get("X-Auth"))
	}
}

func TestRouteGroup_NestedMiddleware(t *testing.T) {
	srv := New()

	var sequence []string

	apiMiddleware := func(c *fiber.Ctx) error {
		sequence = append(sequence, "api")
		return c.Next()
	}

	v1Middleware := func(c *fiber.Ctx) error {
		sequence = append(sequence, "v1")
		return c.Next()
	}

	usersMiddleware := func(c *fiber.Ctx) error {
		sequence = append(sequence, "users")
		return c.Next()
	}

	api := srv.Group("/api", apiMiddleware)
	v1 := api.Group("/v1", v1Middleware)
	users := v1.Group("/users", usersMiddleware)

	users.Get("", func(c *fiber.Ctx) error {
		sequence = append(sequence, "handler")
		return c.SendStatus(fiber.StatusOK)
	})

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/users", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"api", "v1", "users", "handler"}
	if len(sequence) != len(expected) {
		t.Fatalf("expected sequence %v, got %v", expected, sequence)
	}

	for i, v := range expected {
		if sequence[i] != v {
			t.Errorf("expected sequence[%d] = %s, got %s", i, v, sequence[i])
		}
	}
}

func TestRouteGroup_Use(t *testing.T) {
	srv := New()

	middlewareCalled := false

	api := srv.Group("/api")
	api.Use(func(c *fiber.Ctx) error {
		middlewareCalled = true
		return c.Next()
	})

	api.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/api/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !middlewareCalled {
		t.Error("expected middleware to be called via Use()")
	}
}

func TestRouteGroup_VersionedAPI(t *testing.T) {
	srv := New()

	// V1 API
	v1 := srv.Group("/api/v1")
	v1.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": "v1", "users": []string{"john"}})
	})

	// V2 API with more data
	v2 := srv.Group("/api/v2")
	v2.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"version": "v2",
			"data":    []fiber.Map{{"id": "1", "name": "john", "email": "john@example.com"}},
			"meta":    fiber.Map{"total": 1},
		})
	})

	app := srv.App()

	// Test v1
	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/users", nil)
	resp, _ := app.Test(req)

	body, _ := io.ReadAll(resp.Body)
	var v1Result map[string]interface{}
	json.Unmarshal(body, &v1Result)

	if v1Result["version"] != "v1" {
		t.Errorf("expected version v1, got %v", v1Result["version"])
	}

	// Test v2
	req = httptest.NewRequest(fiber.MethodGet, "/api/v2/users", nil)
	resp, _ = app.Test(req)

	body, _ = io.ReadAll(resp.Body)
	var v2Result map[string]interface{}
	json.Unmarshal(body, &v2Result)

	if v2Result["version"] != "v2" {
		t.Errorf("expected version v2, got %v", v2Result["version"])
	}

	if v2Result["meta"] == nil {
		t.Error("expected meta in v2 response")
	}
}

func TestRouteGroup_RouteSpecificMiddleware(t *testing.T) {
	srv := New()

	routeMiddlewareCalled := false
	routeMiddleware := func(c *fiber.Ctx) error {
		routeMiddlewareCalled = true
		return c.Next()
	}

	api := srv.Group("/api")
	api.Get("/with-middleware", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}, routeMiddleware)

	api.Get("/without-middleware", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	app := srv.App()

	// Test route with middleware
	req := httptest.NewRequest(fiber.MethodGet, "/api/with-middleware", nil)
	_, _ = app.Test(req)

	if !routeMiddlewareCalled {
		t.Error("expected route middleware to be called")
	}

	// Reset and test route without middleware
	routeMiddlewareCalled = false
	req = httptest.NewRequest(fiber.MethodGet, "/api/without-middleware", nil)
	_, _ = app.Test(req)

	if routeMiddlewareCalled {
		t.Error("route middleware should not be called for other routes")
	}
}

func TestRouteGroup_ErrorHandler(t *testing.T) {
	customErrorHandlerCalled := false

	srv := New(WithErrorHandler(func(c *fiber.Ctx, err error) error {
		customErrorHandlerCalled = true
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}))

	api := srv.Group("/api")
	api.Get("/error", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "custom error")
	})

	app := srv.App()

	req := httptest.NewRequest(fiber.MethodGet, "/api/error", nil)
	resp, _ := app.Test(req)

	if !customErrorHandlerCalled {
		t.Error("expected custom error handler to be called")
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestRouteGroup_CRUD(t *testing.T) {
	srv := New()

	api := srv.Group("/api")
	users := api.Group("/users")

	// CRUD operations
	users.Get("", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"users": []fiber.Map{{"id": "1"}, {"id": "2"}}})
	})

	users.Get("/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": c.Params("id")})
	})

	users.Post("", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": "new"})
	})

	users.Put("/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": c.Params("id"), "updated": true})
	})

	users.Delete("/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	app := srv.App()

	tests := []struct {
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{fiber.MethodGet, "/api/users", "", fiber.StatusOK},
		{fiber.MethodGet, "/api/users/123", "", fiber.StatusOK},
		{fiber.MethodPost, "/api/users", `{"name":"test"}`, fiber.StatusCreated},
		{fiber.MethodPut, "/api/users/123", `{"name":"updated"}`, fiber.StatusOK},
		{fiber.MethodDelete, "/api/users/123", "", fiber.StatusNoContent},
	}

	for _, tt := range tests {
		var req *http.Request
		if tt.body != "" {
			req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(tt.method, tt.path, nil)
		}

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error for %s %s: %v", tt.method, tt.path, err)
		}

		if resp.StatusCode != tt.expectedCode {
			t.Errorf("expected status %d for %s %s, got %d", tt.expectedCode, tt.method, tt.path, resp.StatusCode)
		}
	}
}
