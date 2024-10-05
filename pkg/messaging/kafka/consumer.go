package kafka

import (
	"context"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
)

type (
	ConsumerOptions func(consumer *consumer)
	ConsumeHandler  func(ctx context.Context, params map[string]string, body []byte) error

	Consumer interface {
		Consume(ctx context.Context) error
		RegisterHandler(eventType string, handler ConsumeHandler)
	}

	consumer struct {
		retries    int
		maxRetries int
		topic      string
		groupID    string
		brokers    []string
		reader     *kafka.Reader
		backoff    backoff.BackOff
		retryChan  chan kafka.Message
		handler    map[string]ConsumeHandler
		enableDLQ  bool
		dlqTopic   string
		broker     KafkaClient
	}
)

func WithTopic(name string) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.topic = name
	}
}

func WithBrokers(brokers []string) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.brokers = brokers
	}
}

func WithGroupID(groupID string) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.groupID = groupID
	}
}

func WithMaxRetries(maxRetries int) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.maxRetries = maxRetries
	}
}

func WithRetryChan(sizeChan int) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.retryChan = make(chan kafka.Message, sizeChan)
	}
}

func WithBackoff(backoff backoff.BackOff) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.backoff = backoff
	}
}

func WithReader() ConsumerOptions {
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

func WithDQL(dlqTopic string) ConsumerOptions {
	return func(consumer *consumer) {
		consumer.enableDLQ = true
		consumer.dlqTopic = dlqTopic
		consumer.broker = NewKafkaClient(consumer.brokers[0])
	}
}

func NewConsumer(options ...ConsumerOptions) Consumer {
	consumer := &consumer{
		handler: make(map[string]ConsumeHandler),
	}
	for _, opt := range options {
		opt(consumer)
	}
	return consumer
}

func (c *consumer) RegisterHandler(eventType string, handler ConsumeHandler) {
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

func (c *consumer) dispatcher(ctx context.Context, message kafka.Message, handler ConsumeHandler) error {
	err := handler(ctx, c.extractHeader(message), message.Value)
	if err != nil {
		c.retries++
		return c.retry(ctx, message, err)
	}

	if err := c.reader.CommitMessages(ctx, message); err != nil {
		return err
	}
	return nil
}

func (c *consumer) retry(ctx context.Context, message kafka.Message, err error) error {
	c.retryChan <- message
	go func() {
		for msg := range c.retryChan {
			if c.retries >= c.maxRetries {
				c.moveToDLQ(ctx, msg, err)
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

	return c.broker.Produce(ctx, c.dlqTopic, headers, &Message{
		Key:   message.Key,
		Value: message.Value,
	})
}

func (c *consumer) extractHeader(msg kafka.Message) map[string]string {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}
	return headers
}
