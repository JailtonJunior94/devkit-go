package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// RetryHook is called by retryTransport on each retry event.
// attempt=0 signals retry is configured (called once at start).
// attempt>0 signals the attempt number being retried, with the failure reason.
type RetryHook func(attempt, maxAttempts int, reason string)

type retryHookContextKey struct{}

// WithRetryHook returns a context carrying hook for use by retryTransport.
func WithRetryHook(ctx context.Context, hook RetryHook) context.Context {
	return context.WithValue(ctx, retryHookContextKey{}, hook)
}

func retryHookFromContext(ctx context.Context) RetryHook {
	h, _ := ctx.Value(retryHookContextKey{}).(RetryHook)
	return h
}

// NewRetryTransport wraps base with retry logic configured by cfg.
func NewRetryTransport(base http.RoundTripper, cfg *RequestConfig, maxBodySize int64) (http.RoundTripper, error) {
	if err := ValidateRetryConfig(cfg); err != nil {
		return nil, err
	}
	return &retryTransport{
		base:        base,
		maxAttempts: cfg.RetryMaxAttempts,
		backoff:     cfg.RetryBackoff,
		policy:      cfg.RetryPolicy,
		maxBodySize: maxBodySize,
	}, nil
}

type retryTransport struct {
	base        http.RoundTripper
	maxAttempts int
	backoff     time.Duration
	policy      RetryPolicy
	maxBodySize int64
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	hook := retryHookFromContext(ctx)

	if hook != nil {
		hook(0, t.maxAttempts, "")
	}

	bodyBytes, err := t.bufferBody(req)
	if err != nil {
		return nil, err
	}

	attempt := 1
	for {
		retryReq := req
		if bodyBytes != nil {
			retryReq = cloneRequest(req, bodyBytes)
		}

		resp, err := t.base.RoundTrip(retryReq)

		if !t.policy(err, resp) {
			return resp, err
		}

		if attempt >= t.maxAttempts {
			return resp, err
		}

		if ctx.Err() != nil {
			t.drainBody(resp)
			return nil, ctx.Err()
		}

		t.drainBody(resp)

		if hook != nil {
			hook(attempt, t.maxAttempts, t.retryReason(err, resp))
		}

		if !t.sleepWithContext(ctx, t.calculateBackoff(attempt)) {
			return nil, ctx.Err()
		}

		attempt++
	}
}

func (t *retryTransport) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	const maxBackoff = 30 * time.Second

	exponential := min(t.backoff*(1<<(attempt-1)), maxBackoff)
	if exponential <= 0 {
		return 0
	}

	return time.Duration(rand.Int63n(int64(exponential)))
}

func cloneRequest(req *http.Request, bodyBytes []byte) *http.Request {
	cloned := *req
	cloned.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	cloned.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
	return &cloned
}

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

func (t *retryTransport) bufferBody(req *http.Request) (bodyBytes []byte, err error) {
	if req.Body == nil {
		return nil, nil
	}

	defer func() {
		if closeErr := req.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close request body: %w", closeErr)
		}
	}()

	limitedReader := io.LimitReader(req.Body, t.maxBodySize+1)
	bodyBytes, err = io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	if int64(len(bodyBytes)) > t.maxBodySize {
		return nil, ErrRequestBodyTooLarge
	}

	return bodyBytes, nil
}

func (t *retryTransport) drainBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	_, _ = io.CopyN(io.Discard, resp.Body, DefaultMaxDrainSize)
	_ = resp.Body.Close()
}

func (t *retryTransport) retryReason(err error, resp *http.Response) string {
	if err != nil {
		return "network_error"
	}

	if resp != nil {
		return fmt.Sprintf("http_%d", resp.StatusCode)
	}

	return "unknown"
}
