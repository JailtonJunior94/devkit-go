package kafka

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/segmentio/kafka-go"
)

type (
	kafkaClient struct {
		client *kafka.Writer
	}
)

func NewKafkaClient(broker string) messaging.Publish {
	client := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Balancer: &kafka.LeastBytes{},
	}
	return &kafkaClient{client: client}
}

func (k *kafkaClient) Produce(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	messageKafka := kafka.Message{
		Topic: topicOrQueue,
		Key:   []byte(key),
		Value: message.Body,
	}

	for key, value := range headers {
		messageKafka.Headers = append(messageKafka.Headers, kafka.Header{Key: key, Value: []byte(value)})
	}

	if err := k.client.WriteMessages(ctx, messageKafka); err != nil {
		return err
	}
	return nil
}
