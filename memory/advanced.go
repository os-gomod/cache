package memory

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
)

func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := c.checkClosed("memory.get_multi"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "memory", Name: "get_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	out := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if v, err := c.Get(ctx, k); err == nil {
			out[k] = v
		}
	}
	return out, nil
}

func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if err := c.checkClosed("memory.set_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "set_multi", KeyCount: len(items)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	for k, v := range items {
		if err := c.Set(ctx, k, v, ttl); err != nil {
			result.Err = fmt.Errorf("memory.set_multi key=%s: %w", k, err)
			return result.Err
		}
	}
	return nil
}

func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("memory.delete_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "delete_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	for _, k := range keys {
		if err := c.Delete(ctx, k); err != nil {
			result.Err = err
			return result.Err
		}
	}
	return nil
}

func (c *Cache) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("memory.get_or_set"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "memory", Name: "get_or_set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	// Try to get the existing entry from the shard for stampede detection.
	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()

	if ok {
		result.Hit = true
		result.ByteSize = len(e.Value)
		e.Touch()
		s.evict.OnAccess(key, e)
		c.stats.Hit()

		// Use stampede detector: if early refresh is warranted, it will
		// trigger a background refresh and return the stale value.
		val, err := c.detector.Do(ctx, key, e.Value, e,
			func(_ context.Context) ([]byte, error) {
				return fn()
			},
			func(newVal []byte) {
				_ = c.Set(ctx, key, newVal, ttl)
			},
		)
		if err != nil {
			// Fallback: return the stale value on detector error.
			return e.Value, nil
		}
		return val, nil
	}

	// Cache miss: use singleflight to deduplicate.
	c.stats.Miss()
	return c.sg.Do(ctx, key, func() ([]byte, error) {
		if val, err := c.Get(ctx, key); err == nil {
			result.Hit = true
			result.ByteSize = len(val)
			return val, nil
		}
		val, err := fn()
		if err != nil {
			result.Err = err
			return nil, err
		}
		_ = c.Set(ctx, key, val, ttl)
		result.ByteSize = len(val)
		return val, nil
	})
}

func (c *Cache) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("memory.getset"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "memory", Name: "getset", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	old, _ := c.Get(ctx, key)
	if old != nil {
		result.Hit = true
		result.ByteSize = len(old)
	}
	_ = c.Set(ctx, key, value, ttl)
	return old, nil
}

func (c *Cache) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("memory.cas"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "memory", Name: "cas", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || e.IsExpiredAt(now) {
		return false, nil
	}
	if !bytes.Equal(e.Value, oldVal) {
		return false, nil
	}
	ne := eviction.NewEntry(key, newVal, ttl, 0)
	s.size += ne.Size - e.Size
	s.items[key] = ne
	s.evict.OnAdd(key, ne)
	c.stats.SetOp()
	return true, nil
}

// Increment and Decrement are not strictly atomic since they don't support an optional TTL, but they do use the shard
// mutex to ensure that concurrent increments/decrements on the same key are serialized, so they will not interfere with
// each other and will produce a consistent result. However, if you need true atomicity with TTL support, you would need
// to implement a more complex compare-and-swap loop or use a different backend that supports atomic operations with
// TTLs (e.g., Redis). nolint:dupl // Increment and Decrement methods are similar enough that deduplication would add
// more complexity than it's worth.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("memory.increment"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "memory", Name: "increment", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.Lock()
	defer s.mu.Unlock()
	var current int64
	if e, ok := s.items[key]; ok && !e.IsExpiredAt(now) {
		if v, err := strconv.ParseInt(string(e.Value), 10, 64); err == nil {
			current = v
		}
	}
	current += delta
	ne := eviction.NewEntry(key, []byte(strconv.FormatInt(current, 10)), 0, 0)
	s.items[key] = ne
	s.evict.OnAdd(key, ne)
	c.stats.SetOp()
	return current, nil
}

// Decrement is implemented using the same logic as Increment, but it simply negates the delta to achieve the decrement
// effect. Note that this method does not support setting a TTL, so it's not strictly atomic in the sense of a full
// compare-and-swap operation, but it does ensure that concurrent decrements will not interfere with each other and will
// produce a consistent result. nolint:dupl // Increment and Decrement methods are similar enough that deduplication
// would add more complexity than it's worth.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.Increment(ctx, key, -delta)
}

func (c *Cache) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("memory.setnx"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "memory", Name: "setnx", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.items[key]; ok && !e.IsExpiredAt(now) {
		return false, nil
	}
	e := eviction.NewEntry(key, value, ttl, 0)
	s.items[key] = e
	s.size += e.Size
	s.count++
	s.evict.OnAdd(key, e)
	c.stats.SetOp()
	c.stats.AddMemory(e.Size)
	c.stats.AddItems(1)
	return true, nil
}
