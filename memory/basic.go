package memory

import (
	"context"
	"errors"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
)

// Get retrieves the value for the given key from the in-memory cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := c.checkClosed("memory.get"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "memory", Name: "get", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if cachectx.ShouldBypassCache(ctx) {
		c.stats.Miss()
		result.Hit = false
		return nil, cacheerrors.ErrNotFound
	}
	c.stats.RecordGet()
	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()
	if !ok {
		c.stats.Miss()
		result.Hit = false
		return nil, cacheerrors.NotFound("memory.get", key)
	}
	e.Touch()
	s.evict.OnAccess(key, e)
	c.stats.Hit()
	result.Hit = true
	result.ByteSize = len(e.Value)
	return e.Value, nil
}

// Set stores a key-value pair with the given TTL.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.checkClosed("memory.set"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	e := eviction.NewEntry(key, value, ttl, 0)
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, hadOld := s.items[key]; hadOld {
		s.size -= old.Size
		s.count--
		s.evict.OnDelete(key)
		c.stats.AddMemory(-old.Size)
		c.stats.AddItems(-1)
	}
	s.items[key] = e
	s.size += e.Size
	s.count++
	s.evict.OnAdd(key, e)
	c.stats.SetOp()
	c.stats.AddMemory(e.Size)
	c.stats.AddItems(1)
	c.enforceCapacity(s, key)
	return nil
}

// Delete removes a key from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed("memory.delete"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "delete", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	c.deleteFromShard(s, key)
	return nil
}

// Exists reports whether the key exists in the cache and is not expired.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("memory.exists"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "memory", Name: "exists", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if cachectx.ShouldBypassCache(ctx) {
		return false, nil
	}
	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()
	result.Hit = ok
	return ok, nil
}

// TTL returns the remaining time-to-live for the given key.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("memory.ttl"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "memory", Name: "ttl", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()
	if !ok {
		result.Err = cacheerrors.NotFound("memory.ttl", key)
		return 0, result.Err
	}
	return e.TTLRemaining(), nil
}

// Keys returns all non-expired keys in the cache.
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := c.checkClosed("memory.keys"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "memory", Name: "keys"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	_ = pattern
	now := time.Now().UnixNano()
	var keys []string
	for _, s := range c.shards {
		s.mu.RLock()
		for k, e := range s.items {
			if !e.IsExpiredAt(now) {
				keys = append(keys, k)
			}
		}
		s.mu.RUnlock()
	}
	op.KeyCount = len(keys)
	return keys, nil
}

// Size returns the number of entries currently in the cache.
func (c *Cache) Size(ctx context.Context) (int64, error) {
	if err := c.checkClosed("memory.size"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "memory", Name: "size"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	return c.stats.Items(), nil
}

// Expire updates the TTL for an existing key.
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.checkClosed("memory.expire"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "expire", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if ttl < 0 {
		result.Err = cacheerrors.InvalidKey("memory.expire", key, errors.New("ttl cannot be negative"))
		return result.Err
	}
	s := c.shardFor(key)
	now := time.Now().UnixNano()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || e.IsExpiredAt(now) {
		result.Err = cacheerrors.NotFound("memory.expire", key)
		return result.Err
	}
	newEntry := eviction.NewEntry(key, e.Value, ttl, 0)
	newEntry.Hits = e.Hits
	newEntry.Frequency = e.Frequency
	s.size += newEntry.Size - e.Size
	s.items[key] = newEntry
	s.evict.OnDelete(key)
	s.evict.OnAdd(key, newEntry)
	c.stats.SetOp()
	return nil
}

// Persist removes the TTL from a key, making it persist indefinitely.
func (c *Cache) Persist(ctx context.Context, key string) error {
	return c.Expire(ctx, key, 0)
}

// Clear removes all entries from the cache and resets statistics.
func (c *Cache) Clear(ctx context.Context) error {
	if err := c.checkClosed("memory.clear"); err != nil {
		return err
	}
	op := observability.Op{Backend: "memory", Name: "clear"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	for _, s := range c.shards {
		s.mu.Lock()
		s.items = make(map[string]*eviction.Entry, 64)
		s.evict.Reset()
		s.size = 0
		s.count = 0
		s.mu.Unlock()
	}
	c.stats.Reset()
	return nil
}
