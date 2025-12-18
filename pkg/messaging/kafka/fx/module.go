package kafkafx

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"go.uber.org/fx"
)

// Module provides Kafka components with lifecycle management.
// Usage:
//
//	fx.New(
//	    kafkafx.Module,
//	    fx.Supply(kafkafx.BrokerConfig{
//	        Brokers:   []string{"localhost:9092"},
//	        Mechanism: vos.PlainText,
//	    }),
//	)
var Module = fx.Module("kafka",
	fx.Provide(
		ProvideBroker,
		ProvideProducer,
	),
)

// ConsumerModule provides Kafka consumer with lifecycle.
// Usage:
//
//	fx.New(
//	    kafkafx.Module,
//	    kafkafx.ConsumerModule,
//	    fx.Supply(kafkafx.ConsumerConfig{...}),
//	    fx.Provide(fx.Annotate(
//	        kafkafx.ProvideHandler("order.created", handleOrderCreated),
//	        fx.ResultTags(`group:"kafka_consumer_handlers"`),
//	    )),
//	)
var ConsumerModule = fx.Module("kafka-consumer",
	fx.Provide(ProvideConsumer),
	fx.Invoke(StartConsumer),
)

// BrokerParams contains dependencies for creating a broker.
type BrokerParams struct {
	fx.In

	Config BrokerConfig
	LC     fx.Lifecycle
}

// BrokerResult contains the broker output.
type BrokerResult struct {
	fx.Out

	Broker kafka.Broker
}

// ProvideBroker creates a Kafka broker with lifecycle management.
func ProvideBroker(p BrokerParams) (BrokerResult, error) {
	ctx := context.Background()

	broker, err := kafka.NewBroker(
		ctx,
		p.Config.Brokers,
		p.Config.Mechanism,
		p.Config.Auth,
	)
	if err != nil {
		return BrokerResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return broker.Close()
		},
	})

	return BrokerResult{Broker: broker}, nil
}

// ProducerParams contains dependencies for creating a producer.
type ProducerParams struct {
	fx.In

	Broker kafka.Broker
	LC     fx.Lifecycle
}

// ProducerResult contains the producer output.
type ProducerResult struct {
	fx.Out

	Publisher messaging.Publisher
}

// ProvideProducer creates a Kafka producer with lifecycle management.
func ProvideProducer(p ProducerParams) (ProducerResult, error) {
	producer, err := p.Broker.NewProducerFromBroker()
	if err != nil {
		return ProducerResult{}, err
	}

	p.LC.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return producer.Close()
		},
	})

	return ProducerResult{Publisher: producer}, nil
}
