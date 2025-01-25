package httpclient

import (
	"bytes"
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
}

func NewHTTPClientRetryable(options ...Option) HTTPClient {
	transport := &retryableTransport{
		transport: &http.Transport{},
	}

	for _, option := range options {
		option(transport)
	}

	return &http.Client{
		Transport: transport,
	}
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
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
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func WithMaxRetries(retryCount int) Option {
	return func(retryableTransport *retryableTransport) {
		retryableTransport.retryCount = retryCount
	}
}

func WithRetryPolicy(retryPolicy RetryPolicy) Option {
	return func(retryableTransport *retryableTransport) {
		retryableTransport.retryPolicy = retryPolicy
	}
}

func WithBackoff(duration time.Duration) Option {
	return func(retryableTransport *retryableTransport) {
		retryableTransport.backoff = duration
	}
}
