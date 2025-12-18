package rabbitmqfx

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	"go.uber.org/fx"
)

// Module provides RabbitMQ components with lifecycle management.
// Usage:
//
//	fx.New(
//	    rabbitmqfx.Module,
//	    fx.Supply(&rabbitmq.Config{URL: "amqp://..."}),
//	)
var Module = fx.Module("rabbitmq",
	fx.Provide(
		ProvideConnectionPool,
		ProvidePublisher,
	),
)

// ConsumerModule provides RabbitMQ consumer with lifecycle.
// Usage:
//
//	fx.New(
//	    rabbitmqfx.Module,
//	    rabbitmqfx.ConsumerModule,
//	    fx.Supply(rabbitmqfx.ConsumerConfig{...}),
//	    fx.Provide(fx.Annotate(
//	        rabbitmqfx.ProvideHandler("order.created", handleOrderCreated),
//	        fx.ResultTags(`group:"consumer_handlers"`),
//	    )),
//	)
var ConsumerModule = fx.Module("rabbitmq-consumer",
	fx.Provide(ProvideConsumer),
	fx.Invoke(StartConsumer),
)

// ConfigModule provides default RabbitMQ config.
var ConfigModule = fx.Provide(rabbitmq.DefaultConfig)

// ConnectionPoolParams contains dependencies for creating a connection pool.
type ConnectionPoolParams struct {
	fx.In

	Config *rabbitmq.Config
	LC     fx.Lifecycle
}

// ConnectionPoolResult contains the pool output.
type ConnectionPoolResult struct {
	fx.Out

	Pool *rabbitmq.ConnectionPool
}

// ProvideConnectionPool creates a connection pool with lifecycle management.
func ProvideConnectionPool(p ConnectionPoolParams) (ConnectionPoolResult, error) {
	pool, err := rabbitmq.NewConnectionPool(p.Config)
	if err != nil {
		return ConnectionPoolResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return pool.Close()
		},
	})

	return ConnectionPoolResult{Pool: pool}, nil
}

// PublisherParams contains dependencies for creating a publisher.
type PublisherParams struct {
	fx.In

	Pool *rabbitmq.ConnectionPool
	LC   fx.Lifecycle
}

// PublisherResult contains the publisher output.
type PublisherResult struct {
	fx.Out

	Publisher messaging.Publisher
}

// ProvidePublisher creates a RabbitMQ publisher with lifecycle management.
func ProvidePublisher(p PublisherParams) (PublisherResult, error) {
	ch, err := p.Pool.GetChannel()
	if err != nil {
		return PublisherResult{}, err
	}

	publisher := rabbitmq.NewRabbitMQPublisher(ch)

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return publisher.Close()
		},
	})

	return PublisherResult{Publisher: publisher}, nil
}
