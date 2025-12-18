package rabbitmqfx

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	"go.uber.org/fx"
)

// ConsumerConfig holds configuration for creating a consumer.
type ConsumerConfig struct {
	// Name is the consumer name prefix.
	Name string

	// Queue is the queue to consume from.
	Queue string

	// Prefetch is the number of messages to prefetch.
	// Default: 10
	Prefetch int

	// AutoAck enables automatic message acknowledgment.
	// Default: false
	AutoAck bool

	// Exclusive makes this an exclusive consumer.
	// Default: false
	Exclusive bool

	// NoLocal prevents consuming messages published by this connection.
	// Default: false
	NoLocal bool

	// NoWait disables waiting for server confirmation.
	// Default: false
	NoWait bool

	// WorkerCount is the number of worker goroutines for ConsumeWithWorkerPool.
	// Default: 1 (uses Consume instead of ConsumeWithWorkerPool)
	WorkerCount int
}

// DefaultConsumerConfig returns a default consumer configuration.
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Name:        "consumer",
		Prefetch:    10,
		AutoAck:     false,
		Exclusive:   false,
		NoLocal:     false,
		NoWait:      false,
		WorkerCount: 1,
	}
}

// Handler wraps a handler with its event type (routing key).
type Handler struct {
	// EventType is the routing key to handle.
	EventType string
	// Handler is the function that processes messages.
	Handler messaging.ConsumeHandler
}

// ConsumerParams contains dependencies for creating a consumer.
type ConsumerParams struct {
	fx.In

	Pool     *rabbitmq.ConnectionPool
	Config   ConsumerConfig    `optional:"true"`
	Handlers []Handler         `group:"consumer_handlers"`
	LC       fx.Lifecycle
}

// ConsumerResult contains the consumer output.
type ConsumerResult struct {
	fx.Out

	Consumer messaging.Consumer
}

// ProvideConsumer creates a RabbitMQ consumer with lifecycle management.
func ProvideConsumer(p ConsumerParams) (ConsumerResult, error) {
	cfg := p.Config
	if cfg.Queue == "" {
		cfg = DefaultConsumerConfig()
	}

	ch, err := p.Pool.GetChannel()
	if err != nil {
		return ConsumerResult{}, err
	}

	opts := []rabbitmq.Option{
		rabbitmq.WithChannel(ch),
		rabbitmq.WithName(cfg.Name),
		rabbitmq.WithQueue(cfg.Queue),
		rabbitmq.WithPrefetch(cfg.Prefetch),
		rabbitmq.WithAutoAck(cfg.AutoAck),
		rabbitmq.WithExclusive(cfg.Exclusive),
		rabbitmq.WithNoLocal(cfg.NoLocal),
		rabbitmq.WithNoWait(cfg.NoWait),
	}

	consumer, err := rabbitmq.NewConsumer(opts...)
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
//	    rabbitmqfx.ProvideHandler("order.created", handleOrderCreated),
//	    fx.ResultTags(`group:"consumer_handlers"`),
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
//	    func(svc *OrderService) rabbitmqfx.Handler {
//	        return rabbitmqfx.Handler{
//	            EventType: "order.created",
//	            Handler:   svc.HandleOrderCreated,
//	        }
//	    },
//	    fx.ResultTags(`group:"consumer_handlers"`),
//	))
func ProvideHandlerFunc(fn func() Handler) func() Handler {
	return fn
}
