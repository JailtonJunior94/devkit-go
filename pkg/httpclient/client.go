package httpclient

import (
	"context"
	"io"
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewHTTPClient() HTTPClient {
	return &http.Client{Timeout: DefaultTimeout}
}

func NewHTTPClientWithTimeout(timeout time.Duration) HTTPClient {
	return &http.Client{Timeout: timeout}
}

func DefaultTransport() http.RoundTripper {
	return &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}
}

type BaseClient struct {
	baseTransport http.RoundTripper
	timeout       time.Duration
	maxBodySize   int64
	httpClient    *http.Client
}

func NewBaseClient(opts ...BaseClientOption) *BaseClient {
	c := &BaseClient{
		baseTransport: DefaultTransport(),
		timeout:       DefaultTimeout,
		maxBodySize:   DefaultMaxRequestBodySize,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.httpClient = &http.Client{
		Transport: c.baseTransport,
		Timeout:   c.timeout,
	}
	return c
}

func (c *BaseClient) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

func (c *BaseClient) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *BaseClient) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *BaseClient) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *BaseClient) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *BaseClient) execute(req *http.Request, opts ...RequestOption) (*http.Response, error) {
	cfg := BuildRequestConfig(opts)
	ApplyHeaders(req, cfg.Headers)

	if !cfg.RetryEnabled {
		return c.httpClient.Do(req)
	}

	transport, err := NewRetryTransport(c.baseTransport, cfg, c.maxBodySize)
	if err != nil {
		return nil, err
	}

	return (&http.Client{Transport: transport, Timeout: c.timeout}).Do(req)
}
