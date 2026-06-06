package kafka

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

const (
	defaultErrorChannelSize    = 1000
	errorChannelWarnThreshold  = 800
	errorChannelMonitoringTick = 10 * time.Second
)

type ConsumerOption func(*consumerConfig)

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

	droppedErrors     atomic.Uint64
	lastDroppedErrLog atomic.Int64

	shutdownOnce       sync.Once
	wg                 sync.WaitGroup
	startMu            sync.Mutex
	monitoringCancel   context.CancelFunc
	monitoringShutdown chan struct{}
}

func WithGroupID(groupID string) ConsumerOption {
	return func(c *consumerConfig) {
		c.groupID = groupID
	}
}

func WithTopics(topics ...string) ConsumerOption {
	return func(c *consumerConfig) {
		c.topics = topics
	}
}

func WithStartOffset(offset int64) ConsumerOption {
	return func(c *consumerConfig) {
		c.startOffset = offset
	}
}
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
		reader:             reader,
		config:             cfg,
		consumerCfg:        consumerCfg,
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, defaultErrorChannelSize),
		monitoringShutdown: make(chan struct{}),
	}

	if err := c.initializeDLQ(); err != nil {
		return nil, fmt.Errorf("failed to initialize DLQ: %w", err)
	}

	c.startErrorChannelMonitoring()

	return c, nil
}

func (c *consumer) Consume(ctx context.Context) error {
	c.startMu.Lock()
	if c.closed.Load() {
		c.startMu.Unlock()
		return ErrConsumerClosed
	}
	c.wg.Add(1)
	c.startMu.Unlock()

	go func() {
		defer c.wg.Done()
		c.consumeLoop(ctx)
	}()

	return nil
}

func (c *consumer) ConsumeBatch(ctx context.Context) error {
	return c.Consume(ctx)
}

func (c *consumer) ConsumeWithWorkerPool(ctx context.Context, workerCount int) error {
	c.startMu.Lock()
	if c.closed.Load() {
		c.startMu.Unlock()
		return ErrConsumerClosed
	}
	c.wg.Add(workerCount + 1)
	c.startMu.Unlock()

	messageCh := make(chan kafka.Message, workerCount*2)

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	for i := range workerCount {
		go func(id int) {
			defer c.wg.Done()
			c.worker(workerCtx, id, messageCh)
		}(i)
	}

	fetcherDone := make(chan struct{})
	go func() {
		defer c.wg.Done()
		defer close(messageCh)
		defer close(fetcherDone)

		for {
			select {
			case <-workerCtx.Done():
				return
			default:
			}

			if c.closed.Load() {
				return
			}

			msg, err := c.reader.FetchMessage(workerCtx)
			if err != nil {
				if workerCtx.Err() != nil {
					return
				}
				c.sendError(err)
				continue
			}

			select {
			case messageCh <- msg:
			case <-workerCtx.Done():
				return
			}
		}
	}()

	<-fetcherDone

	c.config.logger.Info(ctx, "fetcher stopped, waiting for workers to finish")

	c.wg.Wait()

	c.config.logger.Info(ctx, "all workers finished")

	return workerCtx.Err()
}

func (c *consumer) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

func (c *consumer) Errors() <-chan error {
	return c.errorCh
}

func (c *consumer) Close() error {
	var closeErr error

	c.shutdownOnce.Do(func() {
		c.startMu.Lock()
		if c.closed.Swap(true) {
			c.startMu.Unlock()
			return
		}
		c.startMu.Unlock()

		c.config.logger.Info(context.Background(), "closing consumer, waiting for in-flight messages")

		c.stopErrorChannelMonitoring()

		if err := c.reader.Close(); err != nil {
			c.config.logger.Error(context.Background(), "error closing reader",
				Field{Key: "error", Value: err})
			closeErr = err
		}

		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		timeout := time.NewTimer(30 * time.Second)
		defer timeout.Stop()

		select {
		case <-done:
			c.config.logger.Info(context.Background(), "all workers finished gracefully")
		case <-timeout.C:
			c.config.logger.Warn(context.Background(), "timeout waiting for workers, forcing shutdown")
		}

		if dropped := c.droppedErrors.Load(); dropped > 0 {
			c.config.logger.Warn(context.Background(),
				"consumer closed with dropped errors - configure error consumption to prevent loss",
				Field{Key: "total_dropped", Value: dropped},
			)
		}

		close(c.errorCh)

		c.config.logger.Info(context.Background(), "consumer closed")
	})

	return closeErr
}

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

