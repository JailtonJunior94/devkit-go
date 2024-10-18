package messaging

import "context"

type (
	ConsumeHandler func(ctx context.Context, params map[string]string, body []byte) error

	Consumer interface {
		Consume(ctx context.Context) error
		RegisterHandler(eventType string, handler ConsumeHandler)
	}
)
