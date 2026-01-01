// Package events provides a simple event dispatcher implementation
// for publish-subscribe patterns with thread-safe operations.
package events

import (
	"context"
)

// Event represents a domain event with a type identifier and payload.
// Implementations must be safe for concurrent use if shared across goroutines.
type Event interface {
	// GetEventType returns the unique identifier for this event type.
	// The event type is used to match events with registered handlers.
	GetEventType() string

	// GetPayload returns the event data.
	// The concrete type depends on the event type.
	// Consumers should use type assertion to access the payload.
	GetPayload() any
}

// EventDispatcher manages event handlers and dispatches events to them.
// All implementations must be safe for concurrent use by multiple goroutines.
type EventDispatcher interface {
	// Register adds a handler for the specified event type.
	// Returns an error if the handler is nil, already registered, or if eventType is empty.
	// Multiple handlers can be registered for the same event type.
	Register(eventType string, handler EventHandler) error

	// Dispatch sends an event to all registered handlers for the event's type.
	// Handlers are called synchronously in registration order.
	// Returns immediately if the context is cancelled.
	// By default, stops at the first handler error and returns it.
	// Returns nil if no handlers are registered for the event type.
	Dispatch(ctx context.Context, event Event) error

	// Remove unregisters a handler for the specified event type.
	// If the handler is not found, this is a no-op and returns nil.
	// Only the first matching handler is removed if registered multiple times.
	Remove(eventType string, handler EventHandler) error

	// Has checks if a specific handler is registered for an event type.
	// Returns true if the exact handler instance is found.
	Has(eventType string, handler EventHandler) bool

	// Clear removes all registered handlers for all event types.
	// This operation is irreversible.
	Clear()
}

// EventHandler processes events of a specific type.
// Implementations must be safe for concurrent use if shared across goroutines.
type EventHandler interface {
	// Handle processes an event.
	// The context can be used for cancellation, timeouts, and passing request-scoped values.
	// Returns an error if the event cannot be handled successfully.
	Handle(ctx context.Context, event Event) error
}
