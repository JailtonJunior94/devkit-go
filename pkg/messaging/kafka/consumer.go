package kafka

import (
	"context"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/IBM/sarama"
	"github.com/cenkalti/backoff/v4"
)

type (
	Option func(consumer *consumer)

	consumer struct {
		retries       int
		maxRetries    int
		enableDLQ     bool
		topicDLQ      string
		topic         string
		groupID       string
		ready         chan bool
		backoff       backoff.BackOff
		publisher     messaging.Publisher
		consumerGroup sarama.ConsumerGroup
		retryChan     chan *sarama.ConsumerMessage
		handlers      map[string][]messaging.ConsumeHandler
	}
)

func WithTopic(name string) Option {
	return func(consumer *consumer) {
		consumer.topic = name
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
		consumer.retryChan = make(chan *sarama.ConsumerMessage, sizeChan)
	}
}

func WithBackoff(backoff backoff.BackOff) Option {
	return func(consumer *consumer) {
		consumer.backoff = backoff
	}
}

func WithDLQ(topicDLQ string) Option {
	return func(consumer *consumer) {
		consumer.enableDLQ = true
		consumer.topicDLQ = topicDLQ
	}
}

func WithClient(client *Client) Option {
	return func(consumer *consumer) {
		consumer.publisher, _ = NewPublisher(client)
		consumer.consumerGroup, _ = sarama.NewConsumerGroupFromClient(consumer.groupID, client.client)
	}
}

func NewConsumer(options ...Option) messaging.Consumer {
	consumer := &consumer{
		handlers: make(map[string][]messaging.ConsumeHandler),
	}
	for _, opt := range options {
		opt(consumer)
	}
	return consumer
}

func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

func (c *consumer) Consume(ctx context.Context) error {
	go func() {
		for {
			err := c.consumerGroup.Consume(ctx, []string{c.topic}, c)
			if err != nil {
				log.Fatal("failed to consume message:", err)
			}

			if ctx.Err() != nil {
				return
			}
			c.ready = make(chan bool)
		}
	}()
	return nil
}

func (c *consumer) dispatcher(ctx context.Context, session sarama.ConsumerGroupSession, message *sarama.ConsumerMessage, handler messaging.ConsumeHandler) error {
	err := handler(ctx, c.extractHeader(message), message.Value)
	if err != nil {
		c.retries++
		return c.retry(ctx, session, message, err)
	}
	session.MarkMessage(message, "")
	session.Commit()
	return nil
}

func (c *consumer) retry(ctx context.Context, session sarama.ConsumerGroupSession, message *sarama.ConsumerMessage, err error) error {
	c.retryChan <- message
	go func() {
		for msg := range c.retryChan {
			if c.retries >= c.maxRetries {
				if err := c.moveToDLQ(ctx, msg, err); err != nil {
					log.Fatal("failed to move message to DLQ:", err)
				}
				break
			}

			if handlers, ok := c.handlers[c.extractHeader(msg)["event_type"]]; ok {
				for _, handler := range handlers {
					if err := c.dispatcher(ctx, session, msg, handler); err != nil {
						time.Sleep(c.backoff.NextBackOff())
						continue
					}
				}
			}
		}
	}()
	return nil
}

func (c *consumer) moveToDLQ(ctx context.Context, message *sarama.ConsumerMessage, err error) error {
	if !c.enableDLQ {
		return nil
	}

	headers := map[string]string{
		"error":      err.Error(),
		"event_type": c.extractHeader(message)["event_type"],
	}

	return c.publisher.Publish(ctx, c.topicDLQ, string(message.Key), headers, &messaging.Message{
		Body: message.Value,
	})
}

func (c *consumer) extractHeader(message *sarama.ConsumerMessage) map[string]string {
	headers := make(map[string]string)
	for _, header := range message.Headers {
		headers[string(header.Key)] = string(header.Value)
	}
	return headers
}

func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		if handlers, ok := c.handlers[c.extractHeader(message)["event_type"]]; ok {
			for _, handler := range handlers {
				if err := c.dispatcher(context.Background(), session, message, handler); err != nil {
					log.Fatal("failed to dispatch message:", err)
					continue
				}
			}
		}
	}
	return nil
}

func (c *consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}
