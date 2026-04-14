// Package singlefght provides a context-aware singleflight wrapper that
// deduplicates concurrent calls for the same key. If the caller's context
// is cancelled while a computation is in-flight, the caller receives
// ctx.Err() rather than the result of a concurrent waiter.
package singlefght

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

// GroupStats tracks operational metrics for the singleflight group.
type GroupStats struct {
	// Inflight is the current number of in-flight deduplicated calls.
	Inflight int64
	// Deduplicated is the total number of calls that were deduplicated
	// (i.e., they joined an existing in-flight call instead of starting
	// a new one).
	Deduplicated int64
}

// Group wraps golang.org/x/sync/singleflight.Group with context awareness
// and operational metrics.
type Group struct {
	g        singleflight.Group
	inflight atomic.Int64
	deduped  atomic.Int64
}

// NewGroup returns a new Group ready for use.
func NewGroup() *Group { return &Group{} }

// Do executes fn if no other call for the same key is in-flight.
// If a call for key is already in-flight, Do waits for it to complete
// and returns the shared result.
//
// Context cancellation: if ctx is cancelled while waiting for a shared
// result, Do returns ctx.Err() immediately. The in-flight computation
// continues and its result is available to other waiters.
func (g *Group) Do(ctx context.Context, key string, fn func() ([]byte, error)) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	type result struct {
		val []byte
		err error
	}

	ch := make(chan result, 1)
	g.inflight.Add(1)
	go func() {
		defer g.inflight.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: panicError{value: r}}
			}
		}()
		v, err, shared := g.g.Do(key, func() (any, error) {
			return fn()
		})
		if shared {
			g.deduped.Add(1)
		}
		if err != nil {
			ch <- result{err: err}
			return
		}
		if v == nil {
			ch <- result{}
			return
		}
		b, ok := v.([]byte)
		if !ok {
			ch <- result{err: errTypeAssertion}
			return
		}
		ch <- result{val: b}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return res.val, nil
	}
}

// Forget tells the singleflight group to forget about a key so that
// subsequent calls for that key will execute fn rather than waiting
// for a previous result.
func (g *Group) Forget(key string) { g.g.Forget(key) }

// Stats returns a snapshot of the group's operational metrics.
func (g *Group) Stats() GroupStats {
	return GroupStats{
		Inflight:     g.inflight.Load(),
		Deduplicated: g.deduped.Load(),
	}
}

// panicError wraps a panic value as an error.
type panicError struct {
	value any
}

func (p panicError) Error() string { return "singleflight: panic in fn" }

var errTypeAssertion = panicError{value: "type assertion to []byte failed"}
