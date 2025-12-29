package chiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// HealthCheckFunc is a function type for health checks.
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

// executeHealthChecks runs all health checks in parallel with the given timeout.
func executeHealthChecks(
	ctx context.Context,
	checks map[string]HealthCheckFunc,
	timeout time.Duration,
	o11y observability.Observability,
) map[string]CheckResult {
	if len(checks) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make(map[string]CheckResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, checkFunc := range checks {
		wg.Add(1)

		go func(checkName string, fn HealthCheckFunc) {
			defer wg.Done()

			err := fn(ctx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[checkName] = CheckResult{
					Status: "unhealthy",
					Error:  err.Error(),
				}
				o11y.Logger().Warn(ctx, "health check failed",
					observability.String("check", checkName),
					observability.Error(err),
				)
				return
			}

			results[checkName] = CheckResult{
				Status: "healthy",
			}
		}(name, checkFunc)
	}

	wg.Wait()

	return results
}

// isHealthy returns true if all checks are healthy.
func isHealthy(checks map[string]CheckResult) bool {
	for _, result := range checks {
		if result.Status == "unhealthy" {
			return false
		}
	}

	return true
}

// healthHandler returns a handler for the /health endpoint with detailed check results.
func healthHandler(
	config Config,
	checks map[string]HealthCheckFunc,
	o11y observability.Observability,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checkResults := executeHealthChecks(r.Context(), checks, 5*time.Second, o11y)

		status := "healthy"
		statusCode := http.StatusOK

		if !isHealthy(checkResults) {
			status = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}

		health := HealthStatus{
			Status:      status,
			Service:     config.ServiceName,
			Version:     config.ServiceVersion,
			Environment: config.Environment,
			Timestamp:   time.Now(),
			Checks:      checkResults,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		_ = json.NewEncoder(w).Encode(health)
	}
}

// readyHandler returns a handler for the /ready endpoint.
func readyHandler(
	checks map[string]HealthCheckFunc,
	o11y observability.Observability,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checkResults := executeHealthChecks(r.Context(), checks, 3*time.Second, o11y)

		if !isHealthy(checkResults) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Service Unavailable"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

// liveHandler returns a handler for the /live endpoint.
func liveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}
