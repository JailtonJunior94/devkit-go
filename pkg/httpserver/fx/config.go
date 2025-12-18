package httpserverfx

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
)

// Config holds HTTP server configuration.
type Config struct {
	// Port is the server port. Default: "8080"
	Port string

	// ReadTimeout is the maximum duration for reading the entire request.
	// Default: 15 seconds
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	// Default: 15 seconds
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	// when keep-alives are enabled. Default: 60 seconds
	IdleTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	// Default: 5 seconds
	ReadHeaderTimeout time.Duration

	// MaxHeaderBytes is the maximum size of request headers.
	// Default: 1MB (1 << 20)
	MaxHeaderBytes int

	// ShutdownTimeout is the timeout for graceful shutdown.
	// Default: 30 seconds
	ShutdownTimeout time.Duration
}

// DefaultConfig returns the default HTTP server configuration.
func DefaultConfig() Config {
	return Config{
		Port:              "8080",
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ShutdownTimeout:   30 * time.Second,
	}
}

// ConfigModule provides HTTP server config from environment variables.
// Environment variables:
//   - HTTP_PORT: Server port (default: "8080")
//   - HTTP_READ_TIMEOUT: Read timeout in seconds (default: 15)
//   - HTTP_WRITE_TIMEOUT: Write timeout in seconds (default: 15)
//   - HTTP_IDLE_TIMEOUT: Idle timeout in seconds (default: 60)
//   - HTTP_READ_HEADER_TIMEOUT: Read header timeout in seconds (default: 5)
//   - HTTP_MAX_HEADER_BYTES: Max header size in bytes (default: 1048576)
//   - HTTP_SHUTDOWN_TIMEOUT: Shutdown timeout in seconds (default: 30)
var ConfigModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates HTTP server config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Port:              getEnv("HTTP_PORT", "8080"),
		ReadTimeout:       getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:      getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       getEnvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ReadHeaderTimeout: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		MaxHeaderBytes:    getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
		ShutdownTimeout:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
