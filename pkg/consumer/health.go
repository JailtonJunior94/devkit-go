package consumer

import (
	"context"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// HealthStatus represents the overall health status of the consumer
// and its dependencies. It follows the same pattern as pkg/http_server.
type HealthStatus struct {
	Status  string                 `json:"status"`  // "healthy", "degraded", or "unhealthy"
	Checks  map[string]CheckResult `json:"checks"`  // Individual check results
	Message string                 `json:"message"` // Overall status message
}

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Status  string `json:"status"`            // "pass" or "fail"
	Message string `json:"message,omitempty"` // Optional error message
}

// HealthCheckFunc is a function that performs a health check and returns
// an error if the check fails.
type HealthCheckFunc func(ctx context.Context) error

// Health returns the current health status of the consumer and its dependencies.
// It executes all registered health checks in parallel with a timeout.
func (s *Server) Health(ctx context.Context) HealthStatus {
	if !s.config.EnableHealthChecks {
		return HealthStatus{
			Status:  "healthy",
			Message: "health checks disabled",
		}
	}

	// Execute health checks with timeout
	checks := s.executeHealthChecks(ctx, 5*time.Second)

	// Determine overall status
	status := "healthy"
	message := "all checks passed"

	for _, result := range checks {
		if result.Status == "fail" {
			status = "unhealthy"
			message = "one or more checks failed"
			break
		}
	}

	return HealthStatus{
		Status:  status,
		Checks:  checks,
		Message: message,
	}
}

// executeHealthChecks runs all registered health checks in parallel
// and returns the results. This follows the same pattern as pkg/http_server.
func (s *Server) executeHealthChecks(ctx context.Context, timeout time.Duration) map[string]CheckResult {
	healthChecks := s.getAllHealthChecks()

	// Early return if no checks registered
	if len(healthChecks) == 0 {
		return map[string]CheckResult{
			"consumer": {
				Status:  "pass",
				Message: "consumer is running",
			},
		}
	}

	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute checks in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]CheckResult)

	for name, check := range healthChecks {
		wg.Add(1)
		go func(checkName string, checkFunc HealthCheckFunc) {
			defer wg.Done()

			// Execute check
			err := checkFunc(checkCtx)

			// Store result
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[checkName] = CheckResult{
					Status:  "fail",
					Message: err.Error(),
				}
				s.observability.Logger().Warn(checkCtx, "health check failed",
					observability.String("check", checkName),
					observability.String("error", err.Error()))
			} else {
				results[checkName] = CheckResult{
					Status:  "pass",
					Message: "",
				}
			}
		}(name, check)
	}

	// Wait for all checks to complete
	wg.Wait()

	// Add consumer status check
	mu.Lock()
	defer mu.Unlock()

	if s.isRunning.Load() {
		results["consumer"] = CheckResult{
			Status:  "pass",
			Message: "consumer is running",
		}
	} else {
		results["consumer"] = CheckResult{
			Status:  "fail",
			Message: "consumer is not running",
		}
	}

	return results
}

// Readiness returns a simple boolean indicating if the consumer is ready
// to process messages. This is useful for Kubernetes readiness probes.
func (s *Server) Readiness(ctx context.Context) bool {
	status := s.Health(ctx)
	return status.Status == "healthy"
}

// Liveness returns a simple boolean indicating if the consumer is alive.
// This always returns true if the process is running.
// Useful for Kubernetes liveness probes.
func (s *Server) Liveness(ctx context.Context) bool {
	return true
}
