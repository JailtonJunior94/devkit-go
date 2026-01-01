package events

import (
	"context"
	"errors"
	"slices"
	"sync"
)

var (
	// ErrHandlerAlreadyRegistered is returned when attempting to register a handler that is already registered.
	ErrHandlerAlreadyRegistered = errors.New("handler already registered")

	// ErrEventNil is returned when a nil event is passed to Dispatch.
	ErrEventNil = errors.New("event cannot be nil")

	// ErrHandlerNil is returned when a nil handler is passed to Register.
	ErrHandlerNil = errors.New("handler cannot be nil")

	// ErrEventTypeEmpty is returned when an empty event type is passed to Register.
	ErrEventTypeEmpty = errors.New("event type cannot be empty")
)

type eventDispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

func NewEventDispatcher() EventDispatcher {
	return &eventDispatcher{
		handlers: make(map[string][]EventHandler),
	}
}

func (ed *eventDispatcher) Dispatch(ctx context.Context, event Event) error {
	if event == nil {
		return ErrEventNil
	}

	// Acquire read lock to copy handlers
	ed.mu.RLock()
	handlers, ok := ed.handlers[event.GetEventType()]
	if !ok {
		ed.mu.RUnlock()
		return nil
	}

	// Create a copy to avoid holding the lock during handler execution
	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	ed.mu.RUnlock()

	// Execute handlers without holding the lock
	for _, handler := range handlersCopy {
		// Check for context cancellation before each handler
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

func (ed *eventDispatcher) Register(eventName string, handler EventHandler) error {
	if eventName == "" {
		return ErrEventTypeEmpty
	}
	if handler == nil {
		return ErrHandlerNil
	}

	ed.mu.Lock()
	defer ed.mu.Unlock()

	// Check if handler is already registered
	if slices.Contains(ed.handlers[eventName], handler) {
		return ErrHandlerAlreadyRegistered
	}

	ed.handlers[eventName] = append(ed.handlers[eventName], handler)
	return nil
}

func (ed *eventDispatcher) Has(eventName string, handler EventHandler) bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	handlers, ok := ed.handlers[eventName]
	if !ok {
		return false
	}

	return slices.Contains(handlers, handler)
}

func (ed *eventDispatcher) Remove(eventName string, handler EventHandler) error {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	handlers, ok := ed.handlers[eventName]
	if !ok {
		return nil
	}

	// Create new slice to avoid modifying the underlying array
	newHandlers := make([]EventHandler, 0, len(handlers))
	found := false
	for _, h := range handlers {
		// Remove only the first matching handler
		if h == handler && !found {
			found = true
			continue
		}
		newHandlers = append(newHandlers, h)
	}

	// If all handlers were removed, delete the key
	if len(newHandlers) == 0 {
		delete(ed.handlers, eventName)
		return nil
	}

	ed.handlers[eventName] = newHandlers
	return nil
}

func (ed *eventDispatcher) Clear() {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.handlers = make(map[string][]EventHandler)
}
