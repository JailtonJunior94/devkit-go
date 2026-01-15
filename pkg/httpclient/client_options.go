package httpclient

import (
	"net/http"
	"time"
)

// ClientOption configures the ObservableClient.
// Used for global client configuration like timeout, transport, and body size limits.
type ClientOption func(*ObservableClient)

// WithClientTimeout sets the default timeout for all requests.
// Default: 30 seconds (DefaultTimeout).
//
// Note: Individual requests can override this using context.WithTimeout.
//
// Example:
//
//	client := httpclient.NewObservableClient(obs,
//	    httpclient.WithClientTimeout(10*time.Second),
//	)
func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(c *ObservableClient) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithMaxBodySize sets the maximum request body size for retry buffering.
// Default: 10MB (DefaultMaxRequestBodySize).
//
// Set to 0 to disable buffering (retries won't work with request bodies).
// Increase for large uploads, but be aware of memory implications.
//
// Example:
//
//	// Allow 50MB request bodies
//	client := httpclient.NewObservableClient(obs,
//	    httpclient.WithMaxBodySize(50*1024*1024),
//	)
func WithMaxBodySize(size int64) ClientOption {
	return func(c *ObservableClient) {
		if size >= 0 {
			c.maxBodySize = size
		}
	}
}

// WithBaseTransport sets a custom base transport.
// Useful for custom connection pooling, TLS config, proxies, etc.
//
// The provided transport will be wrapped with observability and retry layers.
//
// Example:
//
//	customTransport := &http.Transport{
//	    MaxIdleConns:        200,
//	    MaxIdleConnsPerHost: 20,
//	    TLSClientConfig:     tlsConfig,
//	}
//
//	client := httpclient.NewObservableClient(obs,
//	    httpclient.WithBaseTransport(customTransport),
//	)
func WithBaseTransport(transport http.RoundTripper) ClientOption {
	return func(c *ObservableClient) {
		if transport != nil {
			c.baseTransport = transport
		}
	}
}
