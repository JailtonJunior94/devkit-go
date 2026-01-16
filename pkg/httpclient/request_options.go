package httpclient

import (
	"fmt"
	"maps"
	"time"
)

const (
	// MaxRetryAttempts is the maximum allowed retry attempts.
	// Higher values risk cascading failures and memory exhaustion.
	MaxRetryAttempts = 10

	// MaxRetryBackoff is the maximum allowed initial backoff duration.
	// Exponential backoff will be capped at 30s regardless of this value.
	MaxRetryBackoff = 10 * time.Second
)

// RequestOption configures individual requests.
// Used for per-request configuration like retry, headers, etc.
type RequestOption func(*requestConfig)

// requestConfig holds per-request configuration.
type requestConfig struct {
	retryEnabled     bool
	retryMaxAttempts int
	retryBackoff     time.Duration
	retryPolicy      NewRetryPolicy
	headers          map[string]string
}

// WithRetry enables retry for this request.
//
// Parameters:
//   - maxAttempts: Maximum number of retry attempts (1-10, 0 disables retry)
//   - backoff: Initial delay between retry attempts (must be positive, max 10s)
//   - policy: Function that determines if retry should occur (required)
//
// Invalid configurations will cause Do() to return an error instead of panicking.
// This allows callers to handle configuration errors gracefully.
//
// The retry mechanism:
// - Buffers request body up to maxBodySize (configured in client)
// - Uses exponential backoff with full jitter between attempts
// - Respects context deadline/cancellation
// - Updates span attributes with retry information
//
// Use conservative values to avoid cascading failures.
//
// Example with safe idempotent GET:
//
//	resp, err := client.Get(ctx, "https://api.example.com/balance",
//	    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
//	)
//
// Example without retry for non-idempotent POST:
//
//	// Don't use WithRetry for POST - dangerous to duplicate
//	resp, err := client.Post(ctx, "https://api.example.com/transaction", body)
func WithRetry(maxAttempts int, backoff time.Duration, policy NewRetryPolicy) RequestOption {
	return func(cfg *requestConfig) {
		if maxAttempts <= 0 {
			return // Disabled
		}

		cfg.retryEnabled = true
		cfg.retryMaxAttempts = maxAttempts
		cfg.retryBackoff = backoff
		cfg.retryPolicy = policy
	}
}

// validateRetryConfig validates retry configuration and returns error if invalid.
// Called by ObservableClient.Do() before executing the request.
func validateRetryConfig(cfg *requestConfig) error {
	if cfg.retryMaxAttempts > MaxRetryAttempts {
		return fmt.Errorf("httpclient: maxAttempts %d exceeds maximum %d (risk of cascading failures)", cfg.retryMaxAttempts, MaxRetryAttempts)
	}
	if cfg.retryBackoff < 0 {
		return fmt.Errorf("httpclient: backoff cannot be negative: %v", cfg.retryBackoff)
	}
	if cfg.retryBackoff > MaxRetryBackoff {
		return fmt.Errorf("httpclient: backoff %v exceeds maximum %v (exponential backoff caps at 30s)", cfg.retryBackoff, MaxRetryBackoff)
	}
	if cfg.retryPolicy == nil {
		return fmt.Errorf("httpclient: retry policy cannot be nil")
	}
	return nil
}

// WithHeaders adds multiple headers to the request.
// Existing headers with the same key will be overwritten.
//
// Example:
//
//	headers := map[string]string{
//	    "Authorization": "Bearer token123",
//	    "Content-Type":  "application/json",
//	}
//	resp, err := client.Post(ctx, url, body,
//	    httpclient.WithHeaders(headers),
//	)
func WithHeaders(headers map[string]string) RequestOption {
	return func(cfg *requestConfig) {
		if cfg.headers == nil {
			cfg.headers = make(map[string]string)
		}
		maps.Copy(cfg.headers, headers)
	}
}

// WithHeader adds a single header to the request.
// If the header already exists, it will be overwritten.
//
// Example:
//
//	resp, err := client.Get(ctx, url,
//	    httpclient.WithHeader("Authorization", "Bearer token123"),
//	    httpclient.WithHeader("Accept", "application/json"),
//	)
func WithHeader(key, value string) RequestOption {
	return func(cfg *requestConfig) {
		if cfg.headers == nil {
			cfg.headers = make(map[string]string)
		}
		cfg.headers[key] = value
	}
}
