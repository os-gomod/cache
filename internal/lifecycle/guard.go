// Package lifecycle provides start/close state management via an atomic guard
// pattern. It is used by all cache backends to track whether they have been
// closed, ensuring idempotent shutdown and rejecting operations after close.
package lifecycle

import (
	"fmt"
	"sync/atomic"
)

// Guard manages the closed state of a cache backend using a lock-free atomic
// boolean. It is safe for concurrent use by multiple goroutines.
//
// Typical usage:
//
//	var g lifecycle.Guard
//	if err := g.CheckClosed("get"); err != nil {
//	    return err
//	}
//	// ... perform operation ...
type Guard struct {
	closed atomic.Bool
}

// Close atomically marks the guard as closed. Returns true if the guard was
// already closed (making the current Close a no-op), false if this call
// transitioned the guard from open to closed.
func (g *Guard) Close() bool {
	return g.closed.Swap(true)
}

// IsClosed reports whether the guard is in a closed state.
func (g *Guard) IsClosed() bool {
	return g.closed.Load()
}

// CheckClosed returns an error if the guard is in a closed state, using the
// given operation name for error context. Returns nil if the guard is open
// and operations may proceed.
func (g *Guard) CheckClosed(op string) error {
	if g.closed.Load() {
		return fmt.Errorf("cache: cannot %s: cache is closed", op)
	}
	return nil
}

// Open reports whether the guard is still open (not closed).
func (g *Guard) Open() bool {
	return !g.closed.Load()
}
