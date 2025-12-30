package consumer

import (
	"errors"
	"fmt"
	"time"
)

// Config holds the configuration for the consumer server.
// It follows the same validation and defaults pattern as pkg/http_server.
type Config struct {
	// ServiceName is the name of the service (required).
	ServiceName string

	// ServiceVersion is the version of the service (required).
	ServiceVersion string

	// Environment is the deployment environment (required).
	// Examples: "development", "staging", "production"
	Environment string

	// Topics is the list of topics to consume from (required).
	Topics []string

	// WorkerCount is the number of concurrent workers processing messages.
	// Default: 5
	WorkerCount int

	// BatchSize is the maximum number of messages to fetch per batch.
	// Default: 10
	BatchSize int

	// ProcessingTimeout is the maximum time allowed for processing a single message.
	// Default: 30s
	ProcessingTimeout time.Duration

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	// Default: 30s
	ShutdownTimeout time.Duration

	// CommitInterval is how often to commit offsets (for Kafka-like systems).
	// Default: 5s
	CommitInterval time.Duration

	// MaxRetries is the maximum number of retry attempts for failed messages.
	// Default: 3
	MaxRetries int

	// RetryBackoff is the base duration for exponential backoff between retries.
	// Default: 1s
	RetryBackoff time.Duration

	// EnableHealthChecks enables health check endpoints.
	// Default: true
	EnableHealthChecks bool

	// EnableMetrics enables metrics collection.
	// Default: true
	EnableMetrics bool

	// EnableDLQ enables dead letter queue for failed messages.
	// Default: false
	EnableDLQ bool

	// DLQTopic is the topic name for dead letter queue.
	// Required if EnableDLQ is true.
	DLQTopic string
}

// DefaultConfig returns a Config with sensible defaults for production use.
// All timeouts, limits, and worker counts are set to conservative values.
func DefaultConfig() Config {
	return Config{
		WorkerCount:        5,
		BatchSize:          10,
		ProcessingTimeout:  30 * time.Second,
		ShutdownTimeout:    30 * time.Second,
		CommitInterval:     5 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       1 * time.Second,
		EnableHealthChecks: true,
		EnableMetrics:      true,
		EnableDLQ:          false,
	}
}

// Validate checks if the configuration is valid and returns an error
// with detailed information about what is invalid.
func (c Config) Validate() error {
	var errs []error

	// Required string fields
	if c.ServiceName == "" {
		errs = append(errs, errors.New("ServiceName is required and cannot be empty"))
	}

	if c.ServiceVersion == "" {
		errs = append(errs, errors.New("ServiceVersion is required and cannot be empty"))
	}

	if c.Environment == "" {
		errs = append(errs, errors.New("Environment is required and cannot be empty"))
	}

	// Topics validation
	if len(c.Topics) == 0 {
		errs = append(errs, errors.New("Topics is required and must contain at least one topic"))
	}

	for i, topic := range c.Topics {
		if topic == "" {
			errs = append(errs, fmt.Errorf("Topics[%d] cannot be empty", i))
		}
	}

	// Positive integer validation
	if c.WorkerCount <= 0 {
		errs = append(errs, errors.New("WorkerCount must be greater than 0"))
	}

	if c.BatchSize <= 0 {
		errs = append(errs, errors.New("BatchSize must be greater than 0"))
	}

	if c.MaxRetries < 0 {
		errs = append(errs, errors.New("MaxRetries must be greater than or equal to 0"))
	}

	// Duration validation
	if c.ProcessingTimeout <= 0 {
		errs = append(errs, errors.New("ProcessingTimeout must be greater than 0"))
	}

	if c.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("ShutdownTimeout must be greater than 0"))
	}

	if c.CommitInterval <= 0 {
		errs = append(errs, errors.New("CommitInterval must be greater than 0"))
	}

	if c.RetryBackoff <= 0 {
		errs = append(errs, errors.New("RetryBackoff must be greater than 0"))
	}

	// Conditional validation
	if c.EnableDLQ && c.DLQTopic == "" {
		errs = append(errs, errors.New("DLQTopic is required when EnableDLQ is true"))
	}

	// Return all errors combined
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
