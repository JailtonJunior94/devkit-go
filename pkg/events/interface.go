package events

import (
	"context"
)

type Event interface {
	GetEventType() string
	GetPayload() any
}

type EventDispatcher interface {
	Register(eventType string, handler EventHandler) error
	Dispatch(ctx context.Context, event Event) error
	Remove(eventType string, handler EventHandler) error
	Has(eventType string, handler EventHandler) bool
	Clear()
}

type EventHandler interface {
	Handle(ctx context.Context, event Event) error
}
