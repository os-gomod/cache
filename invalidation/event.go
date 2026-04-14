// Package invalidation provides cache invalidation primitives for
// coordinating key-level and pattern-level invalidation across multiple
// cache backends and instances. It supports both local (in-process) and
// distributed (pub/sub) invalidation topologies.
//
// The core abstraction is the Bus, which distributes invalidation Events
// to registered Handlers. A LocalBus is provided for single-process
// deployments; for multi-instance deployments, implement PubSub on top of
// Redis Pub/Sub or similar.
package invalidation

import "time"

// EventKind classifies the type of cache invalidation event.
type EventKind string

const (
	// KindExpire indicates a key has expired from the cache naturally.
	KindExpire EventKind = "expire"
	// KindEvict indicates a key was evicted due to capacity pressure.
	KindEvict EventKind = "evict"
	// KindDelete indicates a key was explicitly deleted.
	KindDelete EventKind = "delete"
	// KindInvalidate indicates a manual invalidation request (e.g., from
	// an admin endpoint or a pattern-based sweep).
	KindInvalidate EventKind = "invalidate"
	// KindClear indicates the entire cache was cleared.
	KindClear EventKind = "clear"
)

// Event represents a single cache invalidation occurrence. Events are
// lightweight value types; Handlers must not retain references to the
// Key field beyond the callback duration.
type Event struct {
	// Kind is the type of invalidation that occurred.
	Kind EventKind
	// Key is the cache key affected. Empty for KindClear events.
	Key string
	// Pattern is the glob pattern for pattern-based invalidation.
	// Empty for single-key events.
	Pattern string
	// Backend is the name of the cache backend that originated the event.
	Backend string
	// Timestamp is when the event was generated.
	Timestamp time.Time
}

// Handler receives invalidation events. Implementations must be safe for
// concurrent use and must not block (events are dispatched synchronously).
type Handler interface {
	// OnEvent is called for each invalidation event. The handler must not
	// panic; any panic will be recovered and logged by the Bus.
	OnEvent(evt Event)
}

// HandlerFunc is a convenience adapter for ordinary functions to act as
// Handlers.
type HandlerFunc func(evt Event)

// OnEvent calls h(evt).
func (h HandlerFunc) OnEvent(evt Event) { h(evt) }

// Bus distributes invalidation events to registered handlers. All methods
// must be safe for concurrent use.
type Bus interface {
	// Publish sends an event to all registered handlers. Publish must not
	// block the caller for longer than necessary — handlers should be
	// dispatched asynchronously if they may be slow.
	Publish(evt Event)

	// Subscribe registers a handler to receive all future events. The
	// returned function unsubscribes the handler when called.
	Subscribe(handler Handler) (unsubscribe func())
}
