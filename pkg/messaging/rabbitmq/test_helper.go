package rabbitmq

import (
	"context"
	"testing"

	rabbit "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

type RabbitMQContainer struct {
	container *rabbit.RabbitMQContainer
	URL       string
}

func SetupRabbitMQ(t testing.TB) *RabbitMQContainer {
	t.Helper()
	ctx := context.Background()

	rabbitmqContainer, err := rabbit.Run(ctx, "rabbitmq:4.0.2-management-alpine")
	if err != nil {
		t.Fatal(err)
	}

	url, err := rabbitmqContainer.AmqpURL(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return &RabbitMQContainer{
		URL:       url,
		container: rabbitmqContainer,
	}
}

func (r *RabbitMQContainer) Teardown(t testing.TB) {
	if err := r.container.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
