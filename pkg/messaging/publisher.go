package messaging

import "context"

type (
	Publisher interface {
		Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *Message) error
		PublishBatch(ctx context.Context, topicOrQueue, key string, headers map[string]string, messages []*Message) error
		Close() error
	}

	Message struct {
		Body    []byte
		Headers []Header
	}

	Header struct {
		Key   string
		Value []byte
	}
)
