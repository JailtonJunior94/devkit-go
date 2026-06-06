package httpclient

import (
	"net/http"
	"time"
)

type BaseClientOption func(*BaseClient)

func WithTimeout(timeout time.Duration) BaseClientOption {
	return func(c *BaseClient) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func WithBodySize(size int64) BaseClientOption {
	return func(c *BaseClient) {
		if size >= 0 {
			c.maxBodySize = size
		}
	}
}

func WithTransport(transport http.RoundTripper) BaseClientOption {
	return func(c *BaseClient) {
		if transport != nil {
			c.baseTransport = transport
		}
	}
}
