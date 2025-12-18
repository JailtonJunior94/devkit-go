package httpclientfx

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
)

// Config holds HTTP client configuration.
type Config struct {
	// Timeout for HTTP requests. Default: 30 seconds
	Timeout time.Duration

	// MaxRetries is the number of retry attempts. Default: 0 (no retries)
	MaxRetries int

	// BackoffTime is the delay between retries. Default: 1 second
	BackoffTime time.Duration
}

// DefaultConfig returns the default HTTP client configuration.
func DefaultConfig() Config {
	return Config{
		Timeout:     30 * time.Second,
		MaxRetries:  0,
		BackoffTime: time.Second,
	}
}

// DefaultRetryConfig returns a default configuration for retryable HTTP client.
func DefaultRetryConfig() Config {
	return Config{
		Timeout:     30 * time.Second,
		MaxRetries:  3,
		BackoffTime: time.Second,
	}
}

// ConfigModule provides HTTP client config from environment variables.
// Environment variables:
//   - HTTP_CLIENT_TIMEOUT: Timeout in seconds (default: 30)
//   - HTTP_CLIENT_MAX_RETRIES: Number of retries (default: 0)
//   - HTTP_CLIENT_BACKOFF: Backoff time in seconds (default: 1)
var ConfigModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates HTTP client config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Timeout:     getEnvDuration("HTTP_CLIENT_TIMEOUT", 30*time.Second),
		MaxRetries:  getEnvInt("HTTP_CLIENT_MAX_RETRIES", 0),
		BackoffTime: getEnvDuration("HTTP_CLIENT_BACKOFF", time.Second),
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
