package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNilHandler         = errors.New("worker: nil handler")
	ErrDuplicateEventType = errors.New("worker: duplicate event type")
	ErrUnknownEventType   = errors.New("worker: unknown event type")
)

type registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func newRegistry() *registry {
	return &registry{
		handlers: make(map[string]Handler),
	}
}

func (r *registry) register(eventType string, h Handler) error {
	if h == nil {
		return ErrNilHandler
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[eventType]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateEventType, eventType)
	}

	r.handlers[eventType] = h
	return nil
}

func (r *registry) dispatch(ctx context.Context, msg Message) error {
	r.mu.RLock()
	h, ok := r.handlers[msg.EventType]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownEventType, msg.EventType)
	}

	return h.Handle(ctx, msg)
}
