package memory

import (
	"context"
	"errors"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/eviction"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/clock"
	"github.com/os-gomod/cache/internal/obs"
)

// Get retrieves a value by key.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	_, span := obs.Start(ctx, "memory.get")
	defer span.End()

	if err := c.checkClosed("memory.get"); err != nil {
		span.SetError(err)
		return nil, err
	}
	if cachectx.ShouldBypassCache(ctx) {
		c.stats.Miss()
		obs.RecordMiss(ctx, "memory", "get", 0)
		return nil, _errors.ErrNotFound
	}

	c.stats.GetOp()
	s := c.shardFor(key)
	now := clock.Now()

	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()

	if !ok {
		c.stats.Miss()
		obs.RecordMiss(ctx, "memory", "get", 0)
		return nil, _errors.NotFound("memory.get", key)
	}

	e.Touch()
	s.evict.OnAccess(key, e)
	c.stats.Hit()
	obs.RecordHit(ctx, "memory", "get", 0)
	return e.Value, nil
}

// Set stores a value with optional TTL (0 = persistent/no expiry).
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.checkClosed("memory.set"); err != nil {
		return err
	}

	e := eviction.NewEntry(key, value, ttl)
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

// Delete removes a key.
func (c *Cache) Delete(_ context.Context, key string) error {
	if err := c.checkClosed("memory.delete"); err != nil {
		return err
	}
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	c.deleteFromShard(s, key)
	return nil
}

// Exists reports whether key is present and not expired.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("memory.exists"); err != nil {
		return false, err
	}
	if cachectx.ShouldBypassCache(ctx) {
		return false, nil
	}
	s := c.shardFor(key)
	now := clock.Now()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()
	return ok, nil
}

// TTL returns the remaining TTL for a key.
//
//nolint:revive // ctx is required for interface compliance
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("memory.ttl"); err != nil {
		return 0, err
	}
	s := c.shardFor(key)
	now := clock.Now()
	s.mu.RLock()
	e, ok := s.items[key]
	if ok && e.IsExpiredAt(now) {
		ok = false
	}
	s.mu.RUnlock()
	if !ok {
		return 0, _errors.NotFound("memory.ttl", key)
	}
	return e.TTLRemaining(), nil
}

// Keys returns all keys matching pattern (* = any key).
//
//nolint:unparam // pattern is always used; kept for interface compliance
func (c *Cache) Keys(_ context.Context, pattern string) ([]string, error) {
	if err := c.checkClosed("memory.keys"); err != nil {
		return nil, err
	}
	_ = pattern // simple wildcard not implemented; return all non-expired keys
	now := clock.Now()
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
	return keys, nil
}

// Size returns the number of live (non-expired) entries.
func (c *Cache) Size(_ context.Context) (int64, error) {
	if err := c.checkClosed("memory.size"); err != nil {
		return 0, err
	}
	return c.stats.Items(), nil
}

func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	_ = ctx
	if err := c.checkClosed("memory.expire"); err != nil {
		return err
	}
	if ttl < 0 {
		return _errors.InvalidKey("memory.expire", key, errors.New("ttl cannot be negative"))
	}

	s := c.shardFor(key)
	now := clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[key]
	if !ok || e.IsExpiredAt(now) {
		return _errors.NotFound("memory.expire", key)
	}

	// Create new entry with updated TTL
	newEntry := eviction.NewEntry(key, e.Value, ttl)
	newEntry.Hits = e.Hits
	newEntry.Frequency = e.Frequency

	s.size += newEntry.Size - e.Size
	s.items[key] = newEntry

	// Eviction bookkeeping
	s.evict.OnDelete(key)
	s.evict.OnAdd(key, newEntry)

	c.stats.SetOp()
	return nil
}

// Persist removes the TTL from a key, making it never expire.
func (c *Cache) Persist(ctx context.Context, key string) error {
	return c.Expire(ctx, key, 0)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(_ context.Context) error {
	if err := c.checkClosed("memory.clear"); err != nil {
		return err
	}
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
