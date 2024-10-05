package consumer

import (
	"context"
	"errors"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	OrdersExchange = "order"
	OrderCreated   = "order_created"
	OrderUpdated   = "order_updated"
	OrderQueue     = "order"
	FinanceQueue   = "finance_order"
)

var (
	Exchanges = []*rabbitmq.Exchange{
		rabbitmq.NewExchange(OrdersExchange, "direct"),
	}

	Bindings = []*rabbitmq.Binding{
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderCreated),
		rabbitmq.NewBindingRouting(OrderQueue, OrdersExchange, OrderUpdated),
		rabbitmq.NewBindingRouting(FinanceQueue, OrdersExchange, OrderCreated),
	}
)

type consumer struct {
}

func NewConsumer() *consumer {
	return &consumer{}
}

func (s *consumer) Run() {
	connection, err := amqp.Dial("amqp://guest:pass@rabbitmq@localhost:5672")
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	consumer, err := rabbitmq.NewConsumer(
		rabbitmq.WithName("order"),
		rabbitmq.WithConnection(connection),
		rabbitmq.WithChannel(channel),
		rabbitmq.WithQueue(OrderQueue),
		rabbitmq.WithHandler(handlerMessage),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := consumer.Consume(context.Background()); err != nil {
		log.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}

func handlerMessage(ctx context.Context, body []byte) error {
	log.Println("Received message:", string(body))
	return errors.New("deu ruim")
}
