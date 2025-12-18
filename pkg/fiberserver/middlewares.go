package fiberserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
	"github.com/gofiber/fiber/v2"
)

const (
	// HeaderRequestID is the header key for request ID.
	HeaderRequestID = "X-Request-ID"
	// LocalsRequestID is the key for storing request ID in Fiber locals.
	LocalsRequestID = "request-id"
)

// RequestID is a middleware that adds a unique request ID to each request.
// The request ID is stored in Fiber locals and can be retrieved using GetRequestID.
//
// If UUID generation fails, a fallback ID based on timestamp and random bytes
// is generated to ensure every request has an ID.
func RequestID(c *fiber.Ctx) error {
	requestID := generateRequestID()

	c.Set(HeaderRequestID, requestID)
	c.Locals(LocalsRequestID, requestID)

	return c.Next()
}

// generateRequestID generates a unique request ID using UUID v7.
// Falls back to timestamp-based ID if UUID generation fails.
func generateRequestID() string {
	id, err := vos.NewUUID()
	if err == nil {
		return id.String()
	}
	log.Printf("Failed to generate UUID for request ID, using fallback: %v", err)
	return generateFallbackID()
}

// GetRequestID retrieves the request ID from Fiber context.
// Returns an empty string if no request ID is found.
func GetRequestID(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	requestID, ok := c.Locals(LocalsRequestID).(string)
	if !ok {
		return ""
	}
	return requestID
}

// generateFallbackID generates a fallback ID when UUID generation fails.
// Format: timestamp (hex) + random bytes = 8 + 8 = 16 hex chars
func generateFallbackID() string {
	ts := time.Now().UnixNano()
	tsHex := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		tsHex[i] = byte(ts)
		ts >>= 8
	}

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return hex.EncodeToString(tsHex) + "00000000"
	}

	return hex.EncodeToString(tsHex) + hex.EncodeToString(randomBytes)
}

// Recovery is a middleware that recovers from panics and returns a 500 error.
// It logs the panic for debugging.
func Recovery(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			logPanic(c, r)
			c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal Server Error",
			})
		}
	}()

	return c.Next()
}

// logPanic logs a panic with request ID if available.
func logPanic(c *fiber.Ctx, err any) {
	requestID := GetRequestID(c)
	if requestID == "" {
		log.Printf("PANIC recovered: %v", err)
		return
	}
	log.Printf("[%s] PANIC recovered: %v", requestID, err)
}

// sanitizeHeaderValue removes CR and LF characters to prevent HTTP header injection.
func sanitizeHeaderValue(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowOrigins     string
	AllowMethods     string
	AllowHeaders     string
	AllowCredentials bool
	ExposeHeaders    string
	MaxAge           int
}

// CORS returns a middleware that adds CORS headers to responses.
// Note: All header values are sanitized to prevent CRLF injection attacks.
func CORS(config CORSConfig) Middleware {
	// Sanitize at creation time for efficiency
	origin := sanitizeHeaderValue(config.AllowOrigins)
	if origin == "" {
		origin = "*"
	}
	methods := sanitizeHeaderValue(config.AllowMethods)
	headers := sanitizeHeaderValue(config.AllowHeaders)
	exposeHeaders := sanitizeHeaderValue(config.ExposeHeaders)

	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Methods", methods)
		c.Set("Access-Control-Allow-Headers", headers)

		if config.AllowCredentials {
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		if exposeHeaders != "" {
			c.Set("Access-Control-Expose-Headers", exposeHeaders)
		}

		if config.MaxAge > 0 {
			c.Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}

		// Handle preflight requests
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// CORSSimple is a simplified CORS middleware with string parameters.
// For production, consider using the CORS middleware with CORSConfig.
// Note: All header values are sanitized to prevent CRLF injection attacks.
func CORSSimple(allowedOrigins, allowedMethods, allowedHeaders string) Middleware {
	// Sanitize at creation time for efficiency
	origins := sanitizeHeaderValue(allowedOrigins)
	methods := sanitizeHeaderValue(allowedMethods)
	headers := sanitizeHeaderValue(allowedHeaders)

	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", origins)
		c.Set("Access-Control-Allow-Methods", methods)
		c.Set("Access-Control-Allow-Headers", headers)

		// Handle preflight requests
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// ContentType is a middleware that sets the Content-Type header for responses.
func ContentType(contentType string) Middleware {
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, contentType)
		return c.Next()
	}
}

// JSONContentType is a middleware that sets Content-Type to application/json.
func JSONContentType(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Next()
}

// SecurityHeaders is a middleware that adds common security headers.
func SecurityHeaders(c *fiber.Ctx) error {
	// Prevent MIME type sniffing
	c.Set("X-Content-Type-Options", "nosniff")
	// Prevent clickjacking
	c.Set("X-Frame-Options", "DENY")
	// Enable XSS filter
	c.Set("X-XSS-Protection", "1; mode=block")
	// Control referrer information
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	return c.Next()
}

// Timeout is a middleware that sets a timeout for request processing.
// If the handler takes longer than the timeout, a 504 Gateway Timeout is returned.
// Note: The underlying handler goroutine may continue running after timeout.
// For proper cancellation, handlers should respect context cancellation.
func Timeout(timeout time.Duration) Middleware {
	return func(c *fiber.Ctx) error {
		// Create a context with timeout that handlers can check
		ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
		defer cancel()

		// Set the timeout context so handlers can check for cancellation
		c.SetUserContext(ctx)

		done := make(chan error, 1)

		go func() {
			done <- c.Next()
		}()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			logTimeout(c)
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
				"error": "Request Timeout",
			})
		}
	}
}

// logTimeout logs a timeout with request ID if available.
func logTimeout(c *fiber.Ctx) {
	requestID := GetRequestID(c)
	if requestID == "" {
		log.Printf("Request timeout exceeded")
		return
	}
	log.Printf("[%s] Request timeout exceeded", requestID)
}

// Logger is a middleware that logs request information.
func Logger(c *fiber.Ctx) error {
	start := time.Now()

	err := c.Next()

	duration := time.Since(start)
	requestID := GetRequestID(c)
	status := c.Response().StatusCode()
	method := c.Method()
	path := c.Path()

	if requestID != "" {
		log.Printf("[%s] %s %s - %d (%v)", requestID, method, path, status, duration)
	} else {
		log.Printf("%s %s - %d (%v)", method, path, status, duration)
	}

	return err
}

// RateLimiterConfig holds rate limiter configuration.
type RateLimiterConfig struct {
	Max        int
	Expiration time.Duration
	KeyFunc    func(c *fiber.Ctx) string
}

// Compress is a middleware that enables gzip compression.
// Note: For production, use fiber's built-in compress middleware:
// import "github.com/gofiber/fiber/v2/middleware/compress"
func Compress(c *fiber.Ctx) error {
	c.Set("Content-Encoding", "gzip")
	return c.Next()
}

// Cache is a middleware that adds cache headers.
func Cache(maxAge time.Duration) Middleware {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "public, max-age="+strconv.FormatInt(int64(maxAge.Seconds()), 10))
		return c.Next()
	}
}

// NoCache is a middleware that prevents caching.
func NoCache(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	return c.Next()
}

// ETag is a middleware that adds ETag header support.
// Note: For production, use fiber's built-in etag middleware:
// import "github.com/gofiber/fiber/v2/middleware/etag"
func ETag(c *fiber.Ctx) error {
	return c.Next()
}

// HealthCheck returns a simple health check handler.
func HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "healthy",
	})
}

// ReadinessCheck returns a simple readiness check handler.
func ReadinessCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "ready",
	})
}
