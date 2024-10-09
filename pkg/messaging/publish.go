package messaging

import "context"

type (
	Publish interface {
		Produce(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *Message) error
	}

	Message struct {
		Body []byte
	}
)
