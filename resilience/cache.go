// Package resilience provides resilient caching wrappers that add circuit breaking,
// rate limiting, and retry policies to any cache backend.
package resilience

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/chain"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
)

// Cache wraps a backend with a resilience policy (circuit breaker, rate limiter, retry).
// It delegates all operations to the underlying backend through the policy executor.
type Cache struct {
	backend              backendiface.Backend
	policy               Policy
	chain                *observability.Chain
	explicitInterceptors []observability.Interceptor
	stats                *stats.Stats
	closed               atomic.Bool
}

// NewCacheWithPolicy creates a new resilient cache that wraps the given backend
// with the specified resilience policy and options.
func NewCacheWithPolicy(b backendiface.Backend, p Policy, opts ...CacheOption) *Cache {
	c := &Cache{
		backend: b,
		policy:  p,
		chain:   observability.NopChain(),
		stats:   stats.NewStats(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CacheOption is a functional option for configuring a resilient cache.
type CacheOption func(*Cache)

// WithInterceptors sets the observability interceptors for the resilient cache.
func WithInterceptors(interceptors ...observability.Interceptor) CacheOption {
	return func(c *Cache) {
		c.explicitInterceptors = interceptors
		c.chain = chain.SetInterceptors(c.chain, interceptors)
	}
}

// SetInterceptors replaces the observability interceptors for this cache.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	c.chain = chain.SetInterceptors(c.chain, interceptors)
}

// Policy returns the resilience policy used by this cache.
func (c *Cache) Policy() Policy { return c.policy }

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	op := observability.Op{Backend: "resilience", Name: "get", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	var val []byte
	err := c.policy.Execute(ctx, "cache.get", func(ctx context.Context) error {
		var e error
		val, e = c.backend.Get(ctx, key)
		return e
	})
	if err != nil {
		result.Err = err
	} else {
		result.Hit = true
		result.ByteSize = len(val)
	}
	return val, err
}

func (c *Cache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	op := observability.Op{Backend: "resilience", Name: "set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	err := c.policy.Execute(ctx, "cache.set", func(ctx context.Context) error {
		return c.backend.Set(ctx, key, val, ttl)
	})
	if err != nil {
		result.Err = err
	}
	return err
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	op := observability.Op{Backend: "resilience", Name: "delete", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	err := c.policy.Execute(ctx, "cache.delete", func(ctx context.Context) error {
		return c.backend.Delete(ctx, key)
	})
	if err != nil {
		result.Err = err
	}
	return err
}

func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	op := observability.Op{Backend: "resilience", Name: "exists", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	var ok bool
	err := c.policy.Execute(ctx, "cache.exists", func(ctx context.Context) error {
		var e error
		ok, e = c.backend.Exists(ctx, key)
		return e
	})
	if err != nil {
		result.Err = err
	}
	result.Hit = ok
	return ok, err
}

func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	op := observability.Op{Backend: "resilience", Name: "ttl", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	var d time.Duration
	err := c.policy.Execute(ctx, "cache.ttl", func(ctx context.Context) error {
		var e error
		d, e = c.backend.TTL(ctx, key)
		return e
	})
	if err != nil {
		result.Err = err
	}
	return d, err
}

func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	op := observability.Op{Backend: "resilience", Name: "get_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	var val map[string][]byte
	err := c.policy.Execute(ctx, "cache.get_multi", func(ctx context.Context) error {
		var e error
		val, e = c.backend.GetMulti(ctx, keys...)
		return e
	})
	if err != nil {
		result.Err = err
	}
	if len(val) > 0 {
		result.Hit = true
	}
	return val, err
}

func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	op := observability.Op{Backend: "resilience", Name: "set_multi", KeyCount: len(items)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	err := c.policy.Execute(ctx, "cache.set_multi", func(ctx context.Context) error {
		return c.backend.SetMulti(ctx, items, ttl)
	})
	if err != nil {
		result.Err = err
	}
	return err
}

func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	op := observability.Op{Backend: "resilience", Name: "delete_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	err := c.policy.Execute(ctx, "cache.delete_multi", func(ctx context.Context) error {
		return c.backend.DeleteMulti(ctx, keys...)
	})
	if err != nil {
		result.Err = err
	}
	return err
}

func (c *Cache) Ping(ctx context.Context) error {
	op := observability.Op{Backend: "resilience", Name: "ping"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	err := c.policy.Execute(ctx, "cache.ping", func(ctx context.Context) error {
		return c.backend.Ping(ctx)
	})
	if err != nil {
		result.Err = err
	}
	return err
}

// Close closes the underlying backend. Safe to call multiple times.
func (c *Cache) Close(ctx context.Context) error {
	if c == nil || c.backend == nil {
		return nil
	}
	if c.closed.Swap(true) {
		return nil
	}
	return c.backend.Close(ctx)
}

// Name returns the backend identifier for this cache ("resilience").
func (c *Cache) Name() string { return "resilience" }

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.closed.Load() }
