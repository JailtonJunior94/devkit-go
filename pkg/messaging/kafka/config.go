package kafka

import (
	"github.com/segmentio/kafka-go"
)

type (
	KafkaBuilder struct {
		conn   *kafka.Conn
		topics []*TopicConfig
	}

	TopicConfig struct {
		Topic             string
		NumPartitions     int
		ReplicationFactor int
	}
)

func NewKafkaBuilder(conn *kafka.Conn) *KafkaBuilder {
	return &KafkaBuilder{conn: conn}
}

func NewTopicConfig(topic string, numPartitions, replicationFactor int) *TopicConfig {
	return &TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	}
}

func (k *KafkaBuilder) DeclareTopics(topics ...*TopicConfig) *KafkaBuilder {
	k.topics = topics
	return k
}

func (k *KafkaBuilder) Build() error {
	if len(k.topics) > 0 {
		for _, topic := range k.topics {
			if err := k.conn.CreateTopics(kafka.TopicConfig{
				Topic:             topic.Topic,
				NumPartitions:     topic.NumPartitions,
				ReplicationFactor: topic.ReplicationFactor,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
