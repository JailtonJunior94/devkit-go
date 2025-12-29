package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
)

// DLQStrategy defines the strategy for handling failed messages.
type DLQStrategy interface {
	// HandleFailure handles a failed message.
	HandleFailure(ctx context.Context, msg *DLQMessage) error
	// Name returns the strategy name.
	Name() string
}

// DLQMessage represents a message in the Dead Letter Queue with enriched metadata.
type DLQMessage struct {
	// Original message data
	Topic         string            `json:"topic"`
	Partition     int               `json:"partition"`
	Offset        int64             `json:"offset"`
	Key           string            `json:"key"`
	Value         []byte            `json:"value"`
	Headers       map[string]string `json:"headers"`
	OriginalEvent string            `json:"original_event"`

	// Error information
	Error          string    `json:"error"`
	ErrorType      string    `json:"error_type"`
	ErrorTimestamp time.Time `json:"error_timestamp"`

	// Retry information
	Attempts     int       `json:"attempts"`
	MaxAttempts  int       `json:"max_attempts"`
	FirstAttempt time.Time `json:"first_attempt"`
	LastAttempt  time.Time `json:"last_attempt"`
	RetryHistory []Retry   `json:"retry_history,omitempty"`

	// Context information
	ConsumerGroup string            `json:"consumer_group"`
	ServiceName   string            `json:"service_name,omitempty"`
	Environment   string            `json:"environment,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Retry represents a single retry attempt.
type Retry struct {
	Attempt   int       `json:"attempt"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error"`
	Backoff   string    `json:"backoff,omitempty"`
}

// DLQConfig holds the configuration for Dead Letter Queue.
type DLQConfig struct {
	// Enabled indicates if DLQ is enabled.
	Enabled bool

	// Topic is the DLQ topic name.
	Topic string

	// MaxRetries is the maximum number of retry attempts before sending to DLQ.
	MaxRetries int

	// RetryBackoff is the base backoff duration between retries.
	RetryBackoff time.Duration

	// MaxRetryBackoff is the maximum backoff duration.
	MaxRetryBackoff time.Duration

	// Strategy is the DLQ strategy to use.
	Strategy DLQStrategy

	// ServiceName identifies the service sending to DLQ.
	ServiceName string

	// Environment identifies the environment (dev, staging, prod).
	Environment string

	// IncludeStackTrace indicates if stack traces should be included.
	IncludeStackTrace bool

	// Publisher is the Kafka publisher for DLQ messages.
	Publisher messaging.Publisher
}

// defaultDLQConfig returns default DLQ configuration.
func defaultDLQConfig() *DLQConfig {
	return &DLQConfig{
		Enabled:           false,
		Topic:             "",
		MaxRetries:        3,
		RetryBackoff:      2 * time.Second,
		MaxRetryBackoff:   30 * time.Second,
		ServiceName:       "unknown",
		Environment:       "development",
		IncludeStackTrace: false,
	}
}

// PublishToDLQStrategy implements a strategy that publishes to a DLQ topic.
type PublishToDLQStrategy struct {
	publisher messaging.Publisher
	topic     string
	logger    Logger
}

// NewPublishToDLQStrategy creates a new publish to DLQ strategy.
func NewPublishToDLQStrategy(publisher messaging.Publisher, topic string, logger Logger) DLQStrategy {
	if logger == nil {
		logger = NewNoopLogger()
	}

	return &PublishToDLQStrategy{
		publisher: publisher,
		topic:     topic,
		logger:    logger,
	}
}

