package httpclient

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// instrumentation holds OpenTelemetry instrumentation state.
// Metrics are created once and reused (singleton pattern) to prevent redefinition errors.
// Thread-safe for concurrent use.
type instrumentation struct {
	tracer observability.Tracer

	// Metrics (created once, reused across all requests)
	requestCounter   observability.Counter
	errorCounter     observability.Counter
	latencyHistogram observability.Histogram
}

// newInstrumentation creates instrumentation with pre-defined metrics.
// Metrics are created once during client initialization to prevent redefinition errors.
//
// Following OpenTelemetry Semantic Conventions for HTTP client metrics:
// - http.client.request.count: Total number of HTTP client requests
// - http.client.request.errors: Total number of HTTP client request errors
// - http.client.request.duration: Duration of HTTP client requests in milliseconds
func newInstrumentation(tracer observability.Tracer, metrics observability.Metrics) *instrumentation {
	return &instrumentation{
		tracer: tracer,

		requestCounter: metrics.Counter(
			"http.client.request.count",
			"Total number of HTTP client requests",
			"{request}",
		),

		errorCounter: metrics.Counter(
			"http.client.request.errors",
			"Total number of HTTP client request errors",
			"{error}",
		),

		latencyHistogram: metrics.Histogram(
			"http.client.request.duration",
			"Duration of HTTP client requests",
			"ms",
		),
	}
}
