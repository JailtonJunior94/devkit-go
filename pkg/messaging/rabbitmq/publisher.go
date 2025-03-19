package rabbitmq

import (
	"context"
	"errors"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	rabbitMQ struct {
		channel *amqp.Channel
	}
)

func NewRabbitMQPublisher(channel *amqp.Channel) messaging.Publisher {
	return &rabbitMQ{channel: channel}
}

func (r *rabbitMQ) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
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

func (k *rabbitMQ) PublishBatch(ctx context.Context, topic, key string, headers map[string]string, messages []*messaging.Message) error {
	return errors.New("not implemented")
}

func (r *rabbitMQ) Close() error {
	return r.channel.Close()
}
