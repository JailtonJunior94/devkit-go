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
	defer func() {
		// Cleanup state on completion (success or DLQ)
		c.retryAttempts.Delete(messageKey)
	}()

	maxRetries := c.config.dlqConfig.MaxRetries
	currentBackoff := c.config.dlqConfig.RetryBackoff

	// Iterative retry loop (avoids stack overflow)
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to process the message
		err := handler(ctx, headers, msg.Value)

		if err == nil {
			// Success - commit and return
			if c.reader != nil {
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.config.logger.Error(ctx, "failed to commit message",
						Field{Key: "error", Value: commitErr})
					return commitErr
				}
			}
			return nil
		}

		// Update state
		state.mu.Lock()
		state.attempts = attempt + 1
		state.retryHistory = append(state.retryHistory, Retry{
			Attempt:   attempt + 1,
			Timestamp: time.Now(),
			Error:     err.Error(),
			Backoff:   currentBackoff.String(),
		})
		state.mu.Unlock()

		c.config.logger.Warn(ctx, "message processing failed, will retry",
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "partition", Value: msg.Partition},
			Field{Key: "offset", Value: msg.Offset},
			Field{Key: "attempt", Value: attempt + 1},
			Field{Key: "max_attempts", Value: maxRetries},
			Field{Key: "error", Value: err},
			Field{Key: "backoff", Value: currentBackoff.String()},
		)

		// Last attempt - don't sleep, go straight to DLQ
		if attempt == maxRetries-1 {
			break
		}

		// Sleep with context awareness
		if sleepErr := c.sleepWithContext(ctx, currentBackoff); sleepErr != nil {
			return sleepErr
		}

		// Calculate next backoff with exponential increase
		currentBackoff = calculateBackoff(currentBackoff, c.config.dlqConfig.MaxRetryBackoff)
	}

	// All retries exhausted - send to DLQ
	return c.sendToDLQ(ctx, msg, fmt.Errorf("max retries exceeded"), state)
}

// sendToDLQ sends a failed message to the Dead Letter Queue.
func (c *consumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error, state *retryState) error {
	if !c.config.dlqConfig.Enabled || c.dlqStrategy == nil {
		c.config.logger.Error(ctx, "CRITICAL: max retries exceeded but DLQ is disabled - message will be lost",
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "partition", Value: msg.Partition},
			Field{Key: "offset", Value: msg.Offset},
			Field{Key: "error", Value: err},
		)
		// Don't commit - let Kafka redeliver
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

	// CRITICAL: Only commit if DLQ publish succeeds
	if dlqErr := c.dlqStrategy.HandleFailure(ctx, dlqMsg); dlqErr != nil {
		c.config.logger.Error(ctx, "CRITICAL: failed to send message to DLQ - message will be redelivered",
			Field{Key: "error", Value: dlqErr},
			Field{Key: "original_error", Value: err},
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "offset", Value: msg.Offset},
		)
		// DO NOT COMMIT - let Kafka redeliver
		return fmt.Errorf("failed to send to DLQ: %w", dlqErr)
	}

	// Only now it's safe to commit
	if c.reader != nil {
		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			c.config.logger.Error(ctx, "CRITICAL: message in DLQ but failed to commit offset",
				Field{Key: "error", Value: commitErr},
				Field{Key: "topic", Value: msg.Topic},
				Field{Key: "offset", Value: msg.Offset},
			)
			// This is bad but message is at least in DLQ
			return commitErr
		}
	}

	c.config.logger.Info(ctx, "message sent to DLQ successfully",
		Field{Key: "topic", Value: msg.Topic},
		Field{Key: "offset", Value: msg.Offset},
		Field{Key: "attempts", Value: attempts},
	)

	return nil
}

// getOrCreateRetryState gets or creates retry state for a message atomically.
// Uses LoadOrStore to prevent race condition where multiple goroutines
// could create and store different state objects for the same key.
//
// PANIC PREVENTION: Type assertion is checked to prevent runtime panics.
// If an invalid type is stored in the map (programming error), it returns
// a new state instead of panicking, and logs a critical error.
func (c *consumer) getOrCreateRetryState(key string) *retryState {
	newState := &retryState{
		attempts:     0,
		firstAttempt: time.Now(),
		retryHistory: make([]Retry, 0),
	}

	// LoadOrStore is atomic - either loads existing or stores new
	actual, _ := c.retryAttempts.LoadOrStore(key, newState)

	// CRITICAL FIX: Checked type assertion to prevent panic
	// This should NEVER fail in normal operation, but prevents crashes
	// if there's a programming error or memory corruption
	state, ok := actual.(*retryState)
	if !ok {
		// CRITICAL ERROR: Wrong type in map - this indicates a severe bug
		c.config.logger.Error(context.Background(),
			"CRITICAL: invalid type in retryAttempts map - returning new state to prevent panic",
			Field{Key: "key", Value: key},
			Field{Key: "actual_type", Value: fmt.Sprintf("%T", actual)},
			Field{Key: "expected_type", Value: "*retryState"},
		)

		// Return new state to prevent panic and allow processing to continue
		// This is fail-safe behavior - better to lose retry history than crash
		return newState
	}

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
