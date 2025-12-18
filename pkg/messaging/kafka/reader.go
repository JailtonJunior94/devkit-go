package kafka

import (
	"context"
	"errors"
	"io"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
)

const (
	LastOffset  int64 = -1
	FirstOffset int64 = -2
)

var (
	ErrConsumerClosed = errors.New("consumer is closed")
	ErrNoHandler      = errors.New("no handler found for event type")
)

type (
	Options func(reader *reader)
	reader  struct {
		maxRetries      int
		offset          int64
		enableDLT       bool
		topicName       string
		consumerGroupID string
		topicNameDLT    string
		kafkaReader     *kafka.Reader
		backoff         backoff.BackOff
		retryChan       chan retryMessage
		publisher       messaging.Publisher
		handlers        map[string][]messaging.ConsumeHandler
		errorChan       chan error
		closed          atomic.Bool
		retryOnce       sync.Once
		closeOnce       sync.Once
		mu              sync.RWMutex
	}

	retryMessage struct {
		message  kafka.Message
		err      error
		attempts int
	}
)

func (b *broker) NewConsumerFromBroker(options ...Options) (messaging.Consumer, error) {
	consumer := &reader{
		handlers:  make(map[string][]messaging.ConsumeHandler),
		errorChan: make(chan error, 100),
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

	producer, err := b.NewProducerFromBroker()
	if err != nil {
		return nil, err
	}

	consumer.publisher = producer
	consumer.kafkaReader = kafkaReader
	return consumer, nil
}

func (k *reader) Consume(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if k.closed.Load() {
					return
				}

				msg, err := k.kafkaReader.ReadMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
						return
					}
					log.Printf("failed to read message: %v", err)
					k.sendError(err)
					continue
				}

				k.processMessage(ctx, msg)
			}
		}
	}()
	return nil
}

func (k *reader) ConsumeBatch(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if k.closed.Load() {
					return
				}

				message, err := k.kafkaReader.FetchMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					if errors.Is(err, io.EOF) {
						continue
					}
					log.Printf("failed to fetch message: %v", err)
					k.sendError(err)
					continue
				}

				k.processMessage(ctx, message)
			}
		}
	}()
	return nil
}

func (k *reader) ConsumeWithWorkerPool(ctx context.Context, workerCount int) error {
	messageChan := make(chan kafka.Message, workerCount*2)

	for i := 0; i < workerCount; i++ {
		go k.worker(ctx, messageChan)
	}

	go func() {
		defer close(messageChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if k.closed.Load() {
					return
				}

				msg, err := k.kafkaReader.ReadMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
						return
					}
					log.Printf("failed to read message: %v", err)
					k.sendError(err)
					continue
				}

				select {
				case messageChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

func (k *reader) Close() error {
	var closeErr error
	k.closeOnce.Do(func() {
		k.closed.Store(true)

		if k.retryChan != nil {
			close(k.retryChan)
		}

		close(k.errorChan)

		if k.publisher != nil {
			k.publisher.Close()
		}

		closeErr = k.kafkaReader.Close()
	})
	return closeErr
}

func (k *reader) Errors() <-chan error {
	return k.errorChan
}

func (k *reader) worker(ctx context.Context, messageChan <-chan kafka.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messageChan:
			if !ok {
				return
			}
			k.processMessage(ctx, message)
		}
	}
}

func (k *reader) processMessage(ctx context.Context, message kafka.Message) {
	eventType := k.extractHeader(message)["event_type"]

	k.mu.RLock()
	handlers, ok := k.handlers[eventType]
	k.mu.RUnlock()

	if !ok {
		log.Printf("no handler found for event type: %s", eventType)
		return
	}

	for _, handler := range handlers {
		if err := k.dispatcher(ctx, message, handler); err != nil {
			log.Printf("failed to dispatch message: %v", err)
		}
	}
}

func (k *reader) RegisterHandler(eventType string, handler messaging.ConsumeHandler) {
	k.mu.Lock()
	defer k.mu.Unlock()
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
		return k.handleError(ctx, message, err)
	}

	if err := k.kafkaReader.CommitMessages(ctx, message); err != nil {
		return err
	}
	return nil
}