func (c *consumer) worker(ctx context.Context, id int, messageCh <-chan kafka.Message) {
	c.config.logger.Debug(ctx, "worker started", Field{Key: "worker_id", Value: id})
	defer c.config.logger.Debug(ctx, "worker stopped", Field{Key: "worker_id", Value: id})

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						c.handlePanic(ctx, id, msg, r)
					}
				}()

				c.processMessage(ctx, msg)
			}()
		}
	}
}

func (c *consumer) handlePanic(ctx context.Context, workerID int, msg kafka.Message, panicValue any) {
	c.config.logger.Error(ctx, "PANIC in message handler",
		Field{Key: "worker_id", Value: workerID},
		Field{Key: "panic", Value: panicValue},
		Field{Key: "topic", Value: msg.Topic},
		Field{Key: "partition", Value: msg.Partition},
		Field{Key: "offset", Value: msg.Offset},
	)

	// Send to DLQ if configured
	if c.config.dlqConfig.Enabled && c.dlqStrategy != nil {
		state := &retryState{
			attempts:     c.config.dlqConfig.MaxRetries,
			firstAttempt: time.Now(),
			retryHistory: []Retry{{
				Attempt:   1,
				Timestamp: time.Now(),
				Error:     fmt.Sprintf("PANIC: %v", panicValue),
			}},
		}

		if err := c.sendToDLQ(ctx, msg, fmt.Errorf("panic: %v", panicValue), state); err != nil {
			c.config.logger.Error(ctx, "failed to send panic message to DLQ",
				Field{Key: "error", Value: err})
		}
	}

	c.sendError(fmt.Errorf("panic in handler: %v", panicValue))
}

func (c *consumer) processMessage(ctx context.Context, msg kafka.Message) {
	// Check context before processing
	select {
	case <-ctx.Done():
		return
	default:
	}

	headers := extractHeaders(msg)
	eventType := headers["event_type"]

	c.mu.RLock()
	handlersInMap, ok := c.handlers[eventType]
	if !ok {
		c.mu.RUnlock()
		c.config.logger.Warn(ctx, "no handler for event type", Field{Key: "event_type", Value: eventType})
		return
	}

	handlersCopy := make([]messaging.ConsumeHandler, len(handlersInMap))
	copy(handlersCopy, handlersInMap)
	c.mu.RUnlock()

	if c.config.instrumentation != nil {
		_ = c.config.instrumentation.InstrumentConsume(
			ctx,
			msg.Topic,
			msg.Partition,
			msg.Offset,
			string(msg.Key),
			headers,
			c.consumerCfg.groupID,
			func(ctx context.Context) error {
				c.processMessageInternal(ctx, msg, headers, eventType, handlersCopy)
				return nil
			},
		)
		return
	}

	c.processMessageInternal(ctx, msg, headers, eventType, handlersCopy)
}

func (c *consumer) processMessageInternal(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	if c.config.dlqConfig.Enabled {
		c.processMessageWithDLQ(ctx, msg, headers, eventType, handlers)
		return
	}

	c.processMessageWithoutDLQ(ctx, msg, headers, eventType, handlers)
}

