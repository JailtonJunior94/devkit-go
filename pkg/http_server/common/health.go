package common

import (
	"context"
	"sync"
	"time"
)

// HealthCheckFunc is a function type for health checks.
// Health check functions receive a context and should return an error if the check fails.
// The context may have a timeout, so implementations should respect ctx.Done().
type HealthCheckFunc func(ctx context.Context) error

// HealthStatus represents the overall health status of the service.
type HealthStatus struct {
	Status      string                 `json:"status"`
	Service     string                 `json:"service"`
	Version     string                 `json:"version"`
	Environment string                 `json:"environment"`
	Timestamp   time.Time              `json:"timestamp"`
	Checks      map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ExecuteHealthChecks runs all health checks in parallel with concurrency limiting.
// It returns the check results and a boolean indicating if any checks failed.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - checks: Map of check name to check function
//   - timeout: Timeout for all checks to complete
//   - maxConcurrent: Maximum number of concurrent health checks (prevents goroutine bomb)
//
// Returns:
//   - map[string]CheckResult: Results of all health checks
//   - bool: true if any check failed or timed out
//
// Thread-safety: This function is safe for concurrent use. Results map is protected by mutex.
func ExecuteHealthChecks(
	ctx context.Context,
	checks map[string]HealthCheckFunc,
	timeout time.Duration,
	maxConcurrent int,
) (map[string]CheckResult, bool) {
	if len(checks) == 0 {
		return nil, false
	}

	// Create context with timeout for all checks
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, maxConcurrent)

	results := make(map[string]CheckResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	hasErrors := false

	for name, checkFunc := range checks {
		wg.Add(1)

		go func(checkName string, fn HealthCheckFunc) {
			defer wg.Done()

			// Acquire semaphore (blocks if maxConcurrent goroutines already running)
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				// Context cancelled (timeout or cancellation), mark as unhealthy
				mu.Lock()
				results[checkName] = CheckResult{
					Status: "unhealthy",
					Error:  "timeout",
				}
				hasErrors = true
				mu.Unlock()
				return
			}

			// Execute the health check
			err := fn(ctx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[checkName] = CheckResult{
					Status: "unhealthy",
					Error:  err.Error(),
				}
				hasErrors = true
				return
			}

			results[checkName] = CheckResult{
				Status: "healthy",
			}
		}(name, checkFunc)
	}

	wg.Wait()

	return results, hasErrors
}
