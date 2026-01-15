package httpclient

import (
	"errors"
	"time"
)

const (
	// DefaultTimeout is the default timeout for all HTTP requests.
	// Can be overridden per-request using context.WithTimeout.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRequestBodySize is the maximum request body size for retry buffering.
	// Bodies larger than this will fail with ErrRequestBodyTooLarge.
	// Set to 10MB to balance memory usage and retry capability.
	DefaultMaxRequestBodySize = 10 * 1024 * 1024 // 10MB

	// DefaultMaxDrainSize is the maximum amount to drain from response body
	// before closing during retry. Prevents memory exhaustion from large responses.
	DefaultMaxDrainSize = 1 * 1024 * 1024 // 1MB
)

// ErrRequestBodyTooLarge is returned when request body exceeds maxBodySize
// and cannot be buffered for retry.
var ErrRequestBodyTooLarge = errors.New("request body exceeds maximum allowed size for retry buffering")
