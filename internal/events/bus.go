package events

import (
	"reflect"
	"sync"
	"sync/atomic"
)

// HandlerFunc is a function that handles an event.
type HandlerFunc func(event Event)

// Bus is a publish/subscribe event bus.
// Events are dispatched synchronously to subscribers in registration order.
type Bus struct {
	mu          sync.RWMutex
	handlers    map[reflect.Type][]HandlerFunc
	allHandlers []HandlerFunc
	seq         atomic.Int64
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[reflect.Type][]HandlerFunc),
	}
}

// Subscribe registers a handler for a specific event type.
// The handler will only receive events of the given type.
func (b *Bus) Subscribe(eventType reflect.Type, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscribeAll registers a handler that receives all events.
func (b *Bus) SubscribeAll(handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.allHandlers = append(b.allHandlers, handler)
}

// Publish dispatches an event to all matching subscribers.
// Type-specific subscribers are called first, then catch-all subscribers.
func (b *Bus) Publish(event Event) {
	b.seq.Add(1)

	b.mu.RLock()
	defer b.mu.RUnlock()

	eventType := reflect.TypeOf(event)

	// Type-specific handlers
	if handlers, ok := b.handlers[eventType]; ok {
		for _, h := range handlers {
			h(event)
		}
	}

	// Catch-all handlers
	for _, h := range b.allHandlers {
		h(event)
	}
}

// Seq returns the current event sequence number.
func (b *Bus) Seq() int64 {
	return b.seq.Load()
}
