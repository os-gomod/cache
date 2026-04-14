package invalidation

import (
	"sync"
	"time"
)

// LocalBus is an in-process Bus that distributes events to all registered
// handlers. It is safe for concurrent use and suitable for single-instance
// deployments.
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

// NewLocalBus creates a new LocalBus ready to accept subscriptions and events.
func NewLocalBus() *LocalBus {
	return &LocalBus{}
}

// Publish sends an event to all registered handlers. If a handler panics,
// the panic is recovered so that other handlers are not affected.
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

// Subscribe registers a handler. The returned function removes the handler
// when called. If the bus is closed, Subscribe returns a no-op unsubscribe
// function.
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

// Close unsubscribes all handlers and prevents future subscriptions.
// After Close, Publish becomes a no-op.
func (b *LocalBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.handlers = nil
}
