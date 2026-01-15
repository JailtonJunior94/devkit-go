package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// retryTransport wraps requests with retry logic.
// Only active when WithRetry() is configured for a specific request.
//
// This transport:
// - Buffers request body for replay (up to maxBodySize)
// - Implements exponential backoff with full jitter
// - Updates span attributes with retry information
// - Drains response bodies before retry to prevent connection leaks
//
// Span attributes added:
// - retry.enabled: true
// - retry.max_attempts: Maximum number of retry attempts
// - retry.attempt: Current attempt number (1-indexed)
//
// Span events added:
// - retry_attempt: Logged before each retry with attempt number and reason
type retryTransport struct {
	base            http.RoundTripper
	maxAttempts     int
	backoff         time.Duration
	policy          NewRetryPolicy
	maxBodySize     int64
	instrumentation *instrumentation
	rng             *rand.Rand
}

// RoundTrip implements http.RoundTripper with retry logic.
// Respects the http.RoundTripper contract by not mutating the original request.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	span := t.instrumentation.tracer.SpanFromContext(ctx)
	span.SetAttributes(
		observability.Bool("retry.enabled", true),
		observability.Int("retry.max_attempts", t.maxAttempts),
	)

	bodyBytes, err := t.bufferBody(req)
	if err != nil {
		return nil, err
	}

	attempt := 1
	for {
		span.SetAttributes(observability.Int("retry.attempt", attempt))

		// Clone request with fresh body for each attempt to avoid mutating original
		retryReq := req
		if bodyBytes != nil {
			retryReq = cloneRequest(req, bodyBytes)
		}

		resp, err := t.base.RoundTrip(retryReq)

		shouldRetry := t.policy(err, resp)

		if !shouldRetry {
			return resp, err
		}

		if attempt >= t.maxAttempts {
			t.drainBody(resp)
			return resp, err
		}

		if ctx.Err() != nil {
			t.drainBody(resp)
			return nil, ctx.Err()
		}

		t.drainBody(resp)

		span.AddEvent("retry_attempt",
			observability.Int("attempt", attempt),
			observability.String("reason", t.retryReason(err, resp)),
		)

		backoffDuration := t.calculateBackoff(attempt)
		if !t.sleepWithContext(ctx, backoffDuration) {
			return nil, ctx.Err()
		}

		attempt++
	}
}

// calculateBackoff implements exponential backoff with full jitter.
// Formula: random(0, min(maxBackoff, baseBackoff * 2^(attempt-1)))
// This prevents thundering herd by randomizing retry times.
//
// Examples with baseBackoff=1s:
//   - attempt=1: random(0, 1s)
//   - attempt=2: random(0, 2s)
//   - attempt=3: random(0, 4s)
//   - attempt=4: random(0, 8s)
func (t *retryTransport) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Exponential: backoff * 2^(attempt-1)
	exponential := t.backoff * (1 << (attempt - 1))

	// Cap at maximum backoff to prevent overflow and excessive wait
	const maxBackoff = 30 * time.Second
	if exponential > maxBackoff {
		exponential = maxBackoff
	}

	// Full jitter: random between 0 and exponential
	// Spreads retry attempts across time window to avoid thundering herd
	if exponential <= 0 {
		return 0
	}

	jitter := time.Duration(t.rng.Int63n(int64(exponential)))
	return jitter
}

// cloneRequest creates a shallow copy of the request with a fresh body.
// This ensures we don't mutate the original request, respecting http.RoundTripper contract.
func cloneRequest(req *http.Request, bodyBytes []byte) *http.Request {
	// Shallow copy of request
	cloned := *req

	// Replace body with fresh reader
	cloned.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	cloned.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	return &cloned
}

// sleepWithContext sleeps for the specified duration or until context is canceled.
// Returns true if sleep completed, false if context was canceled.
func (t *retryTransport) sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// bufferBody buffers request body for retry.
// Returns error if body exceeds maxBodySize.
// Closes the original body after reading.
func (t *retryTransport) bufferBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	defer req.Body.Close()

	limitedReader := io.LimitReader(req.Body, t.maxBodySize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	if int64(len(bodyBytes)) > t.maxBodySize {
		return nil, ErrRequestBodyTooLarge
	}

	return bodyBytes, nil
}

// drainBody drains and closes response body to prevent connection leaks.
func (t *retryTransport) drainBody(resp *http.Response) {
	if resp == nil {
		return
	}

	if resp.Body == nil {
		return
	}

	_, _ = io.CopyN(io.Discard, resp.Body, DefaultMaxDrainSize)
	_ = resp.Body.Close()
}

// retryReason returns human-readable retry reason for logging.
func (t *retryTransport) retryReason(err error, resp *http.Response) string {
	if err != nil {
		return "network_error"
	}

	if resp != nil {
		return fmt.Sprintf("http_%d", resp.StatusCode)
	}

	return "unknown"
}