// HandleFailure publishes the failed message to the DLQ topic.
func (s *PublishToDLQStrategy) HandleFailure(ctx context.Context, msg *DLQMessage) error {
	if s.publisher == nil {
		return fmt.Errorf("DLQ publisher is not configured")
	}

	// Serialize DLQ message to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error(ctx, "failed to serialize DLQ message",
			Field{Key: "error", Value: err},
			Field{Key: "topic", Value: msg.Topic},
		)
		return fmt.Errorf("failed to serialize DLQ message: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"dlq_version":        "1.0",
		"dlq_original_topic": msg.Topic,
		"dlq_error":          msg.Error,
		"dlq_attempts":       fmt.Sprintf("%d", msg.Attempts),
		"dlq_service":        msg.ServiceName,
		"dlq_environment":    msg.Environment,
		"dlq_timestamp":      msg.ErrorTimestamp.Format(time.RFC3339),
	}

	// Add original headers with prefix
	for k, v := range msg.Headers {
		headers[fmt.Sprintf("original_%s", k)] = v
	}

	// Publish to DLQ
	dlqMessage := &messaging.Message{
		Body: payload,
	}

	if err := s.publisher.Publish(ctx, s.topic, msg.Key, headers, dlqMessage); err != nil {
		s.logger.Error(ctx, "failed to publish to DLQ",
			Field{Key: "error", Value: err},
			Field{Key: "dlq_topic", Value: s.topic},
			Field{Key: "original_topic", Value: msg.Topic},
		)
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	s.logger.Info(ctx, "message sent to DLQ",
		Field{Key: "dlq_topic", Value: s.topic},
		Field{Key: "original_topic", Value: msg.Topic},
		Field{Key: "attempts", Value: msg.Attempts},
		Field{Key: "error", Value: msg.Error},
	)

	return nil
}

// Name returns the strategy name.
func (s *PublishToDLQStrategy) Name() string {
	return "publish_to_dlq"
}

// LogOnlyStrategy implements a strategy that only logs failed messages.
type LogOnlyStrategy struct {
	logger Logger
}

// NewLogOnlyStrategy creates a new log-only strategy.
func NewLogOnlyStrategy(logger Logger) DLQStrategy {
	if logger == nil {
		logger = NewNoopLogger()
	}

	return &LogOnlyStrategy{
		logger: logger,
	}
}

// HandleFailure logs the failed message.
func (s *LogOnlyStrategy) HandleFailure(ctx context.Context, msg *DLQMessage) error {
	s.logger.Error(ctx, "message failed after max retries (log-only strategy)",
		Field{Key: "topic", Value: msg.Topic},
		Field{Key: "key", Value: msg.Key},
		Field{Key: "error", Value: msg.Error},
		Field{Key: "attempts", Value: msg.Attempts},
		Field{Key: "consumer_group", Value: msg.ConsumerGroup},
	)

	return nil
}

// Name returns the strategy name.
func (s *LogOnlyStrategy) Name() string {
	return "log_only"
}

// DiscardStrategy implements a strategy that silently discards failed messages.
type DiscardStrategy struct{}

// NewDiscardStrategy creates a new discard strategy.
func NewDiscardStrategy() DLQStrategy {
	return &DiscardStrategy{}
}

// HandleFailure discards the message (does nothing).
func (s *DiscardStrategy) HandleFailure(ctx context.Context, msg *DLQMessage) error {
	return nil
}

// Name returns the strategy name.
func (s *DiscardStrategy) Name() string {
	return "discard"
}

// NewDLQMessage creates a new DLQ message from message data and error.
func NewDLQMessage(
	topic string,
	partition int,
	offset int64,
	key string,
	value []byte,
	headers map[string]string,
	err error,
	attempts int,
	maxAttempts int,
	consumerGroup string,
	retryHistory []Retry,
	config *DLQConfig,
) *DLQMessage {
	now := time.Now()

	var firstAttempt time.Time
	if len(retryHistory) > 0 {
		firstAttempt = retryHistory[0].Timestamp
	} else {
		firstAttempt = now
	}

	errorType := "unknown"
	if err != nil {
		errorType = fmt.Sprintf("%T", err)
	}

	originalEvent := headers["event_type"]
	if originalEvent == "" {
		originalEvent = "unknown"
	}

	dlqMsg := &DLQMessage{
		Topic:          topic,
		Partition:      partition,
		Offset:         offset,
		Key:            key,
		Value:          value,
		Headers:        headers,
		OriginalEvent:  originalEvent,
		Error:          err.Error(),
		ErrorType:      errorType,
		ErrorTimestamp: now,
		Attempts:       attempts,
		MaxAttempts:    maxAttempts,
		FirstAttempt:   firstAttempt,
		LastAttempt:    now,
		RetryHistory:   retryHistory,
		ConsumerGroup:  consumerGroup,
		ServiceName:    config.ServiceName,
		Environment:    config.Environment,
		Metadata:       make(map[string]string),
	}

	return dlqMsg
}
