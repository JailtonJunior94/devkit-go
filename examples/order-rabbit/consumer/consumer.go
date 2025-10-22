package consumer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"
	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"

	"go.uber.org/fx"
)

const (
	OrdersExchange = "order"
	OrderCreated   = "order_created"
	OrderUpdated   = "order_updated"
	OrderQueue     = "order"
)

var (
	Exchanges = []*rabbitmq.Exchange{
		rabbitmq.NewExchange(OrdersExchange, "direct"),
	}

	Bindings = []*rabbitmq.Binding{
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderCreated),
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderUpdated),
	}
)

func Run() {
	app := fx.New(
		fx.Provide(
			rabbitmq.ProvideRabbitMQConnection,
			NewConsumerManager,
		),
		fx.Invoke(RegisterLifecycleHooks),
	)

	if err := app.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	<-app.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Stop(ctx); err != nil {
		log.Fatalf("failed to stop application gracefully: %v", err)
	}

	log.Println("application stopped successfully")
}

func RegisterLifecycleHooks(lc fx.Lifecycle, consumerManager *ConsumerManager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Println("Starting RabbitMQ consumer application...")
			return consumerManager.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Stopping RabbitMQ consumer application...")
			return consumerManager.Stop(ctx)
		},
	})
}

func (c *ConsumerManager) Start(ctx context.Context) error {
	_, err := rabbitmq.NewAmqpBuilder(c.connection.Channel).
		DeclareExchanges(Exchanges...).
		DeclareBindings(Bindings...).
		WithDLQ().
		WithRetry().
		DeclareTTL(3 * time.Second).
		DeclarePrefetchCount(10).
		Apply()
	if err != nil {
		return fmt.Errorf("failed to setup AMQP topology: %w", err)
	}

	consumer, err := rabbitmq.NewConsumer(
		rabbitmq.WithName("order-processor"),
		rabbitmq.WithConnection(c.connection.Connection),
		rabbitmq.WithChannel(c.connection.Channel),
		rabbitmq.WithQueue(OrderQueue),
		rabbitmq.WithPrefetch(5),
		rabbitmq.WithAutoAck(false),
	)
	if err != nil {
		return err
	}

	consumer.RegisterHandler(OrderCreated, CustomerUpdatedHandler)

	go func() {
		if err := consumer.Consume(ctx); err != nil {
			log.Println("failed to start consumer:", err)
		}
	}()

	return nil
}

func (c *ConsumerManager) Stop(ctx context.Context) error {
	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("error close consumer: %v", err)
	}
	return nil
}

type ConsumerManager struct {
	consumer   messaging.Consumer
	connection *rabbitmq.RabbitMQConnection
}

func NewConsumerManager(connection *rabbitmq.RabbitMQConnection) *ConsumerManager {
	return &ConsumerManager{connection: connection}
}

func CustomerUpdatedHandler(ctx context.Context, params map[string]string, body []byte) error {
	log.Println("Received header:CustomerUpdatedHandler", params)
	log.Println("Received message:CustomerUpdatedHandler", string(body))
	return nil
}
