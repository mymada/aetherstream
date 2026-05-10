package plugin

import (
	"context"
	"fmt"
	"sync"
)

// EventBus is a simple in-memory pub/sub system for plugin events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
	// wildcard subscribers receive every event regardless of type
	wildcard []chan Event
	closed   bool
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]chan Event),
	}
}

// Subscribe registers a channel to receive events of a specific type.
// The returned function can be called to unsubscribe.
func (eb *EventBus) Subscribe(eventType EventType, ch chan Event) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return func() {}
	}

	if eventType == "" {
		eb.wildcard = append(eb.wildcard, ch)
	} else {
		eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	}

	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		if eb.closed {
			return
		}
		if eventType == "" {
			eb.wildcard = removeChan(eb.wildcard, ch)
		} else {
			eb.subscribers[eventType] = removeChan(eb.subscribers[eventType], ch)
		}
	}
}

// SubscribeAll registers a channel to receive every event type.
func (eb *EventBus) SubscribeAll(ch chan Event) func() {
	return eb.Subscribe("", ch)
}

// Publish sends an event to all subscribers of its type and all wildcard subscribers.
// If the context is cancelled, publishing stops early.
func (eb *EventBus) Publish(ctx context.Context, event Event) error {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return fmt.Errorf("event bus is closed")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Send to type-specific subscribers
	for _, ch := range eb.subscribers[event.Type] {
		select {
		case ch <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Send to wildcard subscribers
	for _, ch := range eb.wildcard {
		select {
		case ch <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// Close shuts down the event bus and drains subscriber channels.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}
	eb.closed = true

	for _, ch := range eb.wildcard {
		close(ch)
	}
	for _, subs := range eb.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	eb.wildcard = nil
	eb.subscribers = nil
}

func removeChan(s []chan Event, target chan Event) []chan Event {
	for i, c := range s {
		if c == target {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
