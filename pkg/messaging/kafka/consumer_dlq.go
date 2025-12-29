package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// retryState tracks the retry state for a message.
type retryState struct {
	attempts     int
	firstAttempt time.Time
	retryHistory []Retry
	mu           sync.Mutex
}

// handleMessageWithRetry processes a message with retry and DLQ logic.
func (c *consumer) handleMessageWithRetry(ctx context.Context, msg kafka.Message, handler messaging.ConsumeHandler) error {
	headers := extractHeaders(msg)
	messageKey := fmt.Sprintf("%s-%d-%d", msg.Topic, msg.Partition, msg.Offset)

	// Get or create retry state
	state := c.getOrCreateRetryState(messageKey)

	// Try to process the message
	err := handler(ctx, headers, msg.Value)
	if err == nil {
		// Success - commit and clean up
		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			c.config.logger.Error(ctx, "failed to commit message",
				Field{Key: "error", Value: commitErr},
			)
		}
		c.retryAttempts.Delete(messageKey)
		return nil
	}

	// Message processing failed
	state.mu.Lock()
	state.attempts++
	currentAttempt := state.attempts
	state.mu.Unlock()

	c.config.logger.Warn(ctx, "message processing failed",
		Field{Key: "topic", Value: msg.Topic},
		Field{Key: "partition", Value: msg.Partition},
		Field{Key: "offset", Value: msg.Offset},
		Field{Key: "attempt", Value: currentAttempt},
		Field{Key: "error", Value: err},
	)

	// Check if we should retry or send to DLQ
	maxRetries := c.config.dlqConfig.MaxRetries
	if currentAttempt >= maxRetries {
		return c.sendToDLQ(ctx, msg, err, state)
	}

	// Add to retry history
	state.mu.Lock()
	backoff := calculateBackoff(
		c.config.dlqConfig.RetryBackoff,
		c.config.dlqConfig.MaxRetryBackoff,
	)
	state.retryHistory = append(state.retryHistory, Retry{
		Attempt:   currentAttempt,
		Timestamp: time.Now(),
		Error:     err.Error(),
		Backoff:   backoff.String(),
	})
	state.mu.Unlock()

	// Wait before retry (respecting context)
	if sleepErr := c.sleepWithContext(ctx, backoff); sleepErr != nil {
		return sleepErr
	}

	// Retry
	return c.handleMessageWithRetry(ctx, msg, handler)
}

// sendToDLQ sends a failed message to the Dead Letter Queue.
func (c *consumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error, state *retryState) error {
	if !c.config.dlqConfig.Enabled || c.dlqStrategy == nil {
		c.config.logger.Error(ctx, "max retries exceeded but DLQ is disabled",
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "partition", Value: msg.Partition},
			Field{Key: "offset", Value: msg.Offset},
			Field{Key: "error", Value: err},
		)
		return fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, err)
	}

	state.mu.Lock()
	retryHistory := make([]Retry, len(state.retryHistory))
	copy(retryHistory, state.retryHistory)
	attempts := state.attempts
	state.mu.Unlock()

	// Create DLQ message
	dlqMsg := NewDLQMessage(
		msg.Topic,
		msg.Partition,
		msg.Offset,
		string(msg.Key),
		msg.Value,
		extractHeaders(msg),
		err,
		attempts,
		c.config.dlqConfig.MaxRetries,
		c.consumerCfg.groupID,
		retryHistory,
		c.config.dlqConfig,
	)

	// Send to DLQ using strategy
	if dlqErr := c.dlqStrategy.HandleFailure(ctx, dlqMsg); dlqErr != nil {
		c.config.logger.Error(ctx, "failed to send message to DLQ",
			Field{Key: "error", Value: dlqErr},
			Field{Key: "original_error", Value: err},
		)
		return fmt.Errorf("failed to send to DLQ: %w", dlqErr)
	}

	// Commit the original message to prevent reprocessing
	if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
		c.config.logger.Error(ctx, "failed to commit message after DLQ",
			Field{Key: "error", Value: commitErr},
		)
	}

	// Clean up retry state
	c.retryAttempts.Delete(fmt.Sprintf("%s-%d-%d", msg.Topic, msg.Partition, msg.Offset))

	return nil
}

// getOrCreateRetryState gets or creates retry state for a message.
func (c *consumer) getOrCreateRetryState(key string) *retryState {
	if val, ok := c.retryAttempts.Load(key); ok {
		return val.(*retryState)
	}

	state := &retryState{
		attempts:     0,
		firstAttempt: time.Now(),
		retryHistory: make([]Retry, 0),
	}

	c.retryAttempts.Store(key, state)
	return state
}

// sleepWithContext sleeps for the specified duration while respecting context cancellation.
func (c *consumer) sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// initializeDLQ initializes the DLQ publisher and strategy.
func (c *consumer) initializeDLQ() error {
	if !c.config.dlqConfig.Enabled {
		return nil
	}

	// Determine DLQ topic
	dlqTopic := c.consumerCfg.dlqTopic
	if dlqTopic == "" {
		dlqTopic = c.config.dlqConfig.Topic
	}

	if dlqTopic == "" {
		return fmt.Errorf("DLQ enabled but no topic specified")
	}

	// Create DLQ publisher if not already set
	if c.config.dlqConfig.Publisher == nil {
		// We need a client reference to create a producer
		// For now, we'll create it when needed
		c.config.logger.Warn(context.Background(), "DLQ publisher not configured, will create on demand")
	} else {
		c.dlqPublisher = c.config.dlqConfig.Publisher
	}

	// Initialize DLQ strategy
	if c.config.dlqConfig.Strategy != nil {
		c.dlqStrategy = c.config.dlqConfig.Strategy
	} else {
		// Default to publish strategy
		if c.dlqPublisher != nil {
			c.dlqStrategy = NewPublishToDLQStrategy(c.dlqPublisher, dlqTopic, c.config.logger)
		} else {
			// Fallback to log-only if no publisher
			c.dlqStrategy = NewLogOnlyStrategy(c.config.logger)
			c.config.logger.Warn(context.Background(), "DLQ publisher not available, using log-only strategy")
		}
	}

	return nil
}
