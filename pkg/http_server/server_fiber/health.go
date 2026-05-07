package serverfiber

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/fiber/v2"
)

// HealthCheckFunc is an alias for common.HealthCheckFunc for backward compatibility.
type HealthCheckFunc = common.HealthCheckFunc

const (
	healthTimeout       = 5 * time.Second
	readinessTimeout    = 3 * time.Second
	healthMaxConcurrent = 10
)

func registerHealthChecks(
	app *fiber.App,
	config common.Config,
	o11y observability.Observability,
	checks map[string]common.HealthCheckFunc,
) {
	app.Get("/health", createHealthHandler(config, o11y, checks))
	app.Get("/ready", createReadyHandler(o11y, checks))
	app.Get("/live", createLiveHandler())
}

func createHealthHandler(
	config common.Config,
	o11y observability.Observability,
	checks map[string]common.HealthCheckFunc,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		status := common.HealthStatus{
			Status:      "healthy",
			Service:     config.ServiceName,
			Version:     config.ServiceVersion,
			Environment: config.Environment,
			Timestamp:   time.Now(),
			Checks:      make(map[string]common.CheckResult),
		}

		results, hasErrors := common.ExecuteHealthChecks(ctx, checks, healthTimeout, healthMaxConcurrent)
		if results != nil {
			status.Checks = results
		}
		if hasErrors {
			status.Status = "unhealthy"
			logFailedChecks(ctx, o11y, results, "health check failed")
		}

		statusCode := fiber.StatusOK
		if status.Status == "unhealthy" {
			statusCode = fiber.StatusServiceUnavailable
		}

		return c.Status(statusCode).JSON(status)
	}
}

func createReadyHandler(
	o11y observability.Observability,
	checks map[string]common.HealthCheckFunc,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		results, hasErrors := common.ExecuteHealthChecks(ctx, checks, readinessTimeout, healthMaxConcurrent)
		if hasErrors {
			logFailedChecks(ctx, o11y, results, "readiness check failed")
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}

		return c.SendStatus(fiber.StatusOK)
	}
}

func createLiveHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
}

func logFailedChecks(
	ctx context.Context,
	o11y observability.Observability,
	results map[string]common.CheckResult,
	message string,
) {
	for name, result := range results {
		if result.Status == "healthy" {
			continue
		}
		o11y.Logger().Warn(ctx, message,
			observability.String("check", name),
			observability.String("error", result.Error),
		)
	}
}
