package httpclient

import (
	"fmt"
	"maps"
	"net/http"
	"time"
)

const (
	MaxRetryAttempts = 10
	MaxRetryBackoff  = 10 * time.Second
)

type RequestOption func(*RequestConfig)

type RequestConfig struct {
	RetryEnabled     bool
	RetryMaxAttempts int
	RetryBackoff     time.Duration
	RetryPolicy      RetryPolicy
	Headers          map[string]string
}

func BuildRequestConfig(opts []RequestOption) *RequestConfig {
	cfg := &RequestConfig{
		Headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func ApplyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func ValidateRetryConfig(cfg *RequestConfig) error {
	if cfg.RetryMaxAttempts > MaxRetryAttempts {
		return fmt.Errorf("httpclient: maxAttempts %d exceeds maximum %d (risk of cascading failures)", cfg.RetryMaxAttempts, MaxRetryAttempts)
	}
	if cfg.RetryBackoff < 0 {
		return fmt.Errorf("httpclient: backoff cannot be negative: %v", cfg.RetryBackoff)
	}
	if cfg.RetryBackoff > MaxRetryBackoff {
		return fmt.Errorf("httpclient: backoff %v exceeds maximum %v (exponential backoff caps at 30s)", cfg.RetryBackoff, MaxRetryBackoff)
	}
	if cfg.RetryPolicy == nil {
		return fmt.Errorf("httpclient: retry policy cannot be nil")
	}
	return nil
}

func WithRetry(maxAttempts int, backoff time.Duration, policy RetryPolicy) RequestOption {
	return func(cfg *RequestConfig) {
		if maxAttempts <= 0 {
			return
		}
		cfg.RetryEnabled = true
		cfg.RetryMaxAttempts = maxAttempts
		cfg.RetryBackoff = backoff
		cfg.RetryPolicy = policy
	}
}

func WithHeaders(headers map[string]string) RequestOption {
	return func(cfg *RequestConfig) {
		if cfg.Headers == nil {
			cfg.Headers = make(map[string]string)
		}
		maps.Copy(cfg.Headers, headers)
	}
}

func WithHeader(key, value string) RequestOption {
	return func(cfg *RequestConfig) {
		if cfg.Headers == nil {
			cfg.Headers = make(map[string]string)
		}
		cfg.Headers[key] = value
	}
}
