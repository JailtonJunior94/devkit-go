package kafka

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
)

const (
	LastOffset  int64 = -1
	FirstOffset int64 = -2
)

type (
	Options func(reader *reader)
	reader  struct {
		retries         int
		maxRetries      int
		offset          int64
		enableDLT       bool
		topicName       string
		consumerGroupID string
		topicNameDLT    string
		kafkaReader     *kafka.Reader
		backoff         backoff.BackOff
		retryChan       chan kafka.Message
		publisher       messaging.Publisher
		handlers        map[string][]messaging.ConsumeHandler
	}
)

func (b *broker) NewConsumerFromBroker(options ...Options) (messaging.Consumer, error) {
	consumer := &reader{
		handlers: make(map[string][]messaging.ConsumeHandler),
	}

	for _, option := range options {
		option(consumer)
	}

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        b.brokers,
		Dialer:         b.dialer,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		StartOffset:    consumer.offset,
		Topic:          consumer.topicName,
		GroupID:        consumer.consumerGroupID,
		CommitInterval: 0, // Disable auto commit
	})

	consumer.publisher, _ = b.NewProducerFromBroker()
	consumer.kafkaReader = kafkaReader
	return consumer, nil
}

func (k *reader) Consume(ctx context.Context) error {
	go func() {
		for {
			msg, err := k.kafkaReader.ReadMessage(ctx)
			if err != nil {
				log.Fatalf("failed to read message: %v", err)
				continue
			}

			if handlers, ok := k.handlers[k.extractHeader(msg)["event_type"]]; ok {
				for _, handler := range handlers {
					if err := k.dispatcher(ctx, msg, handler); err != nil {
						log.Fatalf("failed to dispatch message: %v", err)
						continue
					}
				}
			}
		}
	}()
	return nil
}

func (k *reader) ConsumeBatch(ctx context.Context) error {
	go func() {
		for {
			message, err := k.kafkaReader.FetchMessage(ctx)
			if err != nil {
				if err == io.EOF {
					continue
				}
				log.Fatalf("failed to fetch messages: %v", err)
				continue
			}

			if handlers, ok := k.handlers[k.extractHeader(message)["event_type"]]; ok {
				for _, handler := range handlers {
					if err := k.dispatcher(ctx, message, handler); err != nil {
						log.Fatalf("failed to dispatch message: %v", err)
						continue
					}
				}
			}
		}
	}()
	return nil
}

func (k *reader) Close() error {
	return k.kafkaReader.Close()
}

func (k *reader) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	k.handlers[eventType] = append(k.handlers[eventType], handler)
}

func (k *reader) extractHeader(message kafka.Message) map[string]string {
	headers := make(map[string]string)
	for _, header := range message.Headers {
		headers[string(header.Key)] = string(header.Value)
	}
	return headers
}

func (k *reader) dispatcher(ctx context.Context, message kafka.Message, handler messaging.ConsumeHandler) error {
	err := handler(ctx, k.extractHeader(message), message.Value)
	if err != nil {
		k.retries++
		return k.retry(ctx, message, err)
	}

	if err := k.kafkaReader.CommitMessages(ctx, message); err != nil {
		return err
	}
	return nil
}

func (k *reader) retry(ctx context.Context, message kafka.Message, err error) error {
	k.retryChan <- message
	go func() {
		for msg := range k.retryChan {
			if k.retries >= k.maxRetries {
				if err := k.moveToDLT(ctx, msg, err); err != nil {
					log.Fatalf("failed to move message to dlt: %v", err)
				}
				break
			}

			if handlers, ok := k.handlers[k.extractHeader(message)["event_type"]]; ok {
				for _, handler := range handlers {
					if err := k.dispatcher(ctx, message, handler); err != nil {
						time.Sleep(k.backoff.NextBackOff())
						continue
					}
				}
			}
		}
	}()
	return nil
}

func (k *reader) moveToDLT(ctx context.Context, message kafka.Message, err error) error {
	if !k.enableDLT {
		return nil
	}

	headers := map[string]string{
		"error":      err.Error(),
		"attempts":   string(k.retries),
		"event_type": k.extractHeader(message)["event_type"],
	}

	return k.publisher.Publish(ctx, k.topicNameDLT, string(message.Key), headers, &messaging.Message{Body: message.Value})
}

func WithRetry(sizeChan int) Options {
	return func(reader *reader) {
		reader.retryChan = make(chan kafka.Message, sizeChan)
	}
}

func WithMaxRetries(maxRetries int) Options {
	return func(reader *reader) {
		reader.maxRetries = maxRetries
	}
}

func WithBackoff(backoff backoff.BackOff) Options {
	return func(reader *reader) {
		reader.backoff = backoff
	}
}

func WithTopicName(topic string) Options {
	return func(reader *reader) {
		reader.topicName = topic
	}
}

func WithOffset(offset int64) Options {
	return func(reader *reader) {
		reader.offset = offset
	}
}

func WithTopicNameDLT(topicDLT string) Options {
	return func(reader *reader) {
		reader.enableDLT = true
		reader.topicNameDLT = topicDLT
	}
}

func WithConsumerGroupID(groupID string) Options {
	return func(reader *reader) {
		reader.consumerGroupID = groupID
	}
}
