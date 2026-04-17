// Package invalidation provides event-based cache invalidation with local
// and pub/sub-based event buses.
package invalidation

import "time"

// EventKind identifies the type of cache invalidation event.
type EventKind string

const (
	// KindExpire indicates a key expired naturally.
	KindExpire EventKind = "expire"
	// KindEvict indicates a key was evicted due to capacity pressure.
	KindEvict EventKind = "evict"
	// KindDelete indicates a key was explicitly deleted.
	KindDelete EventKind = "delete"
	// KindInvalidate indicates a key was invalidated programmatically.
	KindInvalidate EventKind = "invalidate"
	// KindClear indicates the entire cache was cleared.
	KindClear EventKind = "clear"
)

// Event represents a cache invalidation event.
type Event struct {
	Kind      EventKind
	Key       string
	Pattern   string
	Backend   string
	Timestamp time.Time
}

// Handler processes cache invalidation events.
type Handler interface {
	OnEvent(evt Event)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as Handler.
type HandlerFunc func(evt Event)

// OnEvent calls h with the given event.
func (h HandlerFunc) OnEvent(evt Event) { h(evt) }

// Bus is the interface for publishing and subscribing to cache invalidation events.
type Bus interface {
	Publish(evt Event)
	Subscribe(handler Handler) (unsubscribe func())
}