func (k *reader) handleError(ctx context.Context, message kafka.Message, handlerErr error) error {
	return k.handleErrorWithAttempts(ctx, message, handlerErr, 1)
}

func (k *reader) handleErrorWithAttempts(ctx context.Context, message kafka.Message, handlerErr error, attempts int) error {
	if attempts >= k.maxRetries {
		if err := k.moveToDLT(ctx, message, handlerErr, attempts); err != nil {
			log.Printf("failed to move message to DLT: %v", err)
			return err
		}
		return handlerErr
	}

	if k.retryChan != nil {
		k.startRetryWorker(ctx)

		select {
		case k.retryChan <- retryMessage{message: message, err: handlerErr, attempts: attempts}:
		default:
			log.Printf("retry channel full, moving to DLT")
			if err := k.moveToDLT(ctx, message, handlerErr, attempts); err != nil {
				log.Printf("failed to move message to DLT: %v", err)
			}
		}
	}

	return handlerErr
}

func (k *reader) startRetryWorker(ctx context.Context) {
	k.retryOnce.Do(func() {
		go k.retryWorker(ctx)
	})
}

func (k *reader) retryWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case retryMsg, ok := <-k.retryChan:
			if !ok {
				return
			}

			if k.backoff != nil {
				delay := k.backoff.NextBackOff()
				if delay == backoff.Stop {
					k.backoff.Reset()
					if err := k.moveToDLT(ctx, retryMsg.message, retryMsg.err, retryMsg.attempts); err != nil {
						log.Printf("failed to move message to DLT: %v", err)
					}
					continue
				}
				time.Sleep(delay)
			}

			eventType := k.extractHeader(retryMsg.message)["event_type"]
			k.mu.RLock()
			handlers, ok := k.handlers[eventType]
			k.mu.RUnlock()

			if !ok {
				continue
			}

			for _, handler := range handlers {
				if err := handler(ctx, k.extractHeader(retryMsg.message), retryMsg.message.Value); err != nil {
					nextAttempt := retryMsg.attempts + 1
					if nextAttempt >= k.maxRetries {
						if err := k.moveToDLT(ctx, retryMsg.message, err, nextAttempt); err != nil {
							log.Printf("failed to move message to DLT: %v", err)
						}
					} else {
						select {
						case k.retryChan <- retryMessage{message: retryMsg.message, err: err, attempts: nextAttempt}:
						default:
							if err := k.moveToDLT(ctx, retryMsg.message, err, nextAttempt); err != nil {
								log.Printf("failed to move message to DLT: %v", err)
							}
						}
					}
					continue
				}

				if k.backoff != nil {
					k.backoff.Reset()
				}

				if err := k.kafkaReader.CommitMessages(ctx, retryMsg.message); err != nil {
					log.Printf("failed to commit message after retry: %v", err)
				}
			}
		}
	}
}

func (k *reader) moveToDLT(ctx context.Context, message kafka.Message, err error, attempts int) error {
	if !k.enableDLT || k.publisher == nil {
		return nil
	}

	headers := map[string]string{
		"error":      err.Error(),
		"attempts":   strconv.Itoa(attempts),
		"event_type": k.extractHeader(message)["event_type"],
	}

	return k.publisher.Publish(ctx, k.topicNameDLT, string(message.Key), headers, &messaging.Message{Body: message.Value})
}

func (k *reader) sendError(err error) {
	select {
	case k.errorChan <- err:
	default:
	}
}

func WithRetry(sizeChan int) Options {
	return func(reader *reader) {
		reader.retryChan = make(chan retryMessage, sizeChan)
	}
}

func WithMaxRetries(maxRetries int) Options {
	return func(reader *reader) {
		reader.maxRetries = maxRetries
	}
}

func WithBackoff(b backoff.BackOff) Options {
	return func(reader *reader) {
		reader.backoff = b
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
