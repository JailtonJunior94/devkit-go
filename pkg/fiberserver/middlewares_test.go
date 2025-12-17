package fiberserver

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRequestID_GeneratesID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID)
	app.Get("/", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		if requestID == "" {
			t.Error("expected request ID, got empty string")
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check X-Request-ID header is set
	headerID := resp.Header.Get(HeaderRequestID)
	if headerID == "" {
		t.Error("expected X-Request-ID header, got empty")
	}
}

func TestRequestID_UniqueIDs(t *testing.T) {
	ids := make(map[string]bool)

	app := fiber.New()
	app.Use(RequestID)
	app.Get("/", func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)
		if ids[requestID] {
			t.Errorf("duplicate request ID: %s", requestID)
		}
		ids[requestID] = true
		return c.SendStatus(fiber.StatusOK)
	})

	// Generate multiple request IDs
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		_, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestGetRequestID_NilContext(t *testing.T) {
	id := GetRequestID(nil)
	if id != "" {
		t.Errorf("expected empty string for nil context, got %s", id)
	}
}

func TestGenerateFallbackID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs
	for i := 0; i < 1000; i++ {
		id := generateFallbackID()
		if ids[id] {
			t.Errorf("duplicate fallback ID: %s", id)
		}
		ids[id] = true

		// Check format (should be hex characters)
		if len(id) != 24 { // 8 bytes timestamp + 4 bytes random = 12 bytes = 24 hex chars
			t.Errorf("expected 24 char fallback ID, got %d chars: %s", len(id), id)
		}
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRecovery_WithPanic(t *testing.T) {
	app := fiber.New()
	app.Use(Recovery)
	app.Get("/", func(c *fiber.Ctx) error {
		panic("test panic")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestRecovery_WithRequestID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID)
	app.Use(Recovery)
	app.Get("/", func(c *fiber.Ctx) error {
		panic("test panic")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestCORSSimple_BasicHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return CORSSimple("*", "GET,POST", "Content-Type")(c)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected origin *, got %s", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.Header.Get("Access-Control-Allow-Methods") != "GET,POST" {
		t.Errorf("expected methods GET,POST, got %s", resp.Header.Get("Access-Control-Allow-Methods"))
	}
	if resp.Header.Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("expected headers Content-Type, got %s", resp.Header.Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSSimple_Preflight(t *testing.T) {
	handlerCalled := false
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return CORSSimple("*", "GET,POST", "Content-Type")(c)
	})
	app.Options("/", func(c *fiber.Ctx) error {
		handlerCalled = true
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodOptions, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}

	if handlerCalled {
		t.Error("handler should not be called for preflight")
	}
}

func TestContentType(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return ContentType("text/plain")(c)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", resp.Header.Get("Content-Type"))
	}
}

func TestJSONContentType(t *testing.T) {
	app := fiber.New()
	app.Use(JSONContentType)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != fiber.MIMEApplicationJSON {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestSecurityHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
		if resp.Header.Get(tt.header) != tt.expected {
			t.Errorf("expected %s: %s, got %s", tt.header, tt.expected, resp.Header.Get(tt.header))
		}
	}
}

func TestTimeout_NoTimeout(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return Timeout(1 * time.Second)(c)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		time.Sleep(10 * time.Millisecond)
		return c.Status(fiber.StatusOK).SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestLogger(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID)
	app.Use(Logger)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNoCache(t *testing.T) {
	app := fiber.New()
	app.Use(NoCache)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "no-store, no-cache, must-revalidate, proxy-revalidate" {
		t.Errorf("expected Cache-Control no-store..., got %s", cacheControl)
	}

	pragma := resp.Header.Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %s", pragma)
	}

	expires := resp.Header.Get("Expires")
	if expires != "0" {
		t.Errorf("expected Expires 0, got %s", expires)
	}
}

func TestHealthCheck(t *testing.T) {
	app := fiber.New()
	app.Get("/health", HealthCheck)

	req := httptest.NewRequest(fiber.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestReadinessCheck(t *testing.T) {
	app := fiber.New()
	app.Get("/ready", ReadinessCheck)

	req := httptest.NewRequest(fiber.MethodGet, "/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMiddlewareChain(t *testing.T) {
	var sequence []string

	m1 := func(c *fiber.Ctx) error {
		sequence = append(sequence, "m1-before")
		err := c.Next()
		sequence = append(sequence, "m1-after")
		return err
	}

	m2 := func(c *fiber.Ctx) error {
		sequence = append(sequence, "m2-before")
		err := c.Next()
		sequence = append(sequence, "m2-after")
		return err
	}

	app := fiber.New()
	app.Use(m1, m2)
	app.Get("/", func(c *fiber.Ctx) error {
		sequence = append(sequence, "handler")
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(sequence) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(sequence), sequence)
	}
	for i, v := range expected {
		if sequence[i] != v {
			t.Errorf("expected sequence[%d] = %s, got %s", i, v, sequence[i])
		}
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return CORSSimple("https://example.com", "GET,POST,PUT,DELETE", "Content-Type,Authorization")(c)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("expected origin https://example.com, got %s", origin)
	}
}

func TestCORS_WithConfig(t *testing.T) {
	config := CORSConfig{
		AllowOrigins:     "https://example.com",
		AllowMethods:     "GET,POST",
		AllowHeaders:     "Content-Type",
		AllowCredentials: true,
		ExposeHeaders:    "X-Custom-Header",
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		return CORS(config)(c)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected origin https://example.com, got %s", resp.Header.Get("Access-Control-Allow-Origin"))
	}

	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected credentials true, got %s", resp.Header.Get("Access-Control-Allow-Credentials"))
	}

	if resp.Header.Get("Access-Control-Expose-Headers") != "X-Custom-Header" {
		t.Errorf("expected expose headers X-Custom-Header, got %s", resp.Header.Get("Access-Control-Expose-Headers"))
	}
}
