package chiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// HealthCheckFunc is an alias for common.HealthCheckFunc for backward compatibility.
type HealthCheckFunc = common.HealthCheckFunc


// healthHandler returns a handler for the /health endpoint with detailed check results.
func healthHandler(
	config common.Config,
	checks map[string]common.HealthCheckFunc,
	o11y observability.Observability,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const (
			healthCheckTimeout = 5 * time.Second
			maxConcurrent      = 10
		)

		checkResults, hasErrors := common.ExecuteHealthChecks(
			r.Context(),
			checks,
			healthCheckTimeout,
			maxConcurrent,
		)

		// Log failed checks for observability
		if hasErrors {
			for name, result := range checkResults {
				if result.Status == "unhealthy" {
					o11y.Logger().Warn(r.Context(), "health check failed",
						observability.String("check", name),
						observability.String("error", result.Error),
					)
				}
			}
		}

		status := "healthy"
		statusCode := http.StatusOK

		if hasErrors {
			status = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}

		health := common.HealthStatus{
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
	checks map[string]common.HealthCheckFunc,
	o11y observability.Observability,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const (
			readinessCheckTimeout = 3 * time.Second
			maxConcurrent         = 10
		)

		_, hasErrors := common.ExecuteHealthChecks(
			r.Context(),
			checks,
			readinessCheckTimeout,
			maxConcurrent,
		)

		if hasErrors {
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
