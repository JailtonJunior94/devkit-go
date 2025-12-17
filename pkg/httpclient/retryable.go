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
	transport   http.RoundTripper
	retryCount  int
	retryPolicy RetryPolicy
	backoff     time.Duration
	timeout     time.Duration
}

func NewHTTPClientRetryable(options ...Option) HTTPClient {
	transport := &retryableTransport{
		transport:   &http.Transport{},
		retryCount:  0,
		retryPolicy: defaultRetryPolicy,
		backoff:     time.Second,
		timeout:     DefaultTimeout,
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
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
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
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
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
// Default: 30 seconds
func WithTimeout(timeout time.Duration) Option {
	return func(retryableTransport *retryableTransport) {
		if timeout > 0 {
			retryableTransport.timeout = timeout
		}
	}
}
