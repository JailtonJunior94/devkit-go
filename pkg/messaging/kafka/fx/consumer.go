package kafkafx

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"go.uber.org/fx"
)

// ConsumerConfig holds configuration for creating a Kafka consumer.
type ConsumerConfig struct {
	// TopicName is the topic to consume from.
	TopicName string

	// ConsumerGroupID is the consumer group ID.
	ConsumerGroupID string

	// Offset is the starting offset.
	// Use kafka.LastOffset (-1) for latest or kafka.FirstOffset (-2) for earliest.
	// Default: kafka.LastOffset
	Offset int64

	// TopicNameDLT is the dead letter topic name.
	// If set, enables dead letter topic functionality.
	TopicNameDLT string

	// MaxRetries is the maximum number of retry attempts.
	// Default: 3
	MaxRetries int

	// EnableRetry enables retry functionality.
	// Default: false
	EnableRetry bool

	// RetryChanSize is the size of the retry channel.
	// Default: 100
	RetryChanSize int

	// WorkerCount is the number of worker goroutines for ConsumeWithWorkerPool.
	// Default: 1 (uses Consume instead of ConsumeWithWorkerPool)
	WorkerCount int
}

// DefaultConsumerConfig returns a default consumer configuration.
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Offset:        kafka.LastOffset,
		MaxRetries:    3,
		EnableRetry:   false,
		RetryChanSize: 100,
		WorkerCount:   1,
	}
}

// Handler wraps a handler with its event type.
type Handler struct {
	// EventType is the event type to handle (from message headers).
	EventType string
	// Handler is the function that processes messages.
	Handler messaging.ConsumeHandler
}

// ConsumerParams contains dependencies for creating a consumer.
type ConsumerParams struct {
	fx.In

	Broker   kafka.Broker
	Config   ConsumerConfig `optional:"true"`
	Handlers []Handler      `group:"kafka_consumer_handlers"`
	LC       fx.Lifecycle
}

// ConsumerResult contains the consumer output.
type ConsumerResult struct {
	fx.Out

	Consumer messaging.Consumer
}

// ProvideConsumer creates a Kafka consumer with lifecycle management.
func ProvideConsumer(p ConsumerParams) (ConsumerResult, error) {
	cfg := p.Config
	if cfg.TopicName == "" {
		cfg = DefaultConsumerConfig()
	}

	opts := []kafka.Options{
		kafka.WithTopicName(cfg.TopicName),
		kafka.WithConsumerGroupID(cfg.ConsumerGroupID),
		kafka.WithOffset(cfg.Offset),
	}

	if cfg.MaxRetries > 0 {
		opts = append(opts, kafka.WithMaxRetries(cfg.MaxRetries))
	}

	if cfg.TopicNameDLT != "" {
		opts = append(opts, kafka.WithTopicNameDLT(cfg.TopicNameDLT))
	}

	if cfg.EnableRetry {
		retryChanSize := cfg.RetryChanSize
		if retryChanSize <= 0 {
			retryChanSize = 100
		}
		opts = append(opts, kafka.WithRetry(retryChanSize))
	}

	consumer, err := p.Broker.NewConsumerFromBroker(opts...)
	if err != nil {
		return ConsumerResult{}, err
	}

	// Register all handlers
	for _, h := range p.Handlers {
		consumer.RegisterHandler(h.EventType, h.Handler)
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return consumer.Close()
		},
	})

	return ConsumerResult{Consumer: consumer}, nil
}

// ConsumerLifecycleParams contains dependencies for starting the consumer.
type ConsumerLifecycleParams struct {
	fx.In

	Consumer messaging.Consumer
	Config   ConsumerConfig `optional:"true"`
}

// StartConsumer registers consumer start with FX lifecycle.
func StartConsumer(lc fx.Lifecycle, p ConsumerLifecycleParams) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Config.WorkerCount > 1 {
				return p.Consumer.ConsumeWithWorkerPool(ctx, p.Config.WorkerCount)
			}
			return p.Consumer.Consume(ctx)
		},
	})
}

// ProvideHandler is a helper to provide consumer handlers.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    kafkafx.ProvideHandler("order.created", handleOrderCreated),
//	    fx.ResultTags(`group:"kafka_consumer_handlers"`),
//	))
func ProvideHandler(eventType string, handler messaging.ConsumeHandler) func() Handler {
	return func() Handler {
		return Handler{
			EventType: eventType,
			Handler:   handler,
		}
	}
}

// ProvideHandlerFunc is a helper to provide consumer handlers from a function.
// Usage:
//
//	fx.Provide(fx.Annotate(
//	    func(svc *OrderService) kafkafx.Handler {
//	        return kafkafx.Handler{
//	            EventType: "order.created",
//	            Handler:   svc.HandleOrderCreated,
//	        }
//	    },
//	    fx.ResultTags(`group:"kafka_consumer_handlers"`),
//	))
func ProvideHandlerFunc(fn func() Handler) func() Handler {
	return fn
}
