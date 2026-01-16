package serverfiber

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// recoverMiddleware recovers from panics and logs detailed error information.
// It captures the stack trace and logs it via observability for debugging.
// SECURITY: Prevents double write by checking if headers were already sent.
func recoverMiddleware(o11y observability.Observability) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())

				fields := []observability.Field{
					observability.String("ip", c.IP()),
					observability.String("path", c.Path()),
					observability.String("method", c.Method()),
					observability.String("stack", stack),
					observability.Any("panic", r),
				}

				if requestID, ok := c.Locals("requestID").(string); ok {
					fields = append(fields, observability.String("request_id", requestID))
				}

				o11y.Logger().Error(c.UserContext(), "recovered from panic in HTTP handler", fields...)

				// Check if response was already sent (status code != 0 means headers sent)
				// In Fiber, if status is 0, headers haven't been sent yet
				if c.Response().StatusCode() == 0 {
					_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"code":    fiber.StatusInternalServerError,
						"message": "Internal Server Error",
					})
				} else {
					// Headers already sent, cannot send error response
					var reqID string
					if rid, ok := c.Locals("requestID").(string); ok {
						reqID = rid
					}
					o11y.Logger().Warn(c.UserContext(),
						"cannot send panic error response: headers already sent",
						observability.String("request_id", reqID),
					)
				}
			}
		}()

		return c.Next()
	}
}

func requestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")

		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Locals("requestID", requestID)
		c.Set("X-Request-ID", requestID)

		return c.Next()
	}
}

// timeoutMiddleware applies request timeout with proper cleanup.
// IMPORTANT: Handlers MUST respect context cancellation to prevent goroutine leaks.
// When ctx.Done() is signaled, handlers should stop processing immediately.
func timeoutMiddleware(globalTimeout time.Duration, routeTimeouts map[string]time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timeout := globalTimeout
		if routeTimeouts != nil {
			if routeTimeout, ok := routeTimeouts[c.Path()]; ok {
				timeout = routeTimeout
			}
		}

		ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
		defer cancel()

		// Set context with timeout so handlers can check ctx.Done()
		c.SetUserContext(ctx)

		type result struct {
			err error
		}

		// Buffered channel prevents goroutine leak if we timeout before handler finishes
		resultChan := make(chan result, 1)

		go func() {
			defer func() {
				// Recover any panics from the handler
				if recovered := recover(); recovered != nil {
					// Re-panic will be caught by the outer recover middleware
					panic(recovered)
				}
			}()

			err := c.Next()
			// Non-blocking send (due to buffered channel)
			select {
			case resultChan <- result{err: err}:
				// Result sent successfully
			case <-ctx.Done():
				// Context cancelled, don't block
			}
		}()

		select {
		case res := <-resultChan:
			// Handler completed within timeout
			return res.err
		case <-ctx.Done():
			// Timeout exceeded - context cancellation signals handler to stop

			// CRITICAL: Wait a short time for goroutine cleanup
			// This gives the handler time to respect context cancellation
			// Most well-behaved handlers will stop within 100ms
			cleanupTimer := time.NewTimer(100 * time.Millisecond)
			defer cleanupTimer.Stop()

			select {
			case <-resultChan:
				// Handler finished cleanup successfully
			case <-cleanupTimer.C:
				// Handler didn't respect context cancellation
				// This is a handler bug, but we can't do much more
			}

			// Return timeout error to client
			return fiber.NewError(fiber.StatusRequestTimeout, "request timeout exceeded")
		}
	}
}

// securityHeadersMiddleware adds comprehensive security headers to responses.
// Uses common.SecurityHeaders for centralized security configuration.
func securityHeadersMiddleware() fiber.Handler {
	// Initialize security headers once (reuse for all requests)
	securityHeaders := common.DefaultSecurityHeaders()
	headersMap := securityHeaders.ToMap()

	return func(c *fiber.Ctx) error {
		// Apply all security headers
		for key, value := range headersMap {
			c.Set(key, value)
		}

		return c.Next()
	}
}

func corsMiddleware(origins string) fiber.Handler {
	// Parse allowed origins
	allowedOrigins := parseOrigins(origins)

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")

		// If no Origin header, skip CORS
		if origin == "" {
			return c.Next()
		}

		// Validate if origin is allowed
		if !isOriginAllowed(origin, allowedOrigins) {
			return fiber.NewError(fiber.StatusForbidden, "origin not allowed")
		}

		// SECURITY: Never use wildcard (*) with credentials
		// If wildcard is needed, it must be set explicitly and credentials disabled
		if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
			c.Set("Access-Control-Allow-Origin", "*")
			// Do NOT set Access-Control-Allow-Credentials with wildcard
		} else {
			// Set specific origin (not wildcard)
			c.Set("Access-Control-Allow-Origin", origin)
			// Credentials can be allowed with specific origins
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Set("Access-Control-Max-Age", "3600")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// parseOrigins splits comma-separated origins
func parseOrigins(origins string) []string {
	if origins == "" {
		return []string{}
	}

	// Handle wildcard
	if origins == "*" {
		return []string{"*"}
	}

	// Split by comma and trim spaces
	var result []string
	for _, origin := range splitAndTrim(origins, ",") {
		if origin != "" {
			result = append(result, origin)
		}
	}

	return result
}

// isOriginAllowed checks if the origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	// Check for wildcard
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return true
	}

	// Check exact match
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}

// splitAndTrim splits a string by delimiter and trims each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	current := ""

	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}

	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// otelMetricsMiddleware creates OpenTelemetry HTTP metrics for the server.
// It records request duration, request count, and active requests.
// Metrics follow OpenTelemetry Semantic Conventions for HTTP metrics.
//
// PERFORMANCE: Instruments are created once and reused for all requests.
// Uses the global MeterProvider configured by the observability package.
func otelMetricsMiddleware(serviceName string) fiber.Handler {
	// Get global MeterProvider (already configured by otel.Provider)
	meter := otel.GetMeterProvider().Meter(serviceName)

	// Create instruments once (reuse for all requests to avoid overhead)
	durationHistogram, _ := meter.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("Duration of HTTP server requests in seconds"),
		metric.WithUnit("s"),
	)

	requestCounter, _ := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)

	activeRequests, _ := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)

	return func(c *fiber.Ctx) error {
		start := time.Now()
		ctx := c.UserContext()

		// Increment active requests
		activeRequests.Add(ctx, 1)
		defer activeRequests.Add(ctx, -1)

		// Process request
		err := c.Next()

		// Calculate duration in seconds
		duration := time.Since(start).Seconds()

		// Get route path (use template, not actual path to avoid cardinality explosion)
		// Example: /users/:id instead of /users/123
		route := c.Route().Path
		if route == "" {
			route = "unknown"
		}

		// Prepare attributes following OpenTelemetry Semantic Conventions
		attrs := metric.WithAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", c.Response().StatusCode()),
		)

		// Record metrics
		durationHistogram.Record(ctx, duration, attrs)
		requestCounter.Add(ctx, 1, attrs)

		return err
	}
}
