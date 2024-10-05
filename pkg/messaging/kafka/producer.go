package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type (
	KafkaClient interface {
		Produce(ctx context.Context, topic string, headers map[string]string, message *Message) error
	}

	kafkaClient struct {
		client *kafka.Writer
	}

	Message struct {
		Key   []byte
		Value []byte
	}
)

func NewKafkaClient(broker string) KafkaClient {
	client := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Balancer: &kafka.LeastBytes{},
	}
	return &kafkaClient{client: client}
}

func (k *kafkaClient) Produce(ctx context.Context, topic string, headers map[string]string, message *Message) error {
	messageKafka := kafka.Message{
		Topic: topic,
		Key:   message.Key,
		Value: message.Value,
	}

	for key, value := range headers {
		messageKafka.Headers = append(messageKafka.Headers, kafka.Header{Key: key, Value: []byte(value)})
	}

	if err := k.client.WriteMessages(ctx, messageKafka); err != nil {
		return err
	}
	return nil
}
