package messaging

import "context"

type (
	Publisher interface {
		Publish(ctx context.Context, topicOrQueue, key string, headers map[string]string, message *Message) error
	}

	Message struct {
		Body []byte
	}
)
