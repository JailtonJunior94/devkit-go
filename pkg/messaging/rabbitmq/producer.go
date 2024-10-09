package rabbitmq

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	rabbitMQ struct {
		channel *amqp.Channel
	}
)

func NewRabbitMQ(channel *amqp.Channel) messaging.Publish {
	return &rabbitMQ{channel: channel}
}

func (r *rabbitMQ) Produce(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	msg := amqp.Publishing{
		Body:        message.Body,
		ContentType: headers["content_type"],
		Headers:     amqp.Table{},
	}

	for key, value := range headers {
		msg.Headers[key] = value
	}
	return r.channel.PublishWithContext(ctx, topicOrQueue, key, false, false, msg)
}
