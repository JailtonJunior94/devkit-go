package kafka

import (
	"github.com/IBM/sarama"
)

type (
	KafkaBuilder interface {
		DeclareTopics(topics ...*TopicConfig) KafkaBuilder
		Build() error
		Topic(name string) (string, error)
		Close() error
	}

	kafkaBuilder struct {
		conn   sarama.ClusterAdmin
		topics []*TopicConfig
	}

	TopicConfig struct {
		Topic             string
		NumPartitions     int32
		ReplicationFactor int16
	}
)

func NewKafkaBuilder(brokers []string) (KafkaBuilder, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Errors = true
	config.Version = sarama.V3_6_0_0

	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return nil, err
	}
	return &kafkaBuilder{conn: admin}, nil
}

func NewTopicConfig(topic string, numPartitions int32, replicationFactor int16) *TopicConfig {
	return &TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	}
}

func (k *kafkaBuilder) DeclareTopics(topics ...*TopicConfig) KafkaBuilder {
	k.topics = topics
	return k
}

func (k *kafkaBuilder) Build() error {
	if len(k.topics) > 0 {
		for _, topic := range k.topics {
			err := k.conn.CreateTopic(
				topic.Topic,
				&sarama.TopicDetail{
					NumPartitions:     int32(topic.NumPartitions),
					ReplicationFactor: int16(topic.ReplicationFactor),
				},
				false,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *kafkaBuilder) Topic(name string) (string, error) {
	metadata, err := k.conn.DescribeTopics([]string{name})
	if err != nil {
		return "", err
	}

	if len(metadata) <= 0 || metadata[0].Err != sarama.ErrNoError {
		return "", metadata[0].Err
	}
	return metadata[0].Name, nil
}

func (k *kafkaBuilder) Close() error {
	return k.conn.Close()
}
