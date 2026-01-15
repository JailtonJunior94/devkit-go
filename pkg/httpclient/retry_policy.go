package httpclient

import (
	"context"
	"errors"
	"net/http"
)

// NewRetryPolicy determines if a request should be retried.
// It receives the error (if any) and response (if any).
//
// Return true to retry, false to stop.
//
// Note: retryable.go also defines RetryPolicy, but we define it here
// for the new observable client implementation. Both can coexist.
type NewRetryPolicy func(err error, resp *http.Response) bool

// DefaultNewRetryPolicy retries on network errors and 5xx server errors.
// Does NOT retry on 4xx client errors (they're not transient).
// Does NOT retry on context errors (timeout, cancellation).
//
// Use this policy for most HTTP requests where:
// - Network failures should be retried
// - Server errors (5xx) should be retried
// - Client errors (4xx) should NOT be retried
// - Context cancellation/timeout should NOT be retried
//
// Example:
//
//	resp, err := client.Get(ctx, url,
//	    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
//	)
var DefaultNewRetryPolicy NewRetryPolicy = func(err error, resp *http.Response) bool {
	if err != nil {
		// Don't retry context errors (timeout, cancellation)
		// These are deliberate user actions or deadline enforcement
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		// Retry other errors (network issues, DNS failures, etc.)
		return true
	}

	if resp == nil {
		return false
	}

	return resp.StatusCode >= 500
}

// IdempotentNewRetryPolicy retries only safe/idempotent HTTP methods.
// Includes rate limiting (429) in addition to 5xx errors.
// Does NOT retry on context errors (timeout, cancellation).
//
// Use this policy for idempotent operations like:
// - GET requests (safe to retry)
// - HEAD requests (safe to retry)
// - OPTIONS requests (safe to retry)
// - PUT requests (idempotent by design)
// - DELETE requests (idempotent by design)
//
// Do NOT use for POST requests unless they are idempotent.
//
// Example:
//
//	resp, err := client.Get(ctx, url,
//	    httpclient.WithRetry(5, 2*time.Second, httpclient.IdempotentNewRetryPolicy),
//	)
var IdempotentNewRetryPolicy NewRetryPolicy = func(err error, resp *http.Response) bool {
	if err != nil {
		// Don't retry context errors (timeout, cancellation)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		// Retry other errors
		return true
	}

	if resp == nil {
		return false
	}

	if resp.StatusCode >= 500 {
		return true
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return false
}

// NoNewRetryPolicy never retries.
// Use when you explicitly want to disable retry for a request.
//
// Example:
//
//	// POST request without retry (not idempotent)
//	resp, err := client.Post(ctx, url, body,
//	    httpclient.WithRetry(0, 0, httpclient.NoNewRetryPolicy),
//	)
var NoNewRetryPolicy NewRetryPolicy = func(err error, resp *http.Response) bool {
	return false
}
