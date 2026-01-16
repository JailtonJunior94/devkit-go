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
	// Error channel configuration
	defaultErrorChannelSize    = 1000 // Increased from 100 to reduce dropped errors
	errorChannelWarnThreshold  = 800  // Warn when 80% full
	errorChannelMonitoringTick = 10 * time.Second
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

	// Error channel monitoring
	droppedErrors     atomic.Uint64 // Count of errors dropped due to full channel
	lastDroppedErrLog atomic.Int64  // Unix timestamp of last dropped error log (rate limiting)

	// Shutdown coordination
	shutdownOnce       sync.Once
	wg                 sync.WaitGroup
	monitoringCancel   context.CancelFunc
	monitoringShutdown chan struct{}
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
		reader:             reader,
		config:             cfg,
		consumerCfg:        consumerCfg,
		handlers:           make(map[string][]messaging.ConsumeHandler),
		errorCh:            make(chan error, defaultErrorChannelSize),
		monitoringShutdown: make(chan struct{}),
	}

	// Initialize DLQ if enabled
	if err := c.initializeDLQ(); err != nil {
		return nil, fmt.Errorf("failed to initialize DLQ: %w", err)
	}

	// Start error channel monitoring goroutine
	// This prevents memory leaks by monitoring channel fullness and logging warnings
	c.startErrorChannelMonitoring()

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

	// Cancelable context for workers
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	// Start workers with WaitGroup
	for i := range workerCount {
		c.wg.Add(1)
		go func(id int) {
			defer c.wg.Done()
			c.worker(workerCtx, id, messageCh)
		}(i)
	}

	// Fetcher goroutine
	c.wg.Add(1)
	fetcherDone := make(chan struct{})
	go func() {
		defer c.wg.Done()
		defer close(messageCh)
		defer close(fetcherDone)

		for {
			// Check shutdown
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

	// Wait for shutdown signal
	<-fetcherDone

	c.config.logger.Info(ctx, "fetcher stopped, waiting for workers to finish")

	// Wait for all workers to finish processing
	c.wg.Wait()

	c.config.logger.Info(ctx, "all workers finished")

	return workerCtx.Err()
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
	var closeErr error

	c.shutdownOnce.Do(func() {
		if c.closed.Swap(true) {
			return // Already closed
		}

		c.config.logger.Info(context.Background(), "closing consumer, waiting for in-flight messages")

		// Stop error channel monitoring
		c.stopErrorChannelMonitoring()

		// Wait for workers to finish (with timeout)
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

		// Log final dropped error count if any
		if dropped := c.droppedErrors.Load(); dropped > 0 {
			c.config.logger.Warn(context.Background(),
				"consumer closed with dropped errors - configure error consumption to prevent loss",
				Field{Key: "total_dropped", Value: dropped},
			)
		}

		// Close error channel
		close(c.errorCh)

		// Close reader
		if err := c.reader.Close(); err != nil {
			c.config.logger.Error(context.Background(), "error closing reader",
				Field{Key: "error", Value: err})
			closeErr = err
		}

		c.config.logger.Info(context.Background(), "consumer closed")
	})

	return closeErr
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
func (c *consumer) worker(ctx context.Context, id int, messageCh <-chan kafka.Message) {
	c.config.logger.Debug(ctx, "worker started",
		Field{Key: "worker_id", Value: id})

	defer func() {
		c.config.logger.Debug(ctx, "worker stopped",
			Field{Key: "worker_id", Value: id})
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			// Panic recovery per message
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

// handlePanic handles panics that occur during message processing.
func (c *consumer) handlePanic(ctx context.Context, workerID int, msg kafka.Message, panicValue interface{}) {
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

// processMessage processes a single message.
func (c *consumer) processMessage(ctx context.Context, msg kafka.Message) {
	// Check context before processing
	select {
	case <-ctx.Done():
		return
	default:
	}

	headers := extractHeaders(msg)
	eventType := headers["event_type"]

	// RACE CONDITION FIX: Make defensive copy of handlers slice while holding lock
	// This prevents concurrent modification by RegisterHandler() during iteration
	c.mu.RLock()
	handlersInMap, ok := c.handlers[eventType]
	if !ok {
		c.mu.RUnlock()
		c.config.logger.Warn(ctx, "no handler for event type", Field{Key: "event_type", Value: eventType})
		return
	}

	// Create a defensive copy to avoid holding lock during message processing
	// This is critical for performance and prevents potential deadlocks
	handlersCopy := make([]messaging.ConsumeHandler, len(handlersInMap))
	copy(handlersCopy, handlersInMap)
	c.mu.RUnlock()

	// If instrumentation is enabled, wrap the consumption
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

	// Fallback: execute directly without tracing
	c.processMessageInternal(ctx, msg, headers, eventType, handlersCopy)
}

// processMessageInternal contains the core message processing logic without instrumentation.
func (c *consumer) processMessageInternal(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	// Process with DLQ retry logic
	if c.config.dlqConfig.Enabled {
		c.processMessageWithDLQ(ctx, msg, headers, eventType, handlers)
		return
	}

	// Process without DLQ - execute all handlers first, then commit
	c.processMessageWithoutDLQ(ctx, msg, headers, eventType, handlers)
}

// processMessageWithDLQ processes message with DLQ retry logic.
func (c *consumer) processMessageWithDLQ(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	var lastError error

	for _, handler := range handlers {
		// Check context before each handler
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := c.handleMessageWithRetry(ctx, msg, handler); err != nil {
			lastError = err
			c.sendError(err)
			// Continue processing other handlers even if one fails
			// Each handler gets retry logic independently
		}
	}

	// Note: When DLQ is enabled, handleMessageWithRetry handles commit internally
	// on success, so we don't need to commit here
	_ = lastError
}

// processMessageWithoutDLQ processes message without DLQ - all handlers must succeed before commit.
func (c *consumer) processMessageWithoutDLQ(ctx context.Context, msg kafka.Message, headers map[string]string, eventType string, handlers []messaging.ConsumeHandler) {
	var allSuccess = true

	// Execute ALL handlers first
	for _, handler := range handlers {
		// Check context before each handler
		select {
		case <-ctx.Done():
			return
		default:
		}

		var err error

		// If instrumentation is enabled, wrap the handler execution
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
			// Continue to try other handlers, but mark as failed
		}
	}

	// Only commit if ALL handlers succeeded
	if allSuccess {
		// Check if reader is initialized (nil in unit tests)
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

// sendError sends an error to the error channel (non-blocking).
// If the channel is full, the error is dropped and logged to prevent memory leaks.
// This is a fail-safe mechanism when consumers don't read from Errors() channel.
func (c *consumer) sendError(err error) {
	select {
	case c.errorCh <- err:
		// Successfully sent to channel
	default:
		// Channel is full - drop error to prevent goroutine blocking
		// MEMORY LEAK PREVENTION: Do NOT block here or goroutines will leak
		dropped := c.droppedErrors.Add(1)

		// Rate-limited logging: only log every 10 seconds to prevent log spam
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

// DroppedErrors returns the count of errors dropped due to full channel.
// This is useful for monitoring and alerting.
func (c *consumer) DroppedErrors() uint64 {
	return c.droppedErrors.Load()
}

// startErrorChannelMonitoring starts a background goroutine that monitors
// the error channel fullness and logs warnings to prevent silent data loss.
//
// Memory Leak Prevention:
// Without monitoring, if users don't consume from Errors() channel, the buffer
// fills up and errors are silently dropped. This goroutine:
// 1. Monitors channel fullness every 10 seconds
// 2. Warns when channel is 80% full
// 3. Helps operators detect misconfiguration before errors are lost
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

// stopErrorChannelMonitoring stops the error channel monitoring goroutine.
func (c *consumer) stopErrorChannelMonitoring() {
	if c.monitoringCancel != nil {
		c.monitoringCancel()
		// Wait for monitoring goroutine to finish (with timeout)
		select {
		case <-c.monitoringShutdown:
			// Monitoring stopped gracefully
		case <-time.After(5 * time.Second):
			// Timeout - monitoring goroutine didn't stop, but we continue shutdown
		}
	}
}

// checkErrorChannelHealth checks if the error channel is becoming full
// and logs warnings to help operators detect issues early.
func (c *consumer) checkErrorChannelHealth() {
	channelLen := len(c.errorCh)
	channelCap := cap(c.errorCh)

	// Warn when channel is 80% full
	if channelLen >= errorChannelWarnThreshold {
		droppedCount := c.droppedErrors.Load()
		c.config.logger.Warn(context.Background(),
			"error channel approaching capacity - consume from Errors() to prevent data loss",
			Field{Key: "current_length", Value: channelLen},
			Field{Key: "capacity", Value: channelCap},
			Field{Key: "fullness_percent", Value: (channelLen * 100) / channelCap},
			Field{Key: "dropped_so_far", Value: droppedCount},
		)
	}

	// Info log if channel has items but isn't critical (for debugging)
	if channelLen > 0 && channelLen < errorChannelWarnThreshold {
		c.config.logger.Debug(context.Background(),
			"error channel has buffered errors",
			Field{Key: "buffered_errors", Value: channelLen},
			Field{Key: "capacity", Value: channelCap},
		)
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
