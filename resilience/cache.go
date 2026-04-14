package resilience

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
)

// Backend is a local interface definition that mirrors cache.Backend.
// Defined locally to avoid circular imports with the root cache package.
type Backend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Stats() stats.Snapshot
	Closed() bool
	Name() string
}

// Cache is a thin adapter that wraps a Backend under a Policy.
// All operations are routed through Policy.Execute, which handles timeout
// wrapping, rate limiting, circuit breaking, and retry with backoff.
type Cache struct {
	backend              Backend
	policy               Policy
	chain                *observability.Chain
	explicitInterceptors []observability.Interceptor
	stats                *stats.Stats
	closed               atomic.Bool
}

// NewCacheWithPolicy creates a resilient cache with a full Policy (including
// retry and timeout support). This is the preferred constructor.
func NewCacheWithPolicy(b Backend, p Policy, opts ...CacheOption) *Cache {
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

// CacheOption customizes a Cache constructed via NewCacheWithPolicy.
type CacheOption func(*Cache)

// WithInterceptors sets the interceptor chain on the resilience cache.
func WithInterceptors(interceptors ...observability.Interceptor) CacheOption {
	return func(c *Cache) {
		c.explicitInterceptors = interceptors
		if len(interceptors) > 0 {
			c.chain = observability.NewChain(interceptors...)
		} else {
			c.chain = observability.NopChain()
		}
	}
}

// SetInterceptors replaces the interceptor chain.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	if len(interceptors) > 0 {
		c.chain = observability.NewChain(interceptors...)
	} else {
		c.chain = observability.NopChain()
	}
}

// Policy returns a copy of the cache's resilience policy.
func (c *Cache) Policy() Policy { return c.policy }

// ---------------------------------------------------------------------------
// Core KV — delegated to Policy.Execute, wrapped with interceptor chain
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Batch — delegated to Policy.Execute, wrapped with interceptor chain
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Lifecycle — Ping is guarded; Close/Stats/Closed are not.
// ---------------------------------------------------------------------------

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

func (c *Cache) Close(ctx context.Context) error {
	if c == nil || c.backend == nil {
		return nil
	}
	if c.closed.Swap(true) {
		return nil
	}
	return c.backend.Close(ctx)
}

// ---------------------------------------------------------------------------
// Observability
// ---------------------------------------------------------------------------

// Name returns the backend identifier "resilience".
func (c *Cache) Name() string { return "resilience" }

// Stats returns a snapshot of the cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.closed.Load() }
