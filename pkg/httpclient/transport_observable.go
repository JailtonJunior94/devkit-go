package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// observableTransport wraps all HTTP requests with tracing and metrics.
// It is always active and provides observability for every request.
//
// This transport creates a span for each HTTP request with the following attributes:
// - http.method: HTTP method (GET, POST, etc.)
// - http.url: Full URL of the request
// - http.host: Host portion of the URL
// - http.scheme: URL scheme (http or https)
// - http.status_code: HTTP status code (set after response)
//
// Metrics recorded:
// - http.client.request.count: Incremented for every request
// - http.client.request.errors: Incremented when errors occur
// - http.client.request.duration: Request duration in milliseconds
type observableTransport struct {
	base            http.RoundTripper
	instrumentation *instrumentation
}

// RoundTrip implements http.RoundTripper.
// Creates a span, executes the request, and records metrics.
//
// Metrics are recorded using context.Background() to ensure they are not lost
// when the request context is canceled (e.g., timeout, manual cancellation).
// This guarantees observability data for all requests, especially timeouts.
func (t *observableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	start := time.Now()

	ctx, span := t.instrumentation.tracer.Start(
		ctx,
		"http.client.request",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(
			observability.String("http.method", req.Method),
			observability.String("http.url", req.URL.String()),
			observability.String("http.host", req.URL.Host),
			observability.String("http.scheme", req.URL.Scheme),
		),
	)
	defer span.End()

	req = req.WithContext(ctx)

	resp, err := t.base.RoundTrip(req)

	duration := float64(time.Since(start).Milliseconds())

	metricAttrs := []observability.Field{
		observability.String("http.method", req.Method),
		observability.String("http.host", req.URL.Host),
	}

	// Use context.Background() for metrics to ensure they are recorded
	// even if the request context was canceled (timeout, manual cancellation).
	// Metrics are fire-and-forget and describe what happened regardless of cancellation.
	metricsCtx := context.Background()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())

		errorAttrs := append(metricAttrs,
			observability.String("error.type", classifyError(err)),
		)
		t.instrumentation.errorCounter.Increment(metricsCtx, errorAttrs...)
		t.instrumentation.requestCounter.Increment(metricsCtx, metricAttrs...)
		t.instrumentation.latencyHistogram.Record(metricsCtx, duration, metricAttrs...)

		// Return response as-is (may be nil or partial response)
		// Some transport chains return both response and error (e.g., redirect failures)
		// Caller may need response headers even on error (e.g., Retry-After)
		return resp, err
	}

	statusCode := resp.StatusCode
	span.SetAttributes(observability.Int("http.status_code", statusCode))

	metricAttrs = append(metricAttrs,
		observability.Int("http.status_code", statusCode),
	)

	if statusCode >= 400 {
		span.SetStatus(observability.StatusCodeError,
			fmt.Sprintf("HTTP %d", statusCode))
	}

	if statusCode < 400 {
		span.SetStatus(observability.StatusCodeOK, "request successful")
	}

	t.instrumentation.requestCounter.Increment(metricsCtx, metricAttrs...)
	t.instrumentation.latencyHistogram.Record(metricsCtx, duration, metricAttrs...)

	return resp, nil
}

// classifyError categorizes errors for better observability metrics.
func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	if errors.Is(err, context.Canceled) {
		return "canceled"
	}

	if errors.Is(err, ErrRequestBodyTooLarge) {
		return "body_too_large"
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "network_timeout"
		}
		return "network_error"
	}

	return "unknown"
}
