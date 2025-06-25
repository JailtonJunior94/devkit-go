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
		kafka.WithTopicName("conclusao_corte_local"),
		kafka.WithOffset(kafka.FirstOffset),
		kafka.WithTopicNameDLT("conclusao_corte_local_dlq"),
		kafka.WithConsumerGroupID("order-consumer-group"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Registrar handlers
	consumer.RegisterHandler("", func(ctx context.Context, headers map[string]string, body []byte) error {
		log.Printf("Processing event_type_1: %s", string(body))
		return nil
	})

	// Consumir mensagens com worker pool
	if err := consumer.ConsumeWithWorkerPool(ctx, 50); err != nil {
		log.Fatalf("failed to consume messages: %v", err)
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
