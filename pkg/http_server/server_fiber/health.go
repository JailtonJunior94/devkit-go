package serverfiber

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/fiber/v2"
)

type HealthCheckFunc func(ctx context.Context) error

type HealthStatus struct {
	Status      string                 `json:"status"`
	Service     string                 `json:"service"`
	Version     string                 `json:"version"`
	Environment string                 `json:"environment"`
	Timestamp   time.Time              `json:"timestamp"`
	Checks      map[string]CheckResult `json:"checks,omitempty"`
}

type CheckResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func registerHealthChecks(
	app *fiber.App,
	config Config,
	o11y observability.Observability,
	checks map[string]HealthCheckFunc,
) {
	app.Get("/health", createHealthHandler(config, o11y, checks))
	app.Get("/ready", createReadyHandler(o11y, checks))
	app.Get("/live", createLiveHandler())
}

func createHealthHandler(
	config Config,
	o11y observability.Observability,
	checks map[string]HealthCheckFunc,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()

		status := HealthStatus{
			Status:      "healthy",
			Service:     config.ServiceName,
			Version:     config.ServiceVersion,
			Environment: config.Environment,
			Timestamp:   time.Now(),
			Checks:      make(map[string]CheckResult),
		}

		if len(checks) > 0 {
			checkErrors := executeHealthChecks(ctx, checks, o11y, &status)
			if checkErrors {
				status.Status = "unhealthy"
			}
		}

		statusCode := fiber.StatusOK
		if status.Status == "unhealthy" {
			statusCode = fiber.StatusServiceUnavailable
		}

		return c.Status(statusCode).JSON(status)
	}
}

func executeHealthChecks(
	ctx context.Context,
	checks map[string]HealthCheckFunc,
	o11y observability.Observability,
	status *HealthStatus,
) bool {
	var wg sync.WaitGroup
	var mu sync.Mutex
	checkErrors := false

	for name, checkFunc := range checks {
		wg.Add(1)
		go func(checkName string, check HealthCheckFunc) {
			defer wg.Done()

			result := CheckResult{Status: "healthy"}
			if err := check(ctx); err != nil {
				result.Status = "unhealthy"
				result.Error = err.Error()

				mu.Lock()
				checkErrors = true
				mu.Unlock()

				o11y.Logger().Warn(ctx, "health check failed",
					observability.String("check", checkName),
					observability.Error(err),
				)
			}

			mu.Lock()
			status.Checks[checkName] = result
			mu.Unlock()
		}(name, checkFunc)
	}

	wg.Wait()
	return checkErrors
}

func createReadyHandler(
	o11y observability.Observability,
	checks map[string]HealthCheckFunc,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 3*time.Second)
		defer cancel()

		if len(checks) > 0 {
			if hasCheckErrors := executeReadinessChecks(ctx, checks, o11y); hasCheckErrors {
				return c.SendStatus(fiber.StatusServiceUnavailable)
			}
		}

		return c.SendStatus(fiber.StatusOK)
	}
}

func executeReadinessChecks(
	ctx context.Context,
	checks map[string]HealthCheckFunc,
	o11y observability.Observability,
) bool {
	var wg sync.WaitGroup
	var mu sync.Mutex
	checkErrors := false

	for name, checkFunc := range checks {
		wg.Add(1)
		go func(checkName string, check HealthCheckFunc) {
			defer wg.Done()

			if err := check(ctx); err != nil {
				mu.Lock()
				checkErrors = true
				mu.Unlock()

				o11y.Logger().Warn(ctx, "readiness check failed",
					observability.String("check", checkName),
					observability.Error(err),
				)
			}
		}(name, checkFunc)
	}

	wg.Wait()
	return checkErrors
}

func createLiveHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
}
