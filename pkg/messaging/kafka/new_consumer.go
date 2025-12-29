package kafka

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// ConsumerOption is a functional option for configuring consumers.
type ConsumerOption func(*consumerConfig)

// consumerConfig holds consumer-specific configuration.
type consumerConfig struct {
	groupID     string
	topics      []string
	startOffset int64
	minBytes    int
	maxBytes    int
	commitEach  bool
	dlqEnabled  bool
	dlqTopic    string
}

// consumer implements the messaging.Consumer interface.
type consumer struct {
	reader        *kafka.Reader
	config        *config
	consumerCfg   *consumerConfig
	handlers      map[string][]messaging.ConsumeHandler
	errorCh       chan error
	closed        atomic.Bool
	mu            sync.RWMutex
	dlqPublisher  messaging.Publisher
	dlqStrategy   DLQStrategy
	retryAttempts sync.Map // map[string]*retryState
}

// WithGroupID sets the consumer group ID.
func WithGroupID(groupID string) ConsumerOption {
	return func(c *consumerConfig) {
		c.groupID = groupID
	}
}

// WithTopics sets the topics to consume from.
func WithTopics(topics ...string) ConsumerOption {
	return func(c *consumerConfig) {
		c.topics = topics
	}
}

// WithStartOffset sets the starting offset (-1=newest, -2=oldest).
func WithStartOffset(offset int64) ConsumerOption {
	return func(c *consumerConfig) {
		c.startOffset = offset
	}
}

// newConsumer creates a new Kafka consumer.
func newConsumer(cfg *config, dialer *kafka.Dialer, opts ...ConsumerOption) (messaging.Consumer, error) {
	consumerCfg := &consumerConfig{
		groupID:     cfg.consumerGroupID,
		topics:      cfg.consumerTopics,
		startOffset: cfg.consumerStartOffset,
		minBytes:    cfg.consumerMinBytes,
		maxBytes:    cfg.consumerMaxBytes,
		commitEach:  true,
	}

	for _, opt := range opts {
		opt(consumerCfg)
	}

	if len(consumerCfg.topics) == 0 {
		return nil, fmt.Errorf("at least one topic is required")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.brokers,
		GroupID:        consumerCfg.groupID,
		GroupTopics:    consumerCfg.topics,
		Dialer:         dialer,
		MinBytes:       consumerCfg.minBytes,
		MaxBytes:       consumerCfg.maxBytes,
		StartOffset:    consumerCfg.startOffset,
		CommitInterval: cfg.consumerCommitInterval,
		MaxWait:        cfg.consumerMaxWait,
	})

	c := &consumer{
		reader:      reader,
		config:      cfg,
		consumerCfg: consumerCfg,
		handlers:    make(map[string][]messaging.ConsumeHandler),
		errorCh:     make(chan error, 100),
	}

	// Initialize DLQ if enabled
	if err := c.initializeDLQ(); err != nil {
		return nil, fmt.Errorf("failed to initialize DLQ: %w", err)
	}

	return c, nil
}

// Consume starts consuming messages from Kafka.
func (c *consumer) Consume(ctx context.Context) error {
	if c.closed.Load() {
		return ErrConsumerClosed
	}

	go c.consumeLoop(ctx)
	return nil
}

// ConsumeBatch consumes messages in batch mode.
func (c *consumer) ConsumeBatch(ctx context.Context) error {
	return c.Consume(ctx)
}

// ConsumeWithWorkerPool consumes messages using a worker pool.
func (c *consumer) ConsumeWithWorkerPool(ctx context.Context, workerCount int) error {
	if c.closed.Load() {
		return ErrConsumerClosed
	}

	messageCh := make(chan kafka.Message, workerCount*2)

	for range workerCount {
		go c.worker(ctx, messageCh)
	}

	go func() {
		defer close(messageCh)
		for {
			if c.closed.Load() {
				return
			}

			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				c.sendError(err)
				continue
			}

			select {
			case messageCh <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// RegisterHandler registers a handler for a specific event type.
func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

// Errors returns the error channel.
func (c *consumer) Errors() <-chan error {
	return c.errorCh
}

// Close closes the consumer.
func (c *consumer) Close() error {
	if c.closed.Swap(true) {
		return nil
	}

	close(c.errorCh)

	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("failed to close consumer: %w", err)
	}

	c.config.logger.Info(context.Background(), "consumer closed")
	return nil
}

// consumeLoop is the main consumption loop.
func (c *consumer) consumeLoop(ctx context.Context) {
	for {
		if c.closed.Load() {
			return
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.sendError(err)
			continue
		}

		c.processMessage(ctx, msg)
	}
}

// worker processes messages from the channel.
func (c *consumer) worker(ctx context.Context, messageCh <-chan kafka.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}
			c.processMessage(ctx, msg)
		}
	}
}

// processMessage processes a single message.
func (c *consumer) processMessage(ctx context.Context, msg kafka.Message) {
	headers := extractHeaders(msg)
	eventType := headers["event_type"]

	c.mu.RLock()
	handlers, ok := c.handlers[eventType]
	c.mu.RUnlock()

	if !ok {
		c.config.logger.Warn(ctx, "no handler for event type", Field{Key: "event_type", Value: eventType})
		return
	}

	for _, handler := range handlers {
		// Use retry logic with DLQ if enabled
		if c.config.dlqConfig.Enabled {
			if err := c.handleMessageWithRetry(ctx, msg, handler); err != nil {
				c.sendError(err)
			}
		} else {
			// Original behavior without DLQ
			if err := handler(ctx, headers, msg.Value); err != nil {
				c.config.logger.Error(ctx, "handler error",
					Field{Key: "event_type", Value: eventType},
					Field{Key: "error", Value: err},
				)
				c.sendError(err)
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.config.logger.Error(ctx, "failed to commit message",
					Field{Key: "error", Value: err},
				)
				c.sendError(err)
			}
		}
	}
}

// sendError sends an error to the error channel (non-blocking).
func (c *consumer) sendError(err error) {
	select {
	case c.errorCh <- err:
	default:
	}
}

// extractHeaders extracts headers from a Kafka message.
func extractHeaders(msg kafka.Message) map[string]string {
	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}
	return headers
}
