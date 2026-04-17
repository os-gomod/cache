// Package singlefght provides singleflight-style deduplication for concurrent operations.
package singlefght

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

type GroupStats struct {
	Inflight     int64
	Deduplicated int64
}
type Group struct {
	g        singleflight.Group
	inflight atomic.Int64
	deduped  atomic.Int64
}

func NewGroup() *Group { return &Group{} }
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
func (g *Group) Forget(key string) { g.g.Forget(key) }
func (g *Group) Stats() GroupStats {
	return GroupStats{
		Inflight:     g.inflight.Load(),
		Deduplicated: g.deduped.Load(),
	}
}

type panicError struct {
	value any
}

func (p panicError) Error() string { return "singleflight: panic in fn" }

var errTypeAssertion = panicError{value: "type assertion to []byte failed"}
