package rabbitmq

import (
	"context"
	"fmt"
	"maps"
	"runtime/debug"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

type MessageHandler func(ctx context.Context, msg Message) error

type Message struct {
	Body        []byte
	Headers     map[string]any
	RoutingKey  string
	Exchange    string
	ContentType string
	MessageID   string
	Timestamp   int64
	Delivery    amqp.Delivery
}

type Consumer struct {
	client        *Client
	observability observability.Observability
	queue         string
	prefetchCount int
	autoAck       bool
	exclusive     bool

	mu       sync.RWMutex
	handlers map[string]MessageHandler
	workers  int
	closed   bool
}

type ConsumerOption func(*Consumer)

func WithQueue(name string) ConsumerOption {
	return func(c *Consumer) {
		c.queue = name
	}
}

func WithPrefetchCount(count int) ConsumerOption {
	return func(c *Consumer) {
		c.prefetchCount = count
	}
}

func WithAutoAck(autoAck bool) ConsumerOption {
	return func(c *Consumer) {
		c.autoAck = autoAck
	}
}

func WithExclusive(exclusive bool) ConsumerOption {
	return func(c *Consumer) {
		c.exclusive = exclusive
	}
}

func WithWorkerPool(workers int) ConsumerOption {
	return func(c *Consumer) {
		c.workers = workers
	}
}

func NewConsumer(client *Client, opts ...ConsumerOption) *Consumer {
	c := &Consumer{
		client:        client,
		observability: client.observability,
		prefetchCount: client.config.DefaultPrefetchCount,
		autoAck:       false,
		exclusive:     false,
		handlers:      make(map[string]MessageHandler),
		workers:       1,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func NewConsumerChecked(client *Client, opts ...ConsumerOption) (*Consumer, error) {
	c := NewConsumer(client, opts...)

	if c.queue == "" {
		return nil, fmt.Errorf("rabbitmq: queue name is required")
	}

	if c.workers < 1 {
		c.workers = 1
	}

	if c.prefetchCount < 0 {
		return nil, fmt.Errorf("rabbitmq: prefetch count must be non-negative")
	}

	return c, nil
}

func (c *Consumer) RegisterHandler(routingKey string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handlers[routingKey] = handler

	c.observability.Logger().Info(context.Background(), "handler registered",
		observability.String("queue", c.queue),
		observability.String("routing_key", routingKey),
	)
}

func (c *Consumer) Start(ctx context.Context) error {
	c.observability.Logger().Info(ctx, "starting consumer with auto-recovery",
		observability.String("queue", c.queue),
	)

	backoffInterval := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if c.isClosed() {
			return ErrClientClosed
		}

		err := c.consume(ctx)

		if err == ctx.Err() {
			return err
		}

		if err == nil {
			return nil
		}

		c.observability.Logger().Warn(ctx, "consumer failed, retrying",
			observability.String("queue", c.queue),
			observability.Error(err),
			observability.String("backoff", backoffInterval.String()),
		)

		c.waitBeforeRetry(ctx, backoffInterval)

		backoffInterval *= 2
		if backoffInterval > maxBackoff {
			backoffInterval = maxBackoff
		}
	}
}

func (c *Consumer) Consume(ctx context.Context) error {
	return c.Start(ctx)
}

func (c *Consumer) consume(ctx context.Context) error {
	c.mu.RLock()

	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	c.mu.RUnlock()

	pool, err := c.client.connMgr.getChannelPool()
	if err != nil {
		return fmt.Errorf("failed to get channel pool: %w", err)
	}

	consumerCh, err := pool.GetConsumerChannel(c.queue)
	if err != nil {
		return fmt.Errorf("failed to get consumer channel: %w", err)
	}
	defer func() { _ = pool.ReleaseConsumerChannel(c.queue) }()

	ch, err := consumerCh.Channel()
	if err != nil {
		return fmt.Errorf("failed to get AMQP channel: %w", err)
	}

	if err := ch.Qos(c.prefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	deliveries, err := ch.Consume(c.queue, "", c.autoAck, c.exclusive, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.observability.Logger().Info(ctx, "consumer started",
		observability.String("queue", c.queue),
		observability.Int("prefetch", c.prefetchCount),
		observability.Int("workers", c.workers),
		observability.Bool("auto_ack", c.autoAck),
	)

	if c.workers > 1 {
		return c.consumeWithWorkerPool(ctx, deliveries)
	}

	return c.consumeSingleWorker(ctx, deliveries)
}

func (c *Consumer) waitBeforeRetry(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (c *Consumer) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

func (c *Consumer) consumeSingleWorker(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	for {
		select {
		case <-ctx.Done():
			c.observability.Logger().Info(ctx, "consumer stopped by context",
				observability.String("queue", c.queue),
			)
			return ctx.Err()

		case delivery, ok := <-deliveries:
			if !ok {
				c.observability.Logger().Warn(ctx, "deliveries channel closed",
					observability.String("queue", c.queue),
				)
				return fmt.Errorf("deliveries channel closed")
			}

			c.processMessage(ctx, delivery)
		}
	}
}

func (c *Consumer) consumeWithWorkerPool(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	messageChan := make(chan amqp.Delivery, c.workers*2)
	var wg sync.WaitGroup

	for i := range c.workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, messageChan)
		}(i)
	}

	distributorDone := make(chan struct{})
	go func() {
		defer close(messageChan)
		defer close(distributorDone)

		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					return
				case messageChan <- delivery:
				}
			}
		}
	}()

	<-distributorDone
	wg.Wait()

	return ctx.Err()
}

