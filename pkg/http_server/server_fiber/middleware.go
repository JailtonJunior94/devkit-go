package serverfiber

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	otelobs "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
	"github.com/gofiber/fiber/v2"
	fibermwtimeout "github.com/gofiber/fiber/v2/middleware/timeout"
)

const requestIDLocalKey = "requestID"

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

				if requestID, ok := c.Locals(requestIDLocalKey).(string); ok {
					fields = append(fields, observability.String("request_id", requestID))
				}

				o11y.Logger().Error(c.UserContext(), "recovered from panic in HTTP handler", fields...)

				requestID, _ := c.Locals(requestIDLocalKey).(string)
				problem := common.ProblemDetail{
					Type:      "https://httpstatuses.com/500",
					Title:     common.GetStatusText(fiber.StatusInternalServerError),
					Status:    fiber.StatusInternalServerError,
					Detail:    "Internal server error",
					Instance:  c.Path(),
					Timestamp: time.Now().UTC(),
					RequestID: requestID,
				}

				// Fiber buffers the full response before sending, so it is always safe to
				// overwrite status and body here even if the handler wrote a partial response
				// before panicking.
				body, err := json.Marshal(problem)
				if err != nil {
					_ = c.Status(fiber.StatusInternalServerError).SendString("internal server error")
					return
				}
				c.Status(fiber.StatusInternalServerError)
				c.Set(fiber.HeaderContentType, problemContentType)
				_ = c.Send(body)
			}
		}()

		return c.Next()
	}
}

// requestIDMiddleware validates an inbound X-Request-ID via common.ValidateRequestID.
// Invalid non-empty values are replaced with a freshly generated id and a warn log
// is emitted with metadata only — the raw value is never echoed in the log payload
// or in the response, preventing log injection and information leakage.
func requestIDMiddleware(o11y observability.Observability) fiber.Handler {
	logger := o11y.Logger()
	return func(c *fiber.Ctx) error {
		raw := c.Get(common.HeaderRequestID)
		id, ok := common.ValidateRequestID(raw)
		if !ok {
			if raw != "" {
				logger.Warn(c.UserContext(), "invalid X-Request-ID rejected",
					observability.Int("raw_length", len(raw)),
					observability.String("remote_addr", c.IP()),
					observability.String("path", c.Path()),
					observability.String("method", c.Method()),
				)
			}
			id = common.NewRequestID()
		}
		c.Locals(requestIDLocalKey, id)
		c.Set(common.HeaderRequestID, id)

		return c.Next()
	}
}

type httpHookProvider interface {
	HTTP() otelobs.HTTPInstrumentation
}

type routePatternSetter interface {
	SetRoute(string)
}

