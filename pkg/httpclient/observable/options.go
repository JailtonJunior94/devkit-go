package observable

import (
	"net/http"
	"time"
)

type ClientOption func(*Client)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func WithBodySize(size int64) ClientOption {
	return func(c *Client) {
		if size >= 0 {
			c.maxBodySize = size
		}
	}
}

func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *Client) {
		if transport != nil {
			c.baseTransport = transport
		}
	}
}
