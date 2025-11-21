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
		rabbitmq.ConfigModule,
		fx.Provide(
			rabbitmq.ProvideConnectionPool,
			NewConsumerManager,
		),
		fx.Invoke(RegisterLifecycleHooks),
	)

	if err := app.Start(context.Background()); err != nil {
		log.Fatalf("failed to start application: %v", err)
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
	ch, err := c.pool.GetChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel from pool: %v", err)
	}
	defer c.pool.ReturnChannel(ch)

	_, err = rabbitmq.NewAmqpBuilder(ch).
		DeclareExchanges(Exchanges...).
		DeclareBindings(Bindings...).
		WithDLQ().
		WithRetry().
		DeclareTTL(3 * time.Second).
		DeclarePrefetchCount(10).
		Apply()
	if err != nil {
		return fmt.Errorf("failed to setup AMQP topology: %v", err)
	}

	consumer, err := rabbitmq.NewConsumer(
		rabbitmq.WithChannel(ch),
		rabbitmq.WithName("order-processor"),
		rabbitmq.WithQueue(OrderQueue),
		rabbitmq.WithPrefetch(5),
		rabbitmq.WithAutoAck(false),
	)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %v", err)
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
	consumer messaging.Consumer
	pool     *rabbitmq.ConnectionPool
}

func NewConsumerManager(pool *rabbitmq.ConnectionPool) *ConsumerManager {
	return &ConsumerManager{pool: pool}
}

func CustomerUpdatedHandler(ctx context.Context, params map[string]string, body []byte) error {
	log.Println("received header:CustomerUpdatedHandler", params)
	log.Println("received message:CustomerUpdatedHandler", string(body))
	return nil
}
