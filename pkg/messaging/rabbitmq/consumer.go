package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrConsumerClosed = errors.New("consumer is closed")
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
		errorChan chan error
		mu        sync.RWMutex
		closed    bool
		closeMu   sync.RWMutex
	}
)

func NewConsumer(options ...Option) (messaging.Consumer, error) {
	consumer := &consumer{
		handler:   make(map[string]messaging.ConsumeHandler),
		errorChan: make(chan error, 100),
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
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					return
				}

				if c.isClosed() {
					return
				}

				c.mu.RLock()
				handler, exists := c.handler[message.RoutingKey]
				c.mu.RUnlock()

				if !exists {
					log.Printf("handler not implemented for routing key: %s", message.RoutingKey)
					// Nack message without requeue when no handler exists
					if !c.autoAck {
						if err := message.Nack(false, false); err != nil {
							c.sendError(fmt.Errorf("nack failed for unhandled message: %w", err))
						}
					}
					continue
				}

				c.dispatcher(ctx, message, handler)
			}
		}
	}()

	return nil
}

func (c *consumer) Close() error {
	c.closeMu.Lock()
	c.closed = true
	c.closeMu.Unlock()

	close(c.errorChan)
	return c.channel.Close()
}

func (c *consumer) isClosed() bool {
	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	return c.closed
}

func (c *consumer) Errors() <-chan error {
	return c.errorChan
}

func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler[eventType] = handler
}

func (c *consumer) dispatcher(ctx context.Context, delivery amqp.Delivery, handler messaging.ConsumeHandler) {
	err := handler(ctx, c.extractHeader(delivery), delivery.Body)
	if err != nil {
		log.Printf("handler error: %v", err)
		c.sendError(err)

		// Handle retry logic - this will Nack the message
		if retryErr := c.handleRetry(ctx, c.channel, delivery); retryErr != nil {
			log.Printf("retry handling failed: %v", retryErr)
			c.sendError(retryErr)
		}
		// Don't ACK on error - the message was already Nacked in handleRetry
		return
	}

	// Only ACK when handler succeeds and autoAck is disabled
	if !c.autoAck {
		if err := delivery.Ack(false); err != nil {
			log.Printf("ack failed: %v", err)
			c.sendError(fmt.Errorf("ack failed: %w", err))
		}
	}
}

func (c *consumer) handleRetry(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) error {
	if c.retry(delivery) {
		// Nack with requeue=false to send to DLX for retry
		if err := delivery.Nack(false, false); err != nil {
			return fmt.Errorf("handle_retry nack: %w", err)
		}
		return nil
	}

	// Max retries exceeded - Nack and send to DLQ
	if err := delivery.Nack(false, false); err != nil {
		return fmt.Errorf("handle_retry nack before dlq: %w", err)
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
	headers := amqp.Table{}
	for k, v := range delivery.Headers {
		if k != "x-death" {
			headers[k] = v
		}
	}

	err := ch.PublishWithContext(ctx, "", fmt.Sprintf("%s.dlq", delivery.RoutingKey), false, false, amqp.Publishing{
		ContentType: delivery.ContentType,
		Headers:     headers,
		Body:        delivery.Body,
	})

	if err != nil {
		return fmt.Errorf("send_dlq: %w", err)
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

func (c *consumer) sendError(err error) {
	select {
	case c.errorChan <- err:
	default:
		// Channel full, log and drop error
		log.Printf("error channel full, dropping error: %v", err)
	}
}

func (c *consumer) ConsumeBatch(ctx context.Context) error {
	return errors.New("not implemented")
}

func (c *consumer) ConsumeWithWorkerPool(ctx context.Context, workerCount int) error {
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

	// Create worker pool
	messageChan := make(chan amqp.Delivery, workerCount*2)

	// Start workers
	for i := 0; i < workerCount; i++ {
		go c.worker(ctx, messageChan)
	}

	// Dispatch messages to workers
	go func() {
		defer close(messageChan)
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					return
				}

				if c.isClosed() {
					return
				}

				select {
				case messageChan <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

func (c *consumer) worker(ctx context.Context, messageChan <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messageChan:
			if !ok {
				return
			}

			c.mu.RLock()
			handler, exists := c.handler[message.RoutingKey]
			c.mu.RUnlock()

			if !exists {
				log.Printf("handler not implemented for routing key: %s", message.RoutingKey)
				if !c.autoAck {
					if err := message.Nack(false, false); err != nil {
						c.sendError(fmt.Errorf("nack failed for unhandled message: %w", err))
					}
				}
				continue
			}

			c.dispatcher(ctx, message, handler)
		}
	}
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
