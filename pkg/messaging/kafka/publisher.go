package kafka

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/IBM/sarama"
)

type (
	kafkaPublisher struct {
		producer sarama.SyncProducer
	}
)

func NewKafkaPublisher(brokers []string) (messaging.Publisher, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_6_0_0
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return &kafkaPublisher{producer: producer}, nil
}

func (k *kafkaPublisher) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	msg := &sarama.ProducerMessage{
		Topic: topicOrQueue,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(message.Body),
	}

	for key, value := range headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
	}

	_, _, err := k.producer.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}
