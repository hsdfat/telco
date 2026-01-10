package equeue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ProcessingMode defines how events should be processed
type ProcessingMode int

const (
	// Sequential mode processes events one at a time in order
	Sequential ProcessingMode = iota
	// Parallel mode processes events concurrently
	Parallel
)

// String returns the string representation of ProcessingMode
func (pm ProcessingMode) String() string {
	switch pm {
	case Sequential:
		return "sequential"
	case Parallel:
		return "parallel"
	default:
		return "unknown"
	}
}

// IEventQueue defines the interface for an event queue
type IEventQueue interface {
	// Enqueue adds an event to the queue
	Enqueue(event IEvent) error
	// Start begins processing events from the queue
	Start(ctx context.Context) error
	// Stop gracefully stops the queue processing
	Stop() error
	// RegisterHandler registers a handler for a specific event type
	RegisterHandler(eventType string, handler IEventHandler)
	// GetQueueSize returns the current number of events in the queue
	GetQueueSize() int
}

// EventQueue is the default implementation of IEventQueue
// Uses lock-free design for sequential processing
type EventQueue struct {
	events     chan IEvent
	handlers   map[string]IEventHandler
	mode       atomic.Int32
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	bufferSize int
	running    atomic.Bool
}

// EventQueueConfig holds configuration for creating an event queue
type EventQueueConfig struct {
	BufferSize     int
	ProcessingMode ProcessingMode
}

// NewEventQueue creates a new event queue with the given configuration
func NewEventQueue(config EventQueueConfig) *EventQueue {
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}

	eq := &EventQueue{
		events:     make(chan IEvent, config.BufferSize),
		handlers:   make(map[string]IEventHandler),
		bufferSize: config.BufferSize,
	}
	eq.mode.Store(int32(config.ProcessingMode))
	eq.running.Store(false)

	return eq
}

// NewDefaultEventQueue creates a new event queue with default configuration:
// - BufferSize: 1000
// - ProcessingMode: Sequential
func NewDefaultEventQueue() *EventQueue {
	return NewEventQueue(EventQueueConfig{
		BufferSize:     1000,
		ProcessingMode: Sequential,
	})
}

// Enqueue adds an event to the queue
func (eq *EventQueue) Enqueue(event IEvent) error {
	if !eq.running.Load() {
		return fmt.Errorf("queue is stopped")
	}

	select {
	case eq.events <- event:
		return nil
	case <-eq.ctx.Done():
		return fmt.Errorf("queue context cancelled")
	default:
		return fmt.Errorf("queue is full")
	}
}

// Start begins processing events from the queue
func (eq *EventQueue) Start(ctx context.Context) error {
	if !eq.running.CompareAndSwap(false, true) {
		return fmt.Errorf("queue is already running")
	}

	eq.ctx, eq.cancel = context.WithCancel(ctx)

	eq.wg.Add(1)
	go eq.processEvents()

	return nil
}

// Stop gracefully stops the queue processing
func (eq *EventQueue) Stop() error {
	if !eq.running.CompareAndSwap(true, false) {
		return fmt.Errorf("queue is already stopped")
	}

	if eq.cancel != nil {
		eq.cancel()
	}

	eq.wg.Wait()

	return nil
}

// RegisterHandler registers a handler for a specific event type
// Only one handler per event type is allowed. Registering a new handler will replace the existing one.
// Note: Should be called before Start() to avoid race conditions
func (eq *EventQueue) RegisterHandler(eventType string, handler IEventHandler) {
	eq.handlers[eventType] = handler
}

// GetQueueSize returns the current number of events in the queue
func (eq *EventQueue) GetQueueSize() int {
	return len(eq.events)
}

// processEvents is the main event processing loop
func (eq *EventQueue) processEvents() {
	defer eq.wg.Done()

	for {
		select {
		case event := <-eq.events:
			eq.handleEvent(event)
		case <-eq.ctx.Done():
			// Process remaining events before shutting down
			eq.drainQueue()
			return
		}
	}
}

// handleEvent processes a single event based on the processing mode
func (eq *EventQueue) handleEvent(event IEvent) {

	// Check if event has expired
	if event.IsExpired() {
		event.Done(nil, errors.New("event expired: deadline exceeded"))
		return
	}

	handler, exists := eq.handlers[event.GetType()]
	if !exists {
		event.Done(nil, errors.New("no handler registered for event type"))
		return
	}

	// Log event processing start

	// Call the handler and set result
	err := handler.Handle(event.GetContext(), event)
	if err != nil {
		event.Done(nil, err)
	} else {
		event.Done("processed", nil)
	}
}

// drainQueue processes all remaining events in the queue
func (eq *EventQueue) drainQueue() {
	for {
		select {
		case event := <-eq.events:
			eq.handleEvent(event)
		default:
			return
		}
	}
}
