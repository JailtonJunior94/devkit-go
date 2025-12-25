package rabbitmq

import (
	"context"
	"sync"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	rabbitMQ struct {
		channel *amqp.Channel
		mu      sync.Mutex
	}
)

func NewRabbitMQPublisher(channel *amqp.Channel) messaging.Publisher {
	return &rabbitMQ{channel: channel}
}

func (r *rabbitMQ) Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *messaging.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := amqp.Publishing{
		Body:        message.Body,
		ContentType: headers["content_type"],
		Headers:     amqp.Table{},
	}

	for k, v := range headers {
		msg.Headers[k] = v
	}
	return r.channel.PublishWithContext(ctx, topicOrQueue, key, false, false, msg)
}

func (r *rabbitMQ) PublishBatch(ctx context.Context, topicOrQueue, key string, headers map[string]string, messages []*messaging.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, message := range messages {
		msg := amqp.Publishing{
			Body:        message.Body,
			ContentType: headers["content_type"],
			Headers:     amqp.Table{},
		}

		// Add common headers
		for k, v := range headers {
			msg.Headers[k] = v
		}

		// Add message-specific headers
		for _, header := range message.Headers {
			msg.Headers[header.Key] = string(header.Value)
		}

		if err := r.channel.PublishWithContext(ctx, topicOrQueue, key, false, false, msg); err != nil {
			return err
		}
	}

	return nil
}

func (r *rabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.channel.Close()
}
