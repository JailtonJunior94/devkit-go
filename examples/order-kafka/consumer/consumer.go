package consumer

import (
	"context"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka"
	"github.com/cenkalti/backoff/v4"
)

type consumer struct {
}

func NewConsumer() *consumer {
	return &consumer{}
}

func (s *consumer) Run() {
	client, err := kafka.NewClient([]string{"45.55.105.69:9094"}, &kafka.AuthConfig{
		Username: "admin",
		Password: "nnG66BuJfqZhEs5Tk8Jz8nEAiOeVyyf0",
	})
	if err != nil {
		log.Fatal(err)
	}

	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = time.Second * 1

	consumer := kafka.NewConsumer(
		kafka.WithTopic("orders"),
		kafka.WithGroupID("order-consumer-group"),
		kafka.WithBackoff(backoff),
		kafka.WithMaxRetries(3),
		kafka.WithRetryChan(100),
		kafka.WithDLQ("orders_dlq"),
		kafka.WithClient(client),
	)

	consumer.RegisterHandler("order_created", OrderCreatedHandler)
	consumer.RegisterHandler("order_updated", OrderUpdatedHandler)

	if err := consumer.Consume(context.Background()); err != nil {
		log.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}

func OrderCreatedHandler(ctx context.Context, params map[string]string, body []byte) error {
	log.Println("Received header:OrderCreatedHandler", params)
	log.Println("Received message:OrderCreatedHandler", string(body))
	return nil
}

func OrderUpdatedHandler(ctx context.Context, params map[string]string, body []byte) error {
	log.Println("Received header:OrderUpdatedHandler", params)
	log.Println("Received message:OrderUpdatedHandler", string(body))
	return nil
}
