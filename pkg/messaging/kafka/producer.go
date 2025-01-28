package kafka

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/segmentio/kafka-go"
)

type producer struct {
	producer *kafka.Writer
}

func (b *broker) NewProducerFromBroker() (messaging.Publisher, error) {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: b.brokers,
		Dialer:  b.dialer,
	})
	return &producer{producer: writer}, nil
}

func (k *producer) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	kafkaMessage := kafka.Message{
		Topic: topicOrQueue,
		Key:   []byte(key),
		Value: message.Body,
	}

	for key, value := range headers {
		kafkaMessage.Headers = append(kafkaMessage.Headers, kafka.Header{Key: key, Value: []byte(value)})
	}

	return k.producer.WriteMessages(ctx, kafkaMessage)
}
