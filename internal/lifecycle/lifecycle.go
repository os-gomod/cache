// Package lifecycle provides a reusable closed-state guard for cache backends.
// It replaces the hand-rolled atomic.Bool + checkClosed pattern that was
// duplicated across memory, redis, and layered packages.
package lifecycle

import (
	"sync/atomic"

	_errors "github.com/os-gomod/cache/errors"
)

// Guard tracks whether a resource has been closed. The zero value is open
// and valid, making it safe to embed by value in struct types.
//
// Usage:
//
//	type Cache struct {
//	    guard lifecycle.Guard
//	    // ...
//	}
//
//	func (c *Cache) Get(key string) error {
//	    if err := c.guard.CheckClosed("cache.get"); err != nil {
//	        return err
//	    }
//	    // ...
//	}
type Guard struct {
	closed atomic.Bool
}

// CheckClosed returns a Closed error if the guard has been marked closed.
// The op string is included in the error message for debugging.
func (g *Guard) CheckClosed(op string) error {
	if g.closed.Load() {
		return _errors.Closed(op)
	}
	return nil
}

// Close marks the guard as closed and reports whether this was the first
// close call. If the guard was already closed, it returns false.
func (g *Guard) Close() (alreadyClosed bool) {
	return g.closed.Swap(true)
}

// IsClosed reports whether the guard has been closed.
func (g *Guard) IsClosed() bool {
	return g.closed.Load()
}
