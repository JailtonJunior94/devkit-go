package events

import (
	"context"
	"errors"
	"slices"
	"sync"
)

var (
	ErrHandlerAlreadyRegistered = errors.New("handler already registered")
	ErrEventNil                 = errors.New("event cannot be nil")
	ErrHandlerNil               = errors.New("handler cannot be nil")
	ErrEventTypeEmpty           = errors.New("event type cannot be empty")
)

type eventDispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

type dispatcherOption func(*eventDispatcher)

func WithCapacity(capacity int) dispatcherOption {
	return func(ed *eventDispatcher) {
		if capacity < 0 {
			capacity = 0
		}
		ed.handlers = make(map[string][]EventHandler, capacity)
	}
}

func NewEventDispatcher(opts ...dispatcherOption) EventDispatcher {
	ed := &eventDispatcher{
		handlers: make(map[string][]EventHandler),
	}
	for _, opt := range opts {
		opt(ed)
	}
	return ed
}

func (ed *eventDispatcher) Dispatch(ctx context.Context, event Event) error {
	if event == nil {
		return ErrEventNil
	}

	eventType := event.GetEventType()
	if eventType == "" {
		return ErrEventTypeEmpty
	}

	ed.mu.RLock()
	handlers, ok := ed.handlers[eventType]
	if !ok {
		ed.mu.RUnlock()
		return nil
	}

	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	ed.mu.RUnlock()

	for _, handler := range handlersCopy {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := handler.Handle(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (ed *eventDispatcher) Register(eventType string, handler EventHandler) error {
	if eventType == "" {
		return ErrEventTypeEmpty
	}
	if handler == nil {
		return ErrHandlerNil
	}

	ed.mu.Lock()
	defer ed.mu.Unlock()

	if slices.Contains(ed.handlers[eventType], handler) {
		return ErrHandlerAlreadyRegistered
	}

	ed.handlers[eventType] = append(ed.handlers[eventType], handler)
	return nil
}

func (ed *eventDispatcher) Has(eventType string, handler EventHandler) bool {
	if eventType == "" || handler == nil {
		return false
	}

	ed.mu.RLock()
	defer ed.mu.RUnlock()

	return slices.Contains(ed.handlers[eventType], handler)
}

func (ed *eventDispatcher) Remove(eventType string, handler EventHandler) error {
	if eventType == "" || handler == nil {
		return nil
	}

	ed.mu.Lock()
	defer ed.mu.Unlock()

	handlers, ok := ed.handlers[eventType]
	if !ok {
		return nil
	}

	idx := slices.Index(handlers, handler)
	if idx < 0 {
		return nil
	}

	updated := slices.Delete(handlers, idx, idx+1)
	if len(updated) == 0 {
		delete(ed.handlers, eventType)
		return nil
	}

	ed.handlers[eventType] = updated
	return nil
}

func (ed *eventDispatcher) Clear() {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	clear(ed.handlers)
}