func (c *Consumer) worker(ctx context.Context, workerID int, messageChan <-chan amqp.Delivery) {
	c.observability.Logger().Debug(ctx, "worker started",
		observability.String("queue", c.queue),
		observability.Int("worker_id", workerID),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-messageChan:
			if !ok {
				return
			}
			c.processMessage(ctx, delivery)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, delivery amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.handlePanic(ctx, delivery, r)
		}
	}()

	c.processMessageLogic(ctx, delivery)
}

func (c *Consumer) handlePanic(ctx context.Context, delivery amqp.Delivery, panicValue any) {
	c.observability.Logger().Error(ctx, "PANIC in message handler",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Any("panic", panicValue),
		observability.String("stack", string(debug.Stack())),
	)

	if c.autoAck {
		return
	}

	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack after panic",
			observability.Error(err),
		)
	}
}

func (c *Consumer) processMessageLogic(ctx context.Context, delivery amqp.Delivery) {
	msg := c.buildMessage(delivery)
	retryCount := getRetryCount(delivery)
	handler := c.findHandler(delivery.RoutingKey)

	if handler == nil {
		c.handleNoHandler(ctx, delivery)
		return
	}

	if c.client.instrumentation != nil {
		_ = c.client.instrumentation.InstrumentConsume(
			ctx,
			delivery.Exchange,
			delivery.RoutingKey,
			c.queue,
			msg.Headers,
			func(ctx context.Context) error {
				return c.processMessageWithHandler(ctx, msg, delivery, handler, retryCount)
			},
		)
		return
	}

	_ = c.processMessageWithHandler(ctx, msg, delivery, handler, retryCount)
}

func (c *Consumer) processMessageWithHandler(ctx context.Context, msg Message, delivery amqp.Delivery, handler MessageHandler, retryCount int) error {
	handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var err error

	if c.client.instrumentation != nil {
		err = c.client.instrumentation.InstrumentHandler(handlerCtx, delivery.RoutingKey, func(ctx context.Context) error {
			return handler(ctx, msg)
		})
	} else {
		err = handler(handlerCtx, msg)
	}

	if err == nil {
		c.handleSuccess(ctx, delivery)
		return nil
	}

	c.handleError(ctx, delivery, err, retryCount)
	return err
}

func (c *Consumer) buildMessage(delivery amqp.Delivery) Message {
	headers := make(map[string]any, len(delivery.Headers))
	maps.Copy(headers, delivery.Headers)

	return Message{
		Body:        delivery.Body,
		Headers:     headers,
		RoutingKey:  delivery.RoutingKey,
		Exchange:    delivery.Exchange,
		ContentType: delivery.ContentType,
		MessageID:   delivery.MessageId,
		Timestamp:   delivery.Timestamp.Unix(),
		Delivery:    delivery,
	}
}

func (c *Consumer) findHandler(routingKey string) MessageHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if handler, ok := c.handlers[routingKey]; ok {
		return handler
	}

	if handler, ok := c.handlers["*"]; ok {
		return handler
	}

	return nil
}

func (c *Consumer) handleNoHandler(ctx context.Context, delivery amqp.Delivery) {
	c.observability.Logger().Warn(ctx, "no handler for routing key",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
	)

	if c.autoAck {
		return
	}

	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack unhandled message",
			observability.Error(err),
		)
	}
}

func (c *Consumer) handleSuccess(ctx context.Context, delivery amqp.Delivery) {
	if c.autoAck {
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.observability.Logger().Error(ctx, "failed to ack message",
			observability.Error(err),
		)
	}
}

