package invalidation

import (
	"sync"
	"time"
)

// LocalBus is an in-process event bus that delivers events to subscribed handlers.
// It is safe for concurrent use by multiple goroutines.
type LocalBus struct {
	mu       sync.RWMutex
	handlers []handlerEntry
	nextID   int
	closed   bool
}
type handlerEntry struct {
	id      int
	handler Handler
}

// NewLocalBus creates a new LocalBus.
func NewLocalBus() *LocalBus {
	return &LocalBus{}
}

// Publish delivers the event to all subscribed handlers.
// Handler panics are recovered and do not affect other handlers.
func (b *LocalBus) Publish(evt Event) {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	b.mu.RLock()
	handlers := make([]handlerEntry, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()
	for _, h := range handlers {
		func() {
			defer func() { _ = recover() }()
			h.handler.OnEvent(evt)
		}()
	}
}

// Subscribe registers a handler and returns an unsubscribe function.
func (b *LocalBus) Subscribe(handler Handler) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return func() {}
	}
	id := b.nextID
	b.nextID++
	b.handlers = append(b.handlers, handlerEntry{id: id, handler: handler})
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, h := range b.handlers {
			if h.id == id {
				b.handlers = append(b.handlers[:i], b.handlers[i+1:]...)
				return
			}
		}
	}
}

// Close removes all handlers and prevents further subscriptions.
func (b *LocalBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.handlers = nil
}
