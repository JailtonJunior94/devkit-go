package consumer

import (
	"context"
)

// Message represents a message consumed from a broker.
// This is a generic structure that can be adapted for different
// message brokers (Kafka, RabbitMQ, etc.).
type Message struct {
	// Topic is the name of the topic/queue this message came from
	Topic string

	// Key is the message key (optional, used for partitioning)
	Key []byte

	// Value is the message payload
	Value []byte

	// Headers contains message metadata
	Headers map[string]string

	// Partition is the partition number (for Kafka-like systems)
	Partition int32

	// Offset is the message offset (for Kafka-like systems)
	Offset int64

	// Timestamp is when the message was produced
	Timestamp int64

	// Attempt is the current retry attempt number
	Attempt int

	// Context contains additional context for this message
	Context context.Context
}

// MessageHandler defines the interface for handling messages.
// Handlers must implement the Handle method to process messages.
type MessageHandler interface {
	// Handle processes a single message and returns an error if processing fails.
	Handle(ctx context.Context, msg *Message) error
}

// MessageHandlerFunc is a function type that implements MessageHandler.
// This allows using functions directly as handlers.
type MessageHandlerFunc func(ctx context.Context, msg *Message) error

// Handle implements the MessageHandler interface for MessageHandlerFunc.
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

// Handler is the registration interface for message handlers.
// It allows handlers to register themselves with the consumer.
type Handler interface {
	// Register registers the handler with the consumer server.
	Register(s *Server)
}

// TopicHandler is a simple handler that processes messages for specific topics.
type TopicHandler struct {
	topics  []string
	handler MessageHandler
}

// NewTopicHandler creates a new handler for the specified topics.
func NewTopicHandler(handler MessageHandler, topics ...string) Handler {
	return &TopicHandler{
		topics:  topics,
		handler: handler,
	}
}

// Register implements the Handler interface.
func (h *TopicHandler) Register(s *Server) {
	for _, topic := range h.topics {
		s.registerMessageHandler(topic, h.handler)
	}
}

// FuncHandler creates a Handler from a MessageHandlerFunc.
type FuncHandler struct {
	topics  []string
	handler MessageHandlerFunc
}

// NewFuncHandler creates a new handler from a function for the specified topics.
func NewFuncHandler(topics []string, handler MessageHandlerFunc) Handler {
	return &FuncHandler{
		topics:  topics,
		handler: handler,
	}
}

// Register implements the Handler interface.
func (h *FuncHandler) Register(s *Server) {
	for _, topic := range h.topics {
		s.registerMessageHandler(topic, h.handler)
	}
}

// Example handler implementations

// LoggingHandler is an example handler that logs messages.
type LoggingHandler struct {
	logger interface {
		Info(ctx context.Context, msg string, keysAndValues ...interface{})
	}
}

// Handle implements the MessageHandler interface.
func (h *LoggingHandler) Handle(ctx context.Context, msg *Message) error {
	h.logger.Info(ctx, "received message",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
		"key", string(msg.Key),
		"value_size", len(msg.Value))
	return nil
}

// BatchProcessor is a helper for processing messages in batches.
// This can be useful for optimizing throughput.
type BatchProcessor struct {
	batchSize int
	processor func(ctx context.Context, messages []*Message) error
	batch     []*Message
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(batchSize int, processor func(ctx context.Context, messages []*Message) error) *BatchProcessor {
	return &BatchProcessor{
		batchSize: batchSize,
		processor: processor,
		batch:     make([]*Message, 0, batchSize),
	}
}

// Handle implements the MessageHandler interface.
func (b *BatchProcessor) Handle(ctx context.Context, msg *Message) error {
	b.batch = append(b.batch, msg)

	// Process batch when it reaches the batch size
	if len(b.batch) >= b.batchSize {
		return b.Flush(ctx)
	}

	return nil
}

// Flush processes any remaining messages in the batch.
func (b *BatchProcessor) Flush(ctx context.Context) error {
	if len(b.batch) == 0 {
		return nil
	}

	err := b.processor(ctx, b.batch)
	b.batch = b.batch[:0] // Clear batch
	return err
}
