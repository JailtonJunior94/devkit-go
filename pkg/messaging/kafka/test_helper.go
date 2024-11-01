package kafka

import (
	"context"
	"testing"

	kafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

type KafkaContainer struct {
	Brokers   []string
	container *kafka.KafkaContainer
}

func SetupKafka(t testing.TB) *KafkaContainer {
	t.Helper()
	ctx := context.Background()

	kafkaContainer, err := kafka.Run(
		ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		t.Fatal(err)
	}

	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return &KafkaContainer{
		Brokers:   brokers,
		container: kafkaContainer,
	}
}

func (r *KafkaContainer) Teardown(t testing.TB) {
	if err := r.container.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