func (c *consumer) processMessageWithDLQ(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	dlqFailed := false

	for _, handler := range handlers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := c.handleMessageWithRetry(ctx, msg, handler); err != nil {
			dlqFailed = true
			c.sendError(err)
		}
	}

	if !dlqFailed && c.reader != nil {
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.config.logger.Error(ctx, "failed to commit message",
				Field{Key: "error", Value: err},
			)
			c.sendError(err)
		}
	}
}

func (c *consumer) processMessageWithoutDLQ(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	allSuccess := true

	for _, handler := range handlers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var err error

		if c.config.instrumentation != nil {
			err = c.config.instrumentation.InstrumentHandler(ctx, eventType, func(ctx context.Context) error {
				return handler(ctx, headers, msg.Value)
			})
		} else {
			err = handler(ctx, headers, msg.Value)
		}

		if err != nil {
			c.config.logger.Error(ctx, "handler error",
				Field{Key: "event_type", Value: eventType},
				Field{Key: "error", Value: err},
			)
			c.sendError(err)
			allSuccess = false
		}
	}

	if allSuccess {
		if c.reader != nil {
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.config.logger.Error(ctx, "failed to commit message",
					Field{Key: "error", Value: err},
				)
				c.sendError(err)
			}
		}
	} else {
		c.config.logger.Warn(ctx, "message not committed due to handler failures",
			Field{Key: "event_type", Value: eventType},
			Field{Key: "topic", Value: msg.Topic},
			Field{Key: "partition", Value: msg.Partition},
			Field{Key: "offset", Value: msg.Offset},
		)
	}
}

func (c *consumer) sendError(err error) {
	select {
	case c.errorCh <- err:
	default:
		dropped := c.droppedErrors.Add(1)

		now := time.Now().Unix()
		lastLog := c.lastDroppedErrLog.Load()
		if now-lastLog >= 10 {
			if c.lastDroppedErrLog.CompareAndSwap(lastLog, now) {
				c.config.logger.Warn(context.Background(),
					"error channel full, dropping errors - consume from Errors() channel to prevent loss",
					Field{Key: "dropped_total", Value: dropped},
					Field{Key: "channel_size", Value: cap(c.errorCh)},
					Field{Key: "latest_dropped_error", Value: err.Error()},
				)
			}
		}
	}
}

func (c *consumer) DroppedErrors() uint64 {
	return c.droppedErrors.Load()
}

func (c *consumer) startErrorChannelMonitoring() {
	monitoringCtx, cancel := context.WithCancel(context.Background())
	c.monitoringCancel = cancel

	go func() {
		ticker := time.NewTicker(errorChannelMonitoringTick)
		defer ticker.Stop()
		defer close(c.monitoringShutdown)

		for {
			select {
			case <-monitoringCtx.Done():
				return
			case <-ticker.C:
				c.checkErrorChannelHealth()
			}
		}
	}()
}

func (c *consumer) stopErrorChannelMonitoring() {
	if c.monitoringCancel != nil {
		c.monitoringCancel()
		select {
		case <-c.monitoringShutdown:
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *consumer) checkErrorChannelHealth() {
	channelLen := len(c.errorCh)
	channelCap := cap(c.errorCh)

	if channelLen >= errorChannelWarnThreshold {
		c.config.logger.Warn(context.Background(),
			"error channel approaching capacity - consume from Errors() to prevent data loss",
			Field{Key: "current_length", Value: channelLen},
			Field{Key: "capacity", Value: channelCap},
			Field{Key: "fullness_percent", Value: (channelLen * 100) / channelCap},
			Field{Key: "dropped_so_far", Value: c.droppedErrors.Load()},
		)
	}

	if channelLen > 0 && channelLen < errorChannelWarnThreshold {
		c.config.logger.Debug(context.Background(),
			"error channel has buffered errors",
			Field{Key: "buffered_errors", Value: channelLen},
			Field{Key: "capacity", Value: channelCap},
		)
	}
}

func extractHeaders(msg kafka.Message) map[string]string {
	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}
	return headers
}
