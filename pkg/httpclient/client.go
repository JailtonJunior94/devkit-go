package httpclient

import (
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewHTTPClient() HTTPClient {
	return &http.Client{
		Timeout: DefaultTimeout,
	}
}

func NewHTTPClientWithTimeout(timeout time.Duration) HTTPClient {
	return &http.Client{
		Timeout: timeout,
	}
}
