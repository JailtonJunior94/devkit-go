package observable

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Client struct {
	baseTransport   http.RoundTripper
	timeout         time.Duration
	maxBodySize     int64
	o11y            observability.Observability
	instrumentation *instrumentation
	httpClient      *http.Client
}

func NewClient(o11y observability.Observability, opts ...ClientOption) (*Client, error) {
	if o11y == nil {
		return nil, errors.New("observable: observability provider cannot be nil")
	}

	c := &Client{
		baseTransport: httpclient.DefaultTransport(),
		timeout:       httpclient.DefaultTimeout,
		maxBodySize:   httpclient.DefaultMaxRequestBodySize,
		o11y:          o11y,
		instrumentation: newInstrumentation(o11y.Tracer(), o11y.Metrics()),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.httpClient = &http.Client{
		Transport: &observableTransport{
			base:            c.baseTransport,
			instrumentation: c.instrumentation,
		},
		Timeout: c.timeout,
	}

	return c, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

func (c *Client) Get(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *Client) Post(ctx context.Context, url string, body io.Reader, opts ...httpclient.RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *Client) Put(ctx context.Context, url string, body io.Reader, opts ...httpclient.RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *Client) Delete(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req, opts...)
}

func (c *Client) execute(req *http.Request, opts ...httpclient.RequestOption) (*http.Response, error) {
	cfg := httpclient.BuildRequestConfig(opts)
	httpclient.ApplyHeaders(req, cfg.Headers)

	if !cfg.RetryEnabled {
		return c.httpClient.Do(req)
	}

	retryTransport, err := httpclient.NewRetryTransport(c.baseTransport, cfg, c.maxBodySize)
	if err != nil {
		return nil, err
	}

	transport := &observableTransport{
		base:            retryTransport,
		instrumentation: c.instrumentation,
	}

	return (&http.Client{Transport: transport, Timeout: c.timeout}).Do(req)
}
