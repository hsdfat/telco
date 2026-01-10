package equeue

import "context"

// IEventHandler defines the interface for processing events
type IEventHandler interface {
	// Handle processes the event and returns an error if processing fails
	Handle(ctx context.Context, event IEvent) error
}

// EventHandlerFunc is a function type that implements IEventHandler
type EventHandlerFunc func(ctx context.Context, event IEvent) error

// Handle implements IEventHandler interface
func (f EventHandlerFunc) Handle(ctx context.Context, event IEvent) error {
	return f(ctx, event)
}
