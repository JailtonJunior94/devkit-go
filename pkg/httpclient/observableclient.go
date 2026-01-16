package httpclient

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// ObservableClient is a production-ready HTTP client with built-in observability.
// It is thread-safe and can be used concurrently.
//
// Features:
//   - Automatic distributed tracing (W3C Trace Context)
//   - Request/response metrics (counter, histogram)
//   - Per-request retry configuration
//   - Context-aware timeouts
//   - Request body buffering for idempotency
//
// All requests are automatically instrumented with:
//   - Span: "http.client.request" with attributes (method, url, status_code)
//   - Metrics: request count, error count, latency histogram
//   - Context propagation: W3C Trace Context headers injected automatically
//
// Example without retry:
//
//	client := httpclient.NewObservableClient(obs,
//	    httpclient.WithClientTimeout(10*time.Second),
//	)
//	resp, err := client.Get(ctx, "https://api.example.com/users")
//
// Example with retry:
//
//	resp, err := client.Get(ctx, "https://api.example.com/balance",
//	    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
//	)
type ObservableClient struct {
	baseTransport   http.RoundTripper
	timeout         time.Duration
	maxBodySize     int64
	o11y            observability.Observability
	instrumentation *instrumentation
}

// NewObservableClient creates a new observable HTTP client.
//
// The client is ready to use immediately and all requests are automatically
// instrumented with tracing and metrics.
//
// Parameters:
//   - o11y: Observability provider (required)
//   - opts: Configuration options (optional)
//
// Returns error if o11y is nil.
//
// Example:
//
//	client, err := httpclient.NewObservableClient(o11y,
//	    httpclient.WithClientTimeout(30*time.Second),
//	    httpclient.WithMaxBodySize(10*1024*1024),
//	)
//	if err != nil {
//	    return err
//	}
func NewObservableClient(o11y observability.Observability, opts ...ClientOption) (*ObservableClient, error) {
	if o11y == nil {
		return nil, errors.New("httpclient: observability provider cannot be nil")
	}

	client := &ObservableClient{
		baseTransport: &http.Transport{
			// Connection pool settings
			MaxIdleConns:        100,              // Total idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Idle connections per host
			MaxConnsPerHost:     0,                // 0 = unlimited (default)
			IdleConnTimeout:     90 * time.Second, // How long idle connection stays in pool

			// Timeout settings (critical for preventing goroutine leaks)
			ResponseHeaderTimeout: 10 * time.Second, // Timeout waiting for server's response headers
			TLSHandshakeTimeout:   10 * time.Second, // Timeout for TLS handshake
			ExpectContinueTimeout: 1 * time.Second,  // Timeout for 100-Continue response

			// Keep-alive settings
			DisableKeepAlives: false, // Enable connection reuse (default)

			// Compression
			DisableCompression: false, // Enable gzip (default)

			// HTTP/2 support
			ForceAttemptHTTP2: true, // Try HTTP/2 if server supports it
		},
		timeout:         DefaultTimeout,
		maxBodySize:     DefaultMaxRequestBodySize,
		o11y:            o11y,
		instrumentation: newInstrumentation(o11y.Tracer(), o11y.Metrics()),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Get performs an HTTP GET request with observability.
//
// The request is automatically traced and metricsed. Use WithRetry() to enable retry.
//
// Example:
//
//	resp, err := client.Get(ctx, "https://api.example.com/users/123",
//	    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
//	    httpclient.WithHeader("Authorization", "Bearer token"),
//	)
func (c *ObservableClient) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

// Post performs an HTTP POST request with observability.
//
// Example:
//
//	body := bytes.NewReader([]byte(`{"name": "John"}`))
//	resp, err := client.Post(ctx, "https://api.example.com/users", body,
//	    httpclient.WithHeader("Content-Type", "application/json"),
//	)
func (c *ObservableClient) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

// Put performs an HTTP PUT request with observability.
//
// Example:
//
//	body := bytes.NewReader([]byte(`{"name": "John Updated"}`))
//	resp, err := client.Put(ctx, "https://api.example.com/users/123", body,
//	    httpclient.WithHeader("Content-Type", "application/json"),
//	)
func (c *ObservableClient) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

// Delete performs an HTTP DELETE request with observability.
//
// Example:
//
//	resp, err := client.Delete(ctx, "https://api.example.com/users/123",
//	    httpclient.WithHeader("Authorization", "Bearer token"),
//	)
func (c *ObservableClient) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

// Do executes an HTTP request with observability and optional retry.
//
// This is the core method that all other methods delegate to.
// It builds the transport chain dynamically based on request options.
//
// The transport chain is built as:
//  1. Base transport (http.Transport or custom)
//  2. Retry transport (if WithRetry configured)
//  3. Observable transport (always active - outermost layer)
//
// Example:
//
//	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com/data", nil)
//	resp, err := client.Do(ctx, req,
//	    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
//	)
func (c *ObservableClient) Do(ctx context.Context, req *http.Request, opts ...RequestOption) (*http.Response, error) {
	cfg := c.buildRequestConfig(opts)

	// Validate retry configuration if enabled
	if cfg.retryEnabled {
		if err := validateRetryConfig(cfg); err != nil {
			return nil, err
		}
	}

	c.applyHeaders(req, cfg.headers)
	transport := c.buildTransportChain(cfg)

	httpClient := &http.Client{
		Transport: transport,
		// Timeout removed: context from request controls cancellation
		// Prevents conflict between client.Timeout and context.WithTimeout
	}

	return httpClient.Do(req)
}

// applyHeaders sets request headers from configuration.
func (c *ObservableClient) applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// buildRequestConfig parses request options.
func (c *ObservableClient) buildRequestConfig(opts []RequestOption) *requestConfig {
	cfg := &requestConfig{
		retryEnabled: false,
		headers:      make(map[string]string),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// buildTransportChain constructs the transport chain based on request config.
// Returns a composed transport: observableTransport -> [retryTransport] -> baseTransport
func (c *ObservableClient) buildTransportChain(cfg *requestConfig) http.RoundTripper {
	transport := c.baseTransport

	if cfg.retryEnabled {
		transport = &retryTransport{
			base:            transport,
			maxAttempts:     cfg.retryMaxAttempts,
			backoff:         cfg.retryBackoff,
			policy:          cfg.retryPolicy,
			maxBodySize:     c.maxBodySize,
			instrumentation: c.instrumentation,
			rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		}
	}

	transport = &observableTransport{
		base:            transport,
		instrumentation: c.instrumentation,
	}

	return transport
}
