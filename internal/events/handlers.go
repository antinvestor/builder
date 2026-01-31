package events

import "context"

// EventHandler defines the interface for handling events.
type EventHandler interface {
	// Handle handles an event.
	Handle(ctx context.Context, event *Event) error
}

// EventHandlerFunc is a function adapter for EventHandler.
type EventHandlerFunc func(ctx context.Context, event *Event) error

// Handle implements EventHandler.
func (f EventHandlerFunc) Handle(ctx context.Context, event *Event) error {
	return f(ctx, event)
}
