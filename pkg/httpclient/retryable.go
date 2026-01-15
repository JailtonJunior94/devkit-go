package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Option func(retryableTransport *retryableTransport)
type RetryPolicy func(err error, resp *http.Response) bool

type retryableTransport struct {
	transport          http.RoundTripper
	retryCount         int
	retryPolicy        RetryPolicy
	backoff            time.Duration
	timeout            time.Duration
	maxRequestBodySize int64
}

func NewHTTPClientRetryable(options ...Option) HTTPClient {
	transport := &retryableTransport{
		transport:          &http.Transport{},
		retryCount:         0,
		retryPolicy:        defaultRetryPolicy,
		backoff:            time.Second,
		timeout:            DefaultTimeout,
		maxRequestBodySize: DefaultMaxRequestBodySize,
	}

	for _, option := range options {
		option(transport)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   transport.timeout,
	}
}

func defaultRetryPolicy(err error, resp *http.Response) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode >= 500
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		// Limit the request body size to prevent memory exhaustion
		limitedReader := io.LimitReader(req.Body, t.maxRequestBodySize+1)
		var err error
		bodyBytes, err = io.ReadAll(limitedReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		if int64(len(bodyBytes)) > t.maxRequestBodySize {
			return nil, ErrRequestBodyTooLarge
		}
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	retries := 0
	resp, err := t.transport.RoundTrip(req)

	for t.retryPolicy(err, resp) && retries < t.retryCount {
		time.Sleep(t.backoff)
		t.drainBody(resp)

		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err = t.transport.RoundTrip(req)
		retries++
	}

	return resp, err
}

func (t *retryableTransport) drainBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	// Limit drain size to prevent memory exhaustion from malicious servers
	_, _ = io.CopyN(io.Discard, resp.Body, DefaultMaxDrainSize)
	_ = resp.Body.Close()
}

func WithMaxRetries(retryCount int) Option {
	return func(retryableTransport *retryableTransport) {
		if retryCount < 0 {
			retryCount = 0
		}
		retryableTransport.retryCount = retryCount
	}
}

func WithRetryPolicy(retryPolicy RetryPolicy) Option {
	return func(retryableTransport *retryableTransport) {
		if retryPolicy != nil {
			retryableTransport.retryPolicy = retryPolicy
		}
	}
}

func WithBackoff(duration time.Duration) Option {
	return func(retryableTransport *retryableTransport) {
		if duration > 0 {
			retryableTransport.backoff = duration
		}
	}
}

// WithTimeout sets the timeout for HTTP requests.
// Default: 30 seconds.
func WithTimeout(timeout time.Duration) Option {
	return func(retryableTransport *retryableTransport) {
		if timeout > 0 {
			retryableTransport.timeout = timeout
		}
	}
}

// WithMaxRequestBodySize sets the maximum request body size for retry buffering.
// Default: 10MB. Set to 0 to disable buffering (retries won't work with body).
func WithMaxRequestBodySize(size int64) Option {
	return func(retryableTransport *retryableTransport) {
		if size >= 0 {
			retryableTransport.maxRequestBodySize = size
		}
	}
}
