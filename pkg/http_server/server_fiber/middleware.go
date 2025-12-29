package serverfiber

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// recoverMiddleware recovers from panics and logs detailed error information.
// It captures the stack trace and logs it via observability for debugging.
func recoverMiddleware(o11y observability.Observability) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())

				fields := []observability.Field{
					observability.String("ip", c.IP()),
					observability.String("path", c.Path()),
					observability.String("method", c.Method()),
					observability.Int("status", c.Response().StatusCode()),
					observability.String("stack", stack),
					observability.Any("panic", r),
				}

				if requestID, ok := c.Locals("requestID").(string); ok {
					fields = append(fields, observability.String("request_id", requestID))
				}

				o11y.Logger().Error(c.UserContext(), "recovered from panic in HTTP handler", fields...)
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"code":    fiber.StatusInternalServerError,
					"message": "Internal Server Error",
				})
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

		c.SetUserContext(ctx)

		errChan := make(chan error, 1)
		go func() {
			errChan <- c.Next()
		}()

		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return fiber.NewError(fiber.StatusRequestTimeout, "request timeout exceeded")
		}
	}
}

func securityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("X-Powered-By", "")

		return c.Next()
	}
}

func corsMiddleware(origins string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", origins)
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Set("Access-Control-Max-Age", "3600")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}
