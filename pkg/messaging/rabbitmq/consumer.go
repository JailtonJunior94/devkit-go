package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"
	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	Option func(consumer *consumer)

	consumer struct {
		channel   *amqp.Channel
		handler   map[string]messaging.ConsumeHandler
		queue     string
		name      string
		prefetch  int
		autoAck   bool
		exclusive bool
		noLocal   bool
		noWait    bool
		args      amqp.Table
	}
)

func NewConsumer(options ...Option) (messaging.Consumer, error) {
	consumer := &consumer{
		handler: make(map[string]messaging.ConsumeHandler),
	}

	for _, opt := range options {
		opt(consumer)
	}

	id, err := vos.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("consumer: %v", err)
	}

	consumer.name = fmt.Sprintf("%s:%s:%s", consumer.name, consumer.queue, id.String())
	return consumer, nil
}

func (c *consumer) Consume(ctx context.Context) error {
	if err := c.channel.Qos(c.prefetch, 0, false); err != nil {
		return fmt.Errorf("consume: %v", err)
	}

	messages, err := c.channel.Consume(
		c.queue,
		c.name,
		c.autoAck,
		c.exclusive,
		c.noLocal,
		c.noWait,
		c.args,
	)
	if err != nil {
		return fmt.Errorf("consume: %v", err)
	}

	go func() {
		for message := range messages {
			handler, exists := c.handler[message.RoutingKey]
			if !exists {
				log.Println("handler not implement")
				continue
			}
			c.dispatcher(ctx, message, handler)
		}
	}()

	return nil
}

func (c *consumer) Close() error {
	return c.channel.Close()
}

func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.handler[eventType] = handler
}

func (c *consumer) dispatcher(ctx context.Context, delivery amqp.Delivery, handler messaging.ConsumeHandler) {
	err := handler(ctx, c.extractHeader(delivery), delivery.Body)
	if err != nil {
		if err := c.handleRetry(ctx, c.channel, delivery); err != nil {
			log.Fatalf("dispatcher: %v", err)
		}
	}

	if err := delivery.Ack(true); err != nil {
		log.Fatalf("dispatcher: %v", err)
	}
}

func (c *consumer) handleRetry(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) error {
	if c.retry(delivery) {
		if err := delivery.Nack(false, false); err != nil {
			return fmt.Errorf("handle_retry: %v", err)
		}
	}
	return c.sendDLQ(ctx, ch, delivery)
}

func (c *consumer) retry(delivery amqp.Delivery) bool {
	deaths, ok := delivery.Headers["x-death"].([]any)
	if !ok || len(deaths) == 0 {
		return true
	}

	for _, death := range deaths {
		values, ok := death.(amqp.Table)
		if !ok {
			return true
		}
		if count, ok := values["count"].(int64); !ok || count < 3 {
			return true
		}
	}
	return false
}

func (c *consumer) sendDLQ(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) error {
	delete(delivery.Headers, "x-death")
	err := ch.PublishWithContext(ctx, "", fmt.Sprintf("%s.dlq", delivery.RoutingKey), false, false, amqp.Publishing{
		ContentType: delivery.ContentType,
		Body:        delivery.Body,
	})

	if err != nil {
		return fmt.Errorf("send_dlq: %v", err)
	}
	return nil
}

func (c *consumer) extractHeader(delivery amqp.Delivery) map[string]string {
	headers := make(map[string]string)
	for key, value := range delivery.Headers {
		headers[key] = fmt.Sprintf("%v", value)
	}
	return headers
}

func (c *consumer) ConsumeBatch(ctx context.Context) error {
	return errors.New("not implemented")
}

func (c *consumer) ConsumeWithWorkerPool(ctx context.Context, workerCount int) error {
	return errors.New("not implemented")
}

func WithName(name string) Option {
	return func(consumer *consumer) {
		consumer.name = name
	}
}

func WithChannel(ch *amqp.Channel) Option {
	return func(consumer *consumer) {
		consumer.channel = ch
	}
}

func WithQueue(name string) Option {
	return func(consumer *consumer) {
		consumer.queue = name
	}
}

func WithPrefetch(qty int) Option {
	return func(consumer *consumer) {
		consumer.prefetch = qty
	}
}

func WithAutoAck(autoAck bool) Option {
	return func(consumer *consumer) {
		consumer.autoAck = autoAck
	}
}

func WithExclusive(exclusive bool) Option {
	return func(consumer *consumer) {
		consumer.exclusive = exclusive
	}
}

func WithNoLocal(noLocal bool) Option {
	return func(consumer *consumer) {
		consumer.noLocal = noLocal
	}
}

func WithNoWait(noWait bool) Option {
	return func(consumer *consumer) {
		consumer.noWait = noWait
	}
}

func WithArgs(args amqp.Table) Option {
	return func(consumer *consumer) {
		consumer.args = args
	}
}
