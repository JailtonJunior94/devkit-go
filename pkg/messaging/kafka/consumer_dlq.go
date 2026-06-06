package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

type retryState struct {
	attempts     int
	firstAttempt time.Time
	retryHistory []Retry
	mu           sync.Mutex
}

func (c *consumer) handleMessageWithRetry(ctx context.Context, msg kafka.Message, handler messaging.ConsumeHandler) error {
	headers := extractHeaders(msg)
	messageKey := fmt.Sprintf("%s-%d-%d", msg.Topic, msg.Partition, msg.Offset)

	state := c.getOrCreateRetryState(messageKey)
	defer c.retryAttempts.Delete(messageKey)

	maxRetries := c.config.dlqConfig.MaxRetries
	currentBackoff := c.config.dlqConfig.RetryBackoff

	for attempt := range maxRetries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := handler(ctx, headers, msg.Value)

		if err == nil {
			return nil
		}

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

		if attempt == maxRetries-1 {
			break
		}

		if sleepErr := c.sleepWithContext(ctx, currentBackoff); sleepErr != nil {
			return sleepErr
		}

		currentBackoff = calculateBackoff(currentBackoff, c.config.dlqConfig.MaxRetryBackoff)
	}

	return c.sendToDLQ(ctx, msg, fmt.Errorf("max retries exceeded"), state)
}

func (c *consumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error, state *retryState) error {
	if !c.config.dlqConfig.Enabled || c.dlqStrategy == nil {
		c.config.logger.Error(ctx, "max retries exceeded but DLQ is disabled - message will be lost",
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

	if dlqErr := c.dlqStrategy.HandleFailure(ctx, dlqMsg); dlqErr != nil {
		c.config.logger.Error(ctx, "failed to send message to DLQ - message will be redelivered",
			Field{Key: "error", Value: dlqErr},
			Field{Key: "original_error", Value: err},
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "offset", Value: msg.Offset},
		)
		return fmt.Errorf("failed to send to DLQ: %w", dlqErr)
	}

	c.config.logger.Info(ctx, "message sent to DLQ successfully",
		Field{Key: "topic", Value: msg.Topic},
		Field{Key: "offset", Value: msg.Offset},
		Field{Key: "attempts", Value: attempts},
	)

	return nil
}

func (c *consumer) getOrCreateRetryState(key string) *retryState {
	newState := &retryState{
		firstAttempt: time.Now(),
		retryHistory: make([]Retry, 0),
	}

	actual, _ := c.retryAttempts.LoadOrStore(key, newState)

	state, ok := actual.(*retryState)
	if !ok {
		c.config.logger.Error(context.Background(),
			"invalid type in retryAttempts map - returning new state to prevent panic",
			Field{Key: "key", Value: key},
			Field{Key: "actual_type", Value: fmt.Sprintf("%T", actual)},
		)
		return newState
	}

	return state
}

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

func (c *consumer) initializeDLQ() error {
	if !c.config.dlqConfig.Enabled {
		return nil
	}

	dlqTopic := c.consumerCfg.dlqTopic
	if dlqTopic == "" {
		dlqTopic = c.config.dlqConfig.Topic
	}

	if dlqTopic == "" {
		return fmt.Errorf("DLQ enabled but no topic specified")
	}

	if c.config.dlqConfig.Publisher == nil {
		c.config.logger.Warn(context.Background(), "DLQ publisher not configured, will create on demand")
	} else {
		c.dlqPublisher = c.config.dlqConfig.Publisher
	}

	if c.config.dlqConfig.Strategy != nil {
		c.dlqStrategy = c.config.dlqConfig.Strategy
	} else {
		if c.dlqPublisher != nil {
			c.dlqStrategy = NewPublishToDLQStrategy(c.dlqPublisher, dlqTopic, c.config.logger)
		} else {
			c.dlqStrategy = NewLogOnlyStrategy(c.config.logger)
			c.config.logger.Warn(context.Background(), "DLQ publisher not available, using log-only strategy")
		}
	}

	return nil
}