func (c *Consumer) handleError(ctx context.Context, delivery amqp.Delivery, err error, retryCount int) {
	c.observability.Logger().Error(ctx, "handler error",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
		observability.Error(err),
	)

	if c.autoAck {
		return
	}

	maxRetries := c.getMaxRetries()

	if retryCount >= maxRetries {
		c.sendToDLQ(ctx, delivery, retryCount)
		return
	}

	c.requeueMessage(ctx, delivery, retryCount)
}

func (c *Consumer) sendToDLQ(ctx context.Context, delivery amqp.Delivery, retryCount int) {
	c.observability.Logger().Warn(ctx, "max retries exceeded, sending to DLQ",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
		observability.Int("max_retries", c.getMaxRetries()),
	)

	if c.client.instrumentation != nil {
		dlqQueue := c.queue + ".dlq"
		c.client.instrumentation.RecordDLQPublish(ctx, c.queue, dlqQueue)
	}

	if err := delivery.Nack(false, false); err != nil {
		c.observability.Logger().Error(ctx, "failed to nack to DLQ",
			observability.Error(err),
		)
	}
}

func (c *Consumer) requeueMessage(ctx context.Context, delivery amqp.Delivery, retryCount int) {
	backoff := c.calculateRetryBackoff(retryCount)

	c.observability.Logger().Debug(ctx, "requeuing message for retry",
		observability.String("queue", c.queue),
		observability.String("routing_key", delivery.RoutingKey),
		observability.Int("retry_count", retryCount),
		observability.String("backoff", backoff.String()),
	)

	if c.client.instrumentation != nil {
		c.client.instrumentation.RecordRetryAttempt(ctx, c.queue, retryCount)
	}

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if err := delivery.Nack(false, false); err != nil {
			c.observability.Logger().Error(ctx, "failed to nack on context cancellation",
				observability.Error(err),
			)
		}
		return
	case <-timer.C:
	}

	headers := make(amqp.Table, len(delivery.Headers)+1)
	maps.Copy(headers, delivery.Headers)
	headers["x-retry-count"] = int32(retryCount + 1)

	republish := amqp.Publishing{
		Headers:       headers,
		ContentType:   delivery.ContentType,
		Body:          delivery.Body,
		DeliveryMode:  delivery.DeliveryMode,
		Priority:      delivery.Priority,
		CorrelationId: delivery.CorrelationId,
		ReplyTo:       delivery.ReplyTo,
		MessageId:     delivery.MessageId,
		Timestamp:     delivery.Timestamp,
		Type:          delivery.Type,
		AppId:         delivery.AppId,
	}

	pool, err := c.client.connMgr.getChannelPool()
	if err != nil {
		c.observability.Logger().Error(ctx, "failed to get channel pool for retry",
			observability.Error(err),
		)
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			c.observability.Logger().Error(ctx, "failed to nack after pool error", observability.Error(nackErr))
		}
		return
	}

	pubCh, err := pool.GetPublisherChannel()
	if err != nil {
		c.observability.Logger().Error(ctx, "failed to get publisher channel for retry",
			observability.Error(err),
		)
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			c.observability.Logger().Error(ctx, "failed to nack after channel error", observability.Error(nackErr))
		}
		return
	}

	republishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var publishErr error
	if c.client.config.EnablePublisherConfirms {
		publishErr = pubCh.PublishWithConfirm(republishCtx, delivery.Exchange, delivery.RoutingKey, republish)
	} else {
		publishErr = pubCh.PublishWithoutConfirm(republishCtx, delivery.Exchange, delivery.RoutingKey, republish)
	}

	if publishErr != nil {
		c.observability.Logger().Error(ctx, "failed to republish for retry",
			observability.Error(publishErr),
		)
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			c.observability.Logger().Error(ctx, "failed to nack after republish error", observability.Error(nackErr))
		}
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.observability.Logger().Error(ctx, "failed to ack after successful republish",
			observability.Error(err),
		)
	}
}

func (c *Consumer) calculateRetryBackoff(retryCount int) time.Duration {
	const (
		_baseBackoff = 100 * time.Millisecond
		_maxBackoff  = 30 * time.Second
	)

	backoff := _baseBackoff * (1 << uint(retryCount))

	if backoff > _maxBackoff {
		return _maxBackoff
	}

	return backoff
}

func (c *Consumer) getMaxRetries() int {
	return c.client.config.MaxRetries
}

func getRetryCount(delivery amqp.Delivery) int {
	v, ok := delivery.Headers["x-retry-count"]
	if !ok {
		return 0
	}
	switch count := v.(type) {
	case int32:
		return int(count)
	case int64:
		return int(count)
	case int:
		return count
	default:
		return 0
	}
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true

	c.observability.Logger().Info(context.Background(), "consumer closed",
		observability.String("queue", c.queue),
	)

	return nil
}
