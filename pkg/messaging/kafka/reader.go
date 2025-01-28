package kafka

import (
	"context"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/segmentio/kafka-go"
)

type reader struct {
	consumer *kafka.Reader
}

func (b *broker) NewConsumerFromBroker() (messaging.Consumer, error) {

	return &reader{}, nil
}

func (k *reader) Consume(ctx context.Context) error {
	for {
		m, err := k.consumer.ReadMessage(ctx)
		if err != nil {
			return err
		}
		log.Printf("Message on %s: %s\n", m.Topic, m.Value)
	}
}

func (k *reader) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	// Not implemented
}
