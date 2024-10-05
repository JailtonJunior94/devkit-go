package rabbitmq

import (
	"context"
	"fmt"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	Option         func(consumer *consumer)
	ConsumeHandler func(ctx context.Context, body []byte) error

	Consumer interface {
		Consume(ctx context.Context) error
		RegisterHandler(eventType string, handler ConsumeHandler)
	}
)

type consumer struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	handler    map[string]ConsumeHandler
	queue      string
	name       string
	prefetch   int
	autoAck    bool
	exclusive  bool
	noLocal    bool
	noWait     bool
	args       amqp.Table
}

func NewConsumer(options ...Option) (Consumer, error) {
	consumer := &consumer{
		handler: make(map[string]ConsumeHandler),
	}

	for _, opt := range options {
		opt(consumer)
	}

	id, err := vos.NewUUID()
	if err != nil {
		return nil, err
	}

	consumer.name = fmt.Sprintf("%s:%s:%s", consumer.name, consumer.queue, id.String())
	return consumer, nil
}

func (c *consumer) Consume(ctx context.Context) error {
	if err := c.channel.Qos(c.prefetch, 0, false); err != nil {
		return err
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
		return err
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

func (c *consumer) RegisterHandler(eventType string, handler ConsumeHandler) {
	c.handler[eventType] = handler
}

func (c *consumer) dispatcher(ctx context.Context, delivery amqp.Delivery, handler ConsumeHandler) {
	err := handler(ctx, delivery.Body)
	if err != nil {
		if err := c.handleRetry(c.channel, delivery); err != nil {
			log.Println(err)
		}
	}

	if err := delivery.Ack(true); err != nil {
		log.Println(err)
	}
}

func (c *consumer) handleRetry(ch *amqp.Channel, delivery amqp.Delivery) error {
	if c.retry(delivery) {
		if err := delivery.Nack(false, false); err != nil {
			return err
		}
	}
	return c.sendDLQ(ch, delivery)
}

func (c *consumer) retry(delivery amqp.Delivery) bool {
	deaths, ok := delivery.Headers["x-death"].([]interface{})
	if !ok || len(deaths) <= 0 {
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

func (c *consumer) sendDLQ(ch *amqp.Channel, delivery amqp.Delivery) error {
	delete(delivery.Headers, "x-death")
	err := ch.PublishWithContext(context.Background(), "", fmt.Sprintf("%s.dlq", delivery.RoutingKey), false, false, amqp.Publishing{
		ContentType: delivery.ContentType,
		Body:        delivery.Body,
	})

	if err != nil {
		return err
	}
	return nil
}

func WithName(name string) Option {
	return func(consumer *consumer) {
		consumer.name = name
	}
}

func WithConnection(conn *amqp.Connection) Option {
	return func(consumer *consumer) {
		consumer.connection = conn
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
