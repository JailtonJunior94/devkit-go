package serverfiber

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/gofiber/fiber/v2"
)

// HealthCheckFunc is an alias for common.HealthCheckFunc for backward compatibility.
type HealthCheckFunc = common.HealthCheckFunc

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
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()

		status := common.HealthStatus{
			Status:      "healthy",
			Service:     config.ServiceName,
			Version:     config.ServiceVersion,
			Environment: config.Environment,
			Timestamp:   time.Now(),
			Checks:      make(map[string]common.CheckResult),
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
	checks map[string]common.HealthCheckFunc,
	o11y observability.Observability,
	status *common.HealthStatus,
) bool {
	if len(checks) == 0 {
		return false
	}

	// Limit concurrent goroutines to prevent goroutine bomb
	// Maximum 10 concurrent health checks at a time
	const maxConcurrent = 10
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	var mu sync.Mutex
	checkErrors := false

	for name, checkFunc := range checks {
		wg.Add(1)

		go func(checkName string, check common.HealthCheckFunc) {
			defer wg.Done()

			// Acquire semaphore (blocks if 10 goroutines already running)
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }() // Release semaphore when done
			case <-ctx.Done():
				// Context cancelled, abort
				mu.Lock()
				status.Checks[checkName] = common.CheckResult{
					Status: "unhealthy",
					Error:  "timeout",
				}
				checkErrors = true
				mu.Unlock()
				return
			}

			result := common.CheckResult{Status: "healthy"}
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
	checks map[string]common.HealthCheckFunc,
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
	checks map[string]common.HealthCheckFunc,
	o11y observability.Observability,
) bool {
	if len(checks) == 0 {
		return false
	}

	// Limit concurrent goroutines to prevent goroutine bomb
	const maxConcurrent = 10
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	var mu sync.Mutex
	checkErrors := false

	for name, checkFunc := range checks {
		wg.Add(1)
		go func(checkName string, check common.HealthCheckFunc) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				mu.Lock()
				checkErrors = true
				mu.Unlock()
				return
			}

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
