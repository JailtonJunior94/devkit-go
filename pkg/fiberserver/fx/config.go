package fiberserverfx

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
)

// Config holds Fiber server configuration.
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

	// ShutdownTimeout is the timeout for graceful shutdown.
	// Default: 30 seconds
	ShutdownTimeout time.Duration

	// BodyLimit is the maximum allowed size for a request body.
	// Default: 4MB
	BodyLimit int

	// ReadBufferSize is the per-connection buffer size for requests.
	// Default: 4096
	ReadBufferSize int

	// WriteBufferSize is the per-connection buffer size for responses.
	// Default: 4096
	WriteBufferSize int

	// Prefork enables prefork mode for multi-process handling.
	// WARNING: Use with care. Default: false
	Prefork bool

	// StrictRouting enables strict routing.
	// When enabled, /foo and /foo/ are treated as different routes.
	// Default: false
	StrictRouting bool

	// CaseSensitive enables case-sensitive routing.
	// When enabled, /Foo and /foo are treated as different routes.
	// Default: true
	CaseSensitive bool
}

// DefaultConfig returns the default Fiber server configuration.
func DefaultConfig() Config {
	return Config{
		Port:            "8080",
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		BodyLimit:       4 * 1024 * 1024,
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Prefork:         false,
		StrictRouting:   false,
		CaseSensitive:   true,
	}
}

// ConfigModule provides Fiber server config from environment variables.
// Environment variables:
//   - FIBER_PORT: Server port (default: "8080")
//   - FIBER_READ_TIMEOUT: Read timeout in seconds (default: 15)
//   - FIBER_WRITE_TIMEOUT: Write timeout in seconds (default: 15)
//   - FIBER_IDLE_TIMEOUT: Idle timeout in seconds (default: 60)
//   - FIBER_SHUTDOWN_TIMEOUT: Shutdown timeout in seconds (default: 30)
//   - FIBER_BODY_LIMIT: Body limit in bytes (default: 4194304)
//   - FIBER_READ_BUFFER_SIZE: Read buffer size (default: 4096)
//   - FIBER_WRITE_BUFFER_SIZE: Write buffer size (default: 4096)
//   - FIBER_PREFORK: Enable prefork mode (default: false)
//   - FIBER_STRICT_ROUTING: Enable strict routing (default: false)
//   - FIBER_CASE_SENSITIVE: Enable case-sensitive routing (default: true)
var ConfigModule = fx.Provide(ConfigFromEnv)

// ConfigFromEnv creates Fiber server config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Port:            getEnv("FIBER_PORT", "8080"),
		ReadTimeout:     getEnvDuration("FIBER_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    getEnvDuration("FIBER_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:     getEnvDuration("FIBER_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout: getEnvDuration("FIBER_SHUTDOWN_TIMEOUT", 30*time.Second),
		BodyLimit:       getEnvInt("FIBER_BODY_LIMIT", 4*1024*1024),
		ReadBufferSize:  getEnvInt("FIBER_READ_BUFFER_SIZE", 4096),
		WriteBufferSize: getEnvInt("FIBER_WRITE_BUFFER_SIZE", 4096),
		Prefork:         getEnvBool("FIBER_PREFORK", false),
		StrictRouting:   getEnvBool("FIBER_STRICT_ROUTING", false),
		CaseSensitive:   getEnvBool("FIBER_CASE_SENSITIVE", true),
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
