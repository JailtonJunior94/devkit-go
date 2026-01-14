package kafka

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// producer implements the messaging.Publisher interface.
type producer struct {
	topic  string
	writer *kafka.Writer
	config *config
	closed atomic.Bool
}

// newProducer creates a new Kafka producer.
func newProducer(topic string, cfg *config, dialer *kafka.Dialer) (messaging.Publisher, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    cfg.producerBatchSize,
		BatchTimeout: cfg.producerBatchTimeout,
		MaxAttempts:  cfg.producerMaxAttempts,
		Async:        cfg.producerAsync,
		Compression:  kafka.Compression(cfg.producerCompression),
		RequiredAcks: kafka.RequiredAcks(cfg.producerRequiredAcks),
		WriteTimeout: 10 * time.Second,
	}

	return &producer{
		topic:  topic,
		writer: writer,
		config: cfg,
	}, nil
}

// Publish publishes a single message to Kafka.
func (p *producer) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	// If instrumentation is enabled, wrap the publish operation
	if p.config.instrumentation != nil {
		return p.config.instrumentation.InstrumentPublish(ctx, topicOrQueue, key, headers, func(ctx context.Context) error {
			return p.publishInternal(ctx, topicOrQueue, key, headers, message)
		})
	}

	// Fallback: execute directly without tracing
	return p.publishInternal(ctx, topicOrQueue, key, headers, message)
}

// publishInternal contains the core publish logic without instrumentation.
func (p *producer) publishInternal(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	kafkaMessage := kafka.Message{
		Topic: topicOrQueue,
		Key:   []byte(key),
		Value: message.Body,
		Time:  time.Now(),
	}

	for headerKey, headerValue := range headers {
		kafkaMessage.Headers = append(kafkaMessage.Headers, kafka.Header{
			Key:   headerKey,
			Value: []byte(headerValue),
		})
	}

	for _, header := range message.Headers {
		kafkaMessage.Headers = append(kafkaMessage.Headers, kafka.Header{
			Key:   header.Key,
			Value: header.Value,
		})
	}

	if err := p.writeWithRetry(ctx, kafkaMessage); err != nil {
		p.config.logger.Error(ctx, "failed to publish message",
			Field{Key: "topic", Value: topicOrQueue},
			Field{Key: "key", Value: key},
			Field{Key: "error", Value: err},
		)
		return fmt.Errorf("%w: %v", ErrPublishFailed, err)
	}

	p.config.logger.Debug(ctx, "message published successfully",
		Field{Key: "topic", Value: topicOrQueue},
		Field{Key: "key", Value: key},
	)

	return nil
}

// PublishBatch publishes multiple messages to Kafka in a batch.
func (p *producer) PublishBatch(ctx context.Context, topicOrQueue, key string, headers map[string]string, messages []*messaging.Message) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	if len(messages) == 0 {
		return nil
	}

	kafkaMessages := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		kafkaMessage := kafka.Message{
			Topic: topicOrQueue,
			Key:   []byte(key),
			Value: msg.Body,
			Time:  time.Now(),
		}

		for headerKey, headerValue := range headers {
			kafkaMessage.Headers = append(kafkaMessage.Headers, kafka.Header{
				Key:   headerKey,
				Value: []byte(headerValue),
			})
		}

		for _, header := range msg.Headers {
			kafkaMessage.Headers = append(kafkaMessage.Headers, kafka.Header{
				Key:   header.Key,
				Value: header.Value,
			})
		}

		kafkaMessages = append(kafkaMessages, kafkaMessage)
	}

	if err := p.writeBatchWithRetry(ctx, kafkaMessages); err != nil {
		p.config.logger.Error(ctx, "failed to publish batch",
			Field{Key: "topic", Value: topicOrQueue},
			Field{Key: "count", Value: len(messages)},
			Field{Key: "error", Value: err},
		)
		return fmt.Errorf("%w: %v", ErrPublishFailed, err)
	}

	p.config.logger.Debug(ctx, "batch published successfully",
		Field{Key: "topic", Value: topicOrQueue},
		Field{Key: "count", Value: len(messages)},
	)

	return nil
}

// Close closes the producer and releases resources.
func (p *producer) Close() error {
	if p.closed.Swap(true) {
		return nil
	}

	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}

	p.config.logger.Info(context.Background(), "producer closed",
		Field{Key: "topic", Value: p.topic},
	)

	return nil
}

// writeWithRetry writes a message with retry logic.
func (p *producer) writeWithRetry(ctx context.Context, message kafka.Message) error {
	var lastErr error
	backoff := p.config.retryBackoff

	for attempt := 0; attempt <= p.config.maxRetries; attempt++ {
		if attempt > 0 {
			if err := p.sleep(ctx, backoff); err != nil {
				return err
			}
			backoff = calculateBackoff(backoff, p.config.maxRetryBackoff)
		}

		if err := p.writer.WriteMessages(ctx, message); err != nil {
			lastErr = err
			p.config.logger.Warn(ctx, "write attempt failed",
				Field{Key: "attempt", Value: attempt},
				Field{Key: "error", Value: err},
			)
			continue
		}

		return nil
	}

	return fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

// writeBatchWithRetry writes multiple messages with retry logic.
func (p *producer) writeBatchWithRetry(ctx context.Context, messages []kafka.Message) error {
	var lastErr error
	backoff := p.config.retryBackoff

	for attempt := 0; attempt <= p.config.maxRetries; attempt++ {
		if attempt > 0 {
			if err := p.sleep(ctx, backoff); err != nil {
				return err
			}
			backoff = calculateBackoff(backoff, p.config.maxRetryBackoff)
		}

		if err := p.writer.WriteMessages(ctx, messages...); err != nil {
			lastErr = err
			p.config.logger.Warn(ctx, "batch write attempt failed",
				Field{Key: "attempt", Value: attempt},
				Field{Key: "count", Value: len(messages)},
				Field{Key: "error", Value: err},
			)
			continue
		}

		return nil
	}

	return fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

// sleep sleeps for the specified duration, respecting context cancellation.
func (p *producer) sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
