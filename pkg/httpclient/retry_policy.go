package httpclient

import (
	"context"
	"errors"
	"net/http"
)

type RetryPolicy func(err error, resp *http.Response) bool

var DefaultRetryPolicy RetryPolicy = func(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}

	if resp == nil {
		return false
	}

	return resp.StatusCode >= 500
}

var IdempotentRetryPolicy RetryPolicy = func(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}

	if resp == nil {
		return false
	}

	if resp.StatusCode >= 500 {
		return true
	}

	return resp.StatusCode == http.StatusTooManyRequests
}

var NoRetryPolicy RetryPolicy = func(err error, resp *http.Response) bool {
	return false
}
