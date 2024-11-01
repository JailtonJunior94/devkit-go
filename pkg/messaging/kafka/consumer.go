package kafka

import (
	"context"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
)

type (
	Option func(consumer *consumer)

	consumer struct {
		retries    int
		maxRetries int
		topic      string
		groupID    string
		brokers    []string
		reader     *kafka.Reader
		backoff    backoff.BackOff
		retryChan  chan kafka.Message
		handler    map[string]messaging.ConsumeHandler
		enableDLQ  bool
		dlqTopic   string
		publisher  messaging.Publisher
	}
)

func WithTopic(name string) Option {
	return func(consumer *consumer) {
		consumer.topic = name
	}
}

func WithBrokers(brokers []string) Option {
	return func(consumer *consumer) {
		consumer.brokers = brokers
	}
}

func WithGroupID(groupID string) Option {
	return func(consumer *consumer) {
		consumer.groupID = groupID
	}
}

func WithMaxRetries(maxRetries int) Option {
	return func(consumer *consumer) {
		consumer.maxRetries = maxRetries
	}
}

func WithRetryChan(sizeChan int) Option {
	return func(consumer *consumer) {
		consumer.retryChan = make(chan kafka.Message, sizeChan)
	}
}

func WithBackoff(backoff backoff.BackOff) Option {
	return func(consumer *consumer) {
		consumer.backoff = backoff
	}
}

func WithReader() Option {
	return func(consumer *consumer) {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        consumer.brokers,
			GroupID:        consumer.groupID,
			Topic:          consumer.topic,
			StartOffset:    kafka.LastOffset,
			MinBytes:       10e3,
			MaxBytes:       10e6,
			CommitInterval: 0,
		})
		consumer.reader = reader
	}
}

func WithDQL(dlqTopic string) Option {
	return func(consumer *consumer) {
		consumer.enableDLQ = true
		consumer.dlqTopic = dlqTopic
		// consumer.publisher = NewKafkaPublisher(consumer.brokers[0])
	}
}

func NewConsumer(options ...Option) messaging.Consumer {
	consumer := &consumer{
		handler: make(map[string]messaging.ConsumeHandler),
	}
	for _, opt := range options {
		opt(consumer)
	}
	return consumer
}

func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.handler[eventType] = handler
}

func (c *consumer) Consume(ctx context.Context) error {
	go func() {
		for {
			message, err := c.reader.ReadMessage(ctx)
			if err != nil {
				log.Fatal("failed to read message:", err)
				continue
			}

			handler, exists := c.handler[c.extractHeader(message)["event_type"]]
			if !exists {
				log.Fatal("handler not implement")
				continue
			}

			if err := c.dispatcher(ctx, message, handler); err != nil {
				log.Fatal("failed to dispatch message:", err)
				continue
			}
		}
	}()
	return nil
}

func (c *consumer) dispatcher(ctx context.Context, message kafka.Message, handler messaging.ConsumeHandler) error {
	err := handler(ctx, c.extractHeader(message), message.Value)
	if err != nil {
		c.retries++
		return c.retry(ctx, message, err)
	}
	return c.reader.CommitMessages(ctx, message)
}

func (c *consumer) retry(ctx context.Context, message kafka.Message, err error) error {
	c.retryChan <- message
	go func() {
		for msg := range c.retryChan {
			if c.retries >= c.maxRetries {
				if err := c.moveToDLQ(ctx, msg, err); err != nil {
					log.Fatal("failed to move to DLQ:", err)
				}
				break
			}

			handler, exists := c.handler[c.extractHeader(msg)["event_type"]]
			if !exists {
				log.Fatal("handler not implement")
				continue
			}

			if err := c.dispatcher(ctx, msg, handler); err != nil {
				time.Sleep(c.backoff.NextBackOff())
				continue
			}
			break
		}
	}()
	return nil
}

func (c *consumer) moveToDLQ(ctx context.Context, message kafka.Message, err error) error {
	if !c.enableDLQ {
		return nil
	}

	headers := map[string]string{
		"error":      err.Error(),
		"event_type": c.extractHeader(message)["event_type"],
	}

	return c.publisher.Publish(ctx, c.dlqTopic, string(message.Key), headers, &messaging.Message{
		Body: message.Value,
	})
}

func (c *consumer) extractHeader(msg kafka.Message) map[string]string {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}
	return headers
}
