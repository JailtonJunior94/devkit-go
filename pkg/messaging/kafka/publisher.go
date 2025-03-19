package kafka

import (
	"context"
	"errors"
	"log"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/IBM/sarama"
)

type publisher struct {
	producer sarama.SyncProducer
}

func NewPublisher(client *Client) (messaging.Publisher, error) {
	producer, err := sarama.NewSyncProducerFromClient(client.client)
	if err != nil {
		return nil, err
	}
	return &publisher{producer: producer}, nil
}

func (k *publisher) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	msg := &sarama.ProducerMessage{
		Topic: topicOrQueue,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(message.Body),
	}

	for key, value := range headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
	}

	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		return err
	}

	log.Printf("Message sent to partition %d at offset %d\n", partition, offset)
	return nil
}

func (k *publisher) PublishBatch(ctx context.Context, topic, key string, headers map[string]string, messages []*messaging.Message) error {
	return errors.New("not implemented")
}

func (k *publisher) Close() error {
	return k.producer.Close()
}
