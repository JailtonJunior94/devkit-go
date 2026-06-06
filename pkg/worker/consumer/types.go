package consumer

import "context"

type Handler interface {
	Handle(ctx context.Context, msg Message) error
}

type HandlerFunc func(ctx context.Context, msg Message) error

func (f HandlerFunc) Handle(ctx context.Context, msg Message) error {
	return f(ctx, msg)
}

type Message struct {
	EventType string
	Params    map[string]string
	Body      []byte
}

type Source interface {
	Messages(ctx context.Context) (<-chan Message, error)
	Stop(ctx context.Context) error
}

type Runner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
