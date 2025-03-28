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

func (k *producer) PublishBatch(ctx context.Context, topicOrQueue, key string, headers map[string]string, messages []*messaging.Message) error {
	kafkaMessages := make([]kafka.Message, len(messages))

	for i, message := range messages {
		kafkaMessages[i] = kafka.Message{
			Key:   []byte(key),
			Topic: topicOrQueue,
			Value: message.Body,
		}

		for _, header := range messages[i].Headers {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: header.Key, Value: header.Value})
		}

		for headerKey, headerValue := range headers {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: headerKey, Value: []byte(headerValue)})
		}
	}

	return k.producer.WriteMessages(ctx, kafkaMessages...)
}

func (k *producer) Close() error {
	return k.producer.Close()
}
