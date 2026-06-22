package observable

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type instrumentation struct {
	tracer           observability.Tracer
	requestCounter   observability.Counter
	errorCounter     observability.Counter
	latencyHistogram observability.Histogram
}

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
		latencyHistogram: metrics.HistogramWithBuckets(
			"http.client.request.duration",
			"Duration of HTTP client requests",
			"ms",
			[]float64{5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
		),
	}
}
