package consumer

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"github.com/JailtonJunior94/devkit-go/pkg/vos"

	"github.com/cenkalti/backoff/v4"
)

type consumer struct {
}

func NewConsumer() *consumer {
	return &consumer{}
}

func (s *consumer) Run() {
	ctx := context.Background()

	broker, err := kafka.NewBroker(ctx, []string{"localhost:9092"}, vos.PlainText, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer broker.Close()

	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = time.Second * 3

	consumer, err := broker.NewConsumerFromBroker(
		kafka.WithRetry(100),
		kafka.WithMaxRetries(3),
		kafka.WithBackoff(backoff),
		kafka.WithTopicName("orders"),
		kafka.WithOffset(kafka.FirstOffset),
		kafka.WithTopicNameDLT("orders-dlt"),
		kafka.WithConsumerGroupID("order-consumer-group"),
	)
	if err != nil {
		log.Fatal(err)
	}

	consumer.RegisterHandler("order_created", OrderCreatedHandler)
	consumer.RegisterHandler("order_updated", OrderUpdatedHandler)

	if err := consumer.Consume(ctx); err != nil {
		log.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}

func OrderCreatedHandler(ctx context.Context, params map[string]string, body []byte) error {

	return errors.New("retornando erro para enviar para dlt")

	// log.Println("Received header:OrderCreatedHandler", params)
	// log.Println("Received message:OrderCreatedHandler", string(body))
	// return nil
}

func OrderUpdatedHandler(ctx context.Context, params map[string]string, body []byte) error {
	log.Println("Received header:OrderUpdatedHandler", params)
	log.Println("Received message:OrderUpdatedHandler", string(body))
	return nil
}
