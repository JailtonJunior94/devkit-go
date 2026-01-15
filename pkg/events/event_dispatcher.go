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

// DispatcherOption configures an EventDispatcher.
type DispatcherOption func(*eventDispatcher)

// WithCapacity pre-allocates capacity for the internal event type map.
// Use this when you know approximately how many event types will be registered
// to avoid map reallocations.
func WithCapacity(capacity int) DispatcherOption {
	return func(ed *eventDispatcher) {
		ed.handlers = make(map[string][]EventHandler, capacity)
	}
}

// NewEventDispatcher creates a new EventDispatcher with optional configuration.
// Without options, creates a dispatcher with default settings.
//
// Example:
//
//	dispatcher := NewEventDispatcher()  // default
//	dispatcher := NewEventDispatcher(WithCapacity(50))  // pre-allocated capacity
func NewEventDispatcher(opts ...DispatcherOption) EventDispatcher {
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

	// Acquire read lock to copy handlers
	ed.mu.RLock()
	handlers, ok := ed.handlers[eventType]
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

func (ed *eventDispatcher) Register(eventType string, handler EventHandler) error {
	if eventType == "" {
		return ErrEventTypeEmpty
	}
	if handler == nil {
		return ErrHandlerNil
	}

	ed.mu.Lock()
	defer ed.mu.Unlock()

	// Check if handler is already registered
	// Note: O(n) check, acceptable for typical handler counts (<10)
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

	handlers, ok := ed.handlers[eventType]
	if !ok {
		return false
	}

	return slices.Contains(handlers, handler)
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

	// First check if handler exists (O(n) but avoids allocation)
	found := false
	for _, h := range handlers {
		if h == handler {
			found = true
			break
		}
	}

	// Early return if handler not found
	if !found {
		return nil
	}

	// Handler exists, create new slice
	newHandlers := make([]EventHandler, 0, len(handlers)-1)
	removed := false
	for _, h := range handlers {
		// Remove only the first matching handler
		if h == handler && !removed {
			removed = true
			continue
		}
		newHandlers = append(newHandlers, h)
	}

	// If all handlers were removed, delete the key
	if len(newHandlers) == 0 {
		delete(ed.handlers, eventType)
		return nil
	}

	ed.handlers[eventType] = newHandlers
	return nil
}

func (ed *eventDispatcher) Clear() {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	clear(ed.handlers)
}
