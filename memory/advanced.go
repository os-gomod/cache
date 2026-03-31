// Package memory provides a high-performance, sharded in-process cache.
package memory

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/os-gomod/cache/eviction"
	"github.com/os-gomod/cache/internal/clock"
)

// GetMulti retrieves multiple values.  Missing keys are silently omitted.
func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := c.checkClosed("memory.get_multi"); err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if v, err := c.Get(ctx, k); err == nil {
			out[k] = v
		}
	}
	return out, nil
}

// SetMulti stores multiple values.
func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if err := c.checkClosed("memory.set_multi"); err != nil {
		return err
	}
	for k, v := range items {
		if err := c.Set(ctx, k, v, ttl); err != nil {
			return fmt.Errorf("memory.set_multi key=%s: %w", k, err)
		}
	}
	return nil
}

// DeleteMulti removes multiple keys.
func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("memory.delete_multi"); err != nil {
		return err
	}
	for _, k := range keys {
		if err := c.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

// GetOrSet returns the cached value for key, or calls fn to compute it.
// Uses singleflight to prevent cache-stampede: only one goroutine computes
// for any given key at a time.
func (c *Cache) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("memory.get_or_set"); err != nil {
		return nil, err
	}
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	}
	return c.sg.Do(ctx, key, func() ([]byte, error) {
		if val, err := c.Get(ctx, key); err == nil {
			return val, nil
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		_ = c.Set(ctx, key, val, ttl)
		return val, nil
	})
}

// GetSet atomically sets key to value and returns the old value.
func (c *Cache) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("memory.getset"); err != nil {
		return nil, err
	}
	old, _ := c.Get(ctx, key)
	_ = c.Set(ctx, key, value, ttl)
	return old, nil
}

// CompareAndSwap atomically replaces a value only if it matches oldVal.
func (c *Cache) CompareAndSwap(
	_ context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("memory.cas"); err != nil {
		return false, err
	}
	s := c.shardFor(key)
	now := clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[key]
	if !ok || e.IsExpiredAt(now) {
		return false, nil
	}
	if !bytes.Equal(e.Value, oldVal) {
		return false, nil
	}

	ne := eviction.NewEntry(key, newVal, ttl)
	s.size += ne.Size - e.Size
	s.items[key] = ne
	s.evict.OnAdd(key, ne)
	c.stats.SetOp()
	return true, nil
}

// Increment atomically adds delta to the numeric value stored at key.
func (c *Cache) Increment(_ context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("memory.increment"); err != nil {
		return 0, err
	}
	s := c.shardFor(key)
	now := clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	var current int64
	if e, ok := s.items[key]; ok && !e.IsExpiredAt(now) {
		if v, err := strconv.ParseInt(string(e.Value), 10, 64); err == nil {
			current = v
		}
	}
	current += delta
	ne := eviction.NewEntry(key, []byte(strconv.FormatInt(current, 10)), 0)
	s.items[key] = ne
	s.evict.OnAdd(key, ne)
	c.stats.SetOp()
	return current, nil
}

// Decrement atomically subtracts delta from the numeric value stored at key.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.Increment(ctx, key, -delta)
}

// SetNX sets key to value only if the key does not already exist.
func (c *Cache) SetNX(
	_ context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("memory.setnx"); err != nil {
		return false, err
	}
	s := c.shardFor(key)
	now := clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.items[key]; ok && !e.IsExpiredAt(now) {
		return false, nil
	}

	e := eviction.NewEntry(key, value, ttl)
	s.items[key] = e
	s.size += e.Size
	s.count++
	s.evict.OnAdd(key, e)
	c.stats.SetOp()
	c.stats.AddMemory(e.Size)
	c.stats.AddItems(1)
	return true, nil
}
