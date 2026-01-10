package equeue

import (
	"context"
	"sync/atomic"
	"time"
)

var eventIDCounter uint64

// IEvent represents a basic event interface that can be queued and processed
type IEvent interface {
	// GetID returns the unique identifier of the event
	GetID() uint64
	// GetType returns the type/category of the event
	GetType() string
	// GetContext returns the event context
	GetContext() context.Context
	// GetTimestamp returns when the event was created
	GetTimestamp() time.Time
	// GetDeadline returns the deadline for processing this event
	// Returns zero time if no deadline is set
	GetDeadline() time.Time
	// HasDeadline returns true if the event has a deadline
	HasDeadline() bool
	// IsExpired checks if the event has passed its deadline
	IsExpired() bool
	// Done signals that the event processing is complete and sets the result
	Done(result interface{}, err error)
	// Wait waits for the event to be processed and returns the result
	Wait() (interface{}, error)
}

// EventContext holds the context data and completion channel for an event
type EventContext struct {
	ctx        context.Context
	resultChan chan eventResult
	done       atomic.Bool
}

type eventResult struct {
	result interface{}
	err    error
}

// NewEventContext creates a new event context
func NewEventContext(ctx context.Context) *EventContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &EventContext{
		ctx:        ctx,
		resultChan: make(chan eventResult, 1),
	}
}

// Context returns the underlying context
func (ec *EventContext) Context() context.Context {
	return ec.ctx
}

// Done signals completion with result
func (ec *EventContext) Done(result interface{}, err error) {
	if ec.done.CompareAndSwap(false, true) {
		ec.resultChan <- eventResult{result: result, err: err}
		close(ec.resultChan)
	}
}

// Wait waits for completion and returns the result
func (ec *EventContext) Wait() (interface{}, error) {
	result := <-ec.resultChan
	return result.result, result.err
}

// Event is the default implementation of IEvent
type Event struct {
	id        uint64
	eventType string
	eventCtx  *EventContext
	timestamp time.Time
	deadline  time.Time
}

// EventOption is a function that configures an Event
type EventOption func(*Event)

// WithDeadline sets a deadline for the event
func WithDeadline(deadline time.Time) EventOption {
	return func(e *Event) {
		e.deadline = deadline
	}
}

// WithTimeout sets a timeout duration for the event from creation time
func WithTimeout(timeout time.Duration) EventOption {
	return func(e *Event) {
		e.deadline = e.timestamp.Add(timeout)
	}
}

// NewEvent creates a new event instance with auto-incrementing ID
func NewEvent(eventType string, ctx context.Context, options ...EventOption) *Event {
	event := &Event{
		id:        atomic.AddUint64(&eventIDCounter, 1),
		eventType: eventType,
		eventCtx:  NewEventContext(ctx),
		timestamp: time.Now(),
	}

	for _, option := range options {
		option(event)
	}

	return event
}

// GetID returns the unique identifier of the event
func (e *Event) GetID() uint64 {
	return e.id
}

// GetType returns the type/category of the event
func (e *Event) GetType() string {
	return e.eventType
}

// GetContext returns the event context
func (e *Event) GetContext() context.Context {
	return e.eventCtx.Context()
}

// GetTimestamp returns when the event was created
func (e *Event) GetTimestamp() time.Time {
	return e.timestamp
}

// GetDeadline returns the deadline for processing this event
func (e *Event) GetDeadline() time.Time {
	return e.deadline
}

// HasDeadline returns true if the event has a deadline
func (e *Event) HasDeadline() bool {
	return !e.deadline.IsZero()
}

// IsExpired checks if the event has passed its deadline
func (e *Event) IsExpired() bool {
	if !e.HasDeadline() {
		return false
	}
	return time.Now().After(e.deadline)
}

// Done signals that the event processing is complete
func (e *Event) Done(result interface{}, err error) {
	e.eventCtx.Done(result, err)
}

// Wait waits for the event to be processed and returns the result
func (e *Event) Wait() (interface{}, error) {
	return e.eventCtx.Wait()
}
