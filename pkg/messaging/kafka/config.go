package kafka

import (
	"github.com/IBM/sarama"
)

type (
	KafkaBuilder interface {
		Build() error
		Close() error
		Topic(name string) (string, error)
		DeclareTopics(topics ...*TopicConfig) KafkaBuilder
	}

	kafkaBuilder struct {
		topics          []*TopicConfig
		adminConnection sarama.ClusterAdmin
	}

	TopicConfig struct {
		Topic             string
		NumPartitions     int32
		ReplicationFactor int16
	}
)

func NewKafkaBuilder(client *Client) (KafkaBuilder, error) {
	admin, err := sarama.NewClusterAdminFromClient(client.client)
	if err != nil {
		return nil, err
	}
	return &kafkaBuilder{adminConnection: admin}, nil
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
			err := k.adminConnection.CreateTopic(
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
	metadata, err := k.adminConnection.DescribeTopics([]string{name})
	if err != nil {
		return "", err
	}

	if len(metadata) == 0 || metadata[0].Err != sarama.ErrNoError {
		return "", metadata[0].Err
	}
	return metadata[0].Name, nil
}

func (k *kafkaBuilder) Close() error {
	return k.adminConnection.Close()
}
