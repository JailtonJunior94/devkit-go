package messaging

import "context"

type (
	ConsumeHandler func(ctx context.Context, params map[string]string, body []byte) error
	Consumer       interface {
		Close() error
		Consume(ctx context.Context) error
		ConsumeBatch(ctx context.Context) error
		RegisterHandler(eventType string, handler ConsumeHandler)
		ConsumeWithWorkerPool(ctx context.Context, workerCount int) error
	}
)
