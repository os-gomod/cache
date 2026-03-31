// Package singlefght wraps golang.org/x/sync/singleflight with a typed,
// context-aware API for cache-stampede suppression.
package singlefght

import (
	"context"

	"golang.org/x/sync/singleflight"
)

// Group deduplicates concurrent calls to the same key.
type Group struct {
	g singleflight.Group
}

// NewGroup returns a new Group.
func NewGroup() *Group { return &Group{} }

// Do executes fn if no in-flight call for key exists; otherwise it waits and
// returns the result of the in-flight call.  The context is checked before
// invoking fn but is not passed to the deduplication group itself, meaning a
// cancel on one caller's context does not abort other waiters.
func (g *Group) Do(ctx context.Context, key string, fn func() ([]byte, error)) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	type result struct {
		val any
		err error
		pan any
	}

	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{pan: r}
			}
		}()

		v, err, _ := g.g.Do(key, func() (any, error) {
			return fn()
		})
		ch <- result{val: v, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.pan != nil {
			panic(res.pan)
		}
		if res.err != nil {
			return nil, res.err
		}
		if res.val == nil {
			return nil, nil
		}
		return res.val.([]byte), nil //nolint:forcetypeassert // fn always returns []byte
	}
}

// Forget clears any in-flight entry for key; subsequent calls will re-execute fn.
func (g *Group) Forget(key string) { g.g.Forget(key) }