func observabilityMiddleware(o11y observability.Observability) fiber.Handler {
	hook := httpInstrumentation(o11y)
	if hook == nil {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	return func(c *fiber.Ctx) error {
		requestID, _ := c.Locals(requestIDLocalKey).(string)
		ctx, scope := hook.StartRequest(c.UserContext(), otelobs.HTTPRequest{
			Method:        c.Method(),
			Route:         matchedFiberRoutePattern(c),
			Target:        c.Path(),
			RemoteAddr:    c.IP(),
			UserAgent:     c.Get("User-Agent"),
			RequestID:     requestID,
			CorrelationID: c.Get("Correlation-ID"),
		})
		c.SetUserContext(ctx)
		if scope == nil {
			return c.Next()
		}

		defer func() {
			if setter, ok := scope.(routePatternSetter); ok {
				setter.SetRoute(fiberRoutePattern(c))
			}
			finishFiberObservedRequest(scope, c)
		}()

		err := c.Next()
		if err != nil {
			scope.OnError(err)
			c.Locals(observedStatusCodeKey{}, fiberStatusCode(c, err))
		}
		return err
	}
}

func httpInstrumentation(o11y observability.Observability) otelobs.HTTPInstrumentation {
	provider, ok := o11y.(httpHookProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.HTTP()
}

func matchedFiberRoutePattern(c *fiber.Ctx) string {
	if c == nil || c.App() == nil {
		return "unmatched"
	}

	method := c.Method()
	path := c.Path()
	cfg := c.App().Config()
	for _, registered := range c.App().GetRoutes(true) {
		if registered.Method != method {
			continue
		}
		if fiber.RoutePatternMatch(path, registered.Path, cfg) {
			return registered.Path
		}
	}

	return "unmatched"
}

// fiberRoutePattern returns the matched route's registered pattern (for example
// "/users/:id"). It returns "unmatched" when no concrete handler route was
// matched, which keeps the metric label cardinality bounded.
//
// fiber rewrites Route.Method on Use middlewares to the actual HTTP method,
// and c.Route() falls back to a synthetic Route with empty Handlers when no
// layer matched. Distinguishing a real handler match from an active Use layer
// therefore requires consulting the registered route table (filtered to skip
// Use middlewares).
func fiberRoutePattern(c *fiber.Ctx) string {
	route := c.Route()
	if route == nil || route.Path == "" || len(route.Handlers) == 0 {
		return "unmatched"
	}
	method, path := route.Method, route.Path
	for _, registered := range c.App().GetRoutes(true) {
		if registered.Method == method && registered.Path == path {
			return path
		}
	}
	return "unmatched"
}

func fiberStatusCode(c *fiber.Ctx, err error) int {
	if statusCode, ok := c.Locals(observedStatusCodeKey{}).(int); ok && statusCode > 0 {
		return statusCode
	}
	if err != nil {
		var fiberErr *fiber.Error
		if asFiber(err, &fiberErr) {
			return fiberErr.Code
		}
		return fiber.StatusInternalServerError
	}
	return c.Response().StatusCode()
}

func asFiber(err error, target **fiber.Error) bool {
	if e, ok := err.(*fiber.Error); ok {
		*target = e
		return true
	}
	return false
}

type observedStatusCodeKey struct{}

func finishFiberObservedRequest(scope otelobs.HTTPRequestScope, c *fiber.Ctx) {
	if recovered := recover(); recovered != nil {
		scope.OnError(fmt.Errorf("panic: %v", recovered))
		scope.Finish(otelobs.HTTPResponse{
			StatusCode: fiber.StatusInternalServerError,
			Bytes:      int64(len(c.Response().Body())),
		})
		panic(recovered)
	}

	scope.Finish(otelobs.HTTPResponse{
		StatusCode: fiberStatusCode(c, nil),
		Bytes:      int64(len(c.Response().Body())),
	})
}

// makeTimeoutHandler wraps next with the official fiber timeout middleware
// (fibermwtimeout.NewWithContext). When d <= 0, next is returned unwrapped
// to keep the zero-cost path explicit.
//
// The official middleware applies context.WithTimeout to c.UserContext() and
// maps a context.DeadlineExceeded returned by the handler chain to
// fiber.ErrRequestTimeout. It does NOT spawn a goroutine to run the handler,
// so the previous bug of touching *fiber.Ctx after recycling is gone.
//
// Trade-off intentionally accepted (PRD spec-version 7 / ADR-002):
// NewWithContext does NOT interrupt a handler that ignores ctx.Done(); a
// hung handler keeps the response goroutine blocked until the *fiber.App
// WriteTimeout fires. Handlers are expected to honor c.UserContext() done
// signals; the server-level WriteTimeout is the last-resort guard.
func makeTimeoutHandler(d time.Duration, next fiber.Handler) fiber.Handler {
	if d <= 0 {
		return next
	}
	return fibermwtimeout.NewWithContext(next, d)
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

// corsMiddleware enforces a fail-closed CORS policy using the shared helpers in
// pkg/http_server/common (single source of truth across adapters). origins is a
// comma-separated value or "*". An invalid configuration is a programmer error;
// we panic during setup (fail-fast) so misconfigurations cannot silently fall
// through to a permissive runtime. Server.New runs Config.Validate before this
// middleware is constructed, so reaching the panic here means an internal
// bypass of validation and must surface immediately. Mirrors chi_server's
// corsMiddleware behavior for cross-adapter parity.
func corsMiddleware(origins string) fiber.Handler {
	allowedOrigins, err := common.ParseOrigins(origins)
	if err != nil {
		panic(fmt.Sprintf("invalid CORS configuration: %v", err))
	}

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")

		if origin == "" {
			return c.Next()
		}

		if !common.IsOriginAllowed(origin, allowedOrigins) {
			return fiber.NewError(fiber.StatusForbidden, "origin not allowed")
		}

		// SECURITY: Never use wildcard (*) with credentials.
		if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
			c.Set("Access-Control-Allow-Origin", "*")
		} else {
			c.Set("Access-Control-Allow-Origin", origin)
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
