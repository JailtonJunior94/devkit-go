package observable

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type observableTransport struct {
	base            http.RoundTripper
	instrumentation *instrumentation
}

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

	ctx = httpclient.WithRetryHook(ctx, func(attempt, maxAttempts int, reason string) {
		if attempt == 0 {
			span.SetAttributes(
				observability.Bool("retry.enabled", true),
				observability.Int("retry.max_attempts", maxAttempts),
			)
			return
		}
		span.SetAttributes(observability.Int("retry.attempt", attempt))
		span.AddEvent("retry_attempt",
			observability.Int("attempt", attempt),
			observability.String("reason", reason),
		)
	})

	req = req.WithContext(ctx)
	resp, err := t.base.RoundTrip(req)

	duration := float64(time.Since(start).Milliseconds())
	metricAttrs := []observability.Field{
		observability.String("http.method", req.Method),
		observability.String("http.host", req.URL.Host),
	}

	metricsCtx := context.Background()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())

		errorAttrs := make([]observability.Field, len(metricAttrs), len(metricAttrs)+1)
		copy(errorAttrs, metricAttrs)
		errorAttrs = append(errorAttrs, observability.String("error.type", classifyError(err)))

		t.instrumentation.errorCounter.Increment(metricsCtx, errorAttrs...)
		t.instrumentation.requestCounter.Increment(metricsCtx, metricAttrs...)
		t.instrumentation.latencyHistogram.Record(metricsCtx, duration, metricAttrs...)

		return resp, err
	}

	statusCode := resp.StatusCode
	span.SetAttributes(observability.Int("http.status_code", statusCode))
	metricAttrs = append(metricAttrs, observability.Int("http.status_code", statusCode))

	if statusCode >= 400 {
		span.SetStatus(observability.StatusCodeError, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(observability.StatusCodeOK, "")
	}

	t.instrumentation.requestCounter.Increment(metricsCtx, metricAttrs...)
	t.instrumentation.latencyHistogram.Record(metricsCtx, duration, metricAttrs...)

	return resp, nil
}

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

	if errors.Is(err, httpclient.ErrRequestBodyTooLarge) {
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
