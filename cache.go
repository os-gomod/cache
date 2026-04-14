// Package cache provides a unified caching library with multiple backend
// implementations, resilience patterns, observability integrations, and
// type-safe access.
//
// The recommended entry point for simple use cases is the cache.Memory,
// cache.Redis, or cache.Layered constructors. For production deployments
// with multiple backends, resilience policies, and observability, use
// cache.New() which returns a manager.CacheManager.
//
// Quick Start:
//
//	import "github.com/os-gomod/cache"
//
//	c, _ := cache.Memory()
//	_ = c.Set(ctx, "key", []byte("value"), 0)
//	val, _ := c.Get(ctx, "key")
package cache

import (
	"context"
	"time"

	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/layer"
	"github.com/os-gomod/cache/manager"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
)

// ---------------------------------------------------------------------------
// Type Aliases — single-import convenience
// ---------------------------------------------------------------------------

// Policy is the unified resilience policy.
type Policy = resilience.Policy

// Manager is the orchestration layer for named backends.
type Manager = manager.CacheManager

// Namespace is a key-prefixed view over a Manager.
type Namespace = manager.Namespace

// ---------------------------------------------------------------------------
// Compile-time interface checks — moved from sub-packages
// ---------------------------------------------------------------------------

var (
	_ Backend       = (*memory.Cache)(nil)
	_ AtomicBackend = (*memory.Cache)(nil)
	_ ScanBackend   = (*memory.Cache)(nil)

	_ Backend       = (*redis.Cache)(nil)
	_ AtomicBackend = (*redis.Cache)(nil)
	_ ScanBackend   = (*redis.Cache)(nil)

	_ AtomicBackend = (*layer.Cache)(nil)
	_ Backend       = (*resilience.Cache)(nil)
)

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// New creates a CacheManager with the given options.
func New(opts ...manager.Option) (*manager.CacheManager, error) {
	return manager.New(opts...)
}

// Memory creates a ready-to-use in-process memory cache.
func Memory(opts ...memory.Option) (*memory.Cache, error) {
	return memory.New(opts...)
}

// MemoryWithContext creates a ready-to-use in-process memory cache while
// propagating ctx to initialization.
func MemoryWithContext(ctx context.Context, opts ...memory.Option) (*memory.Cache, error) {
	return memory.NewWithContext(ctx, opts...)
}

// Redis creates a ready-to-use Redis-backed cache.
func Redis(opts ...redis.Option) (*redis.Cache, error) {
	return redis.New(opts...)
}

// RedisWithContext creates a ready-to-use Redis-backed cache while
// propagating ctx to initialization.
func RedisWithContext(ctx context.Context, opts ...redis.Option) (*redis.Cache, error) {
	return redis.NewWithContext(ctx, opts...)
}

// Layered creates a two-tier cache (L1 memory, L2 Redis).
func Layered(opts ...layer.Option) (*layer.Cache, error) {
	return layer.New(opts...)
}

// LayeredWithContext creates a two-tier cache (L1 memory, L2 Redis) while
// propagating ctx to backend construction and worker lifecycles.
func LayeredWithContext(ctx context.Context, opts ...layer.Option) (*layer.Cache, error) {
	return layer.NewWithContext(ctx, opts...)
}

// ---------------------------------------------------------------------------
// Generic Functions
// ---------------------------------------------------------------------------

// Get retrieves a value from b and decodes it using c.
func Get[T any](ctx context.Context, b Backend, key string, c codec.Codec[T]) (T, error) {
	var zero T
	data, err := b.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	return c.Decode(data)
}

// Set encodes value using c and stores it in b with the given TTL.
func Set[T any](
	ctx context.Context,
	b Backend,
	key string,
	value T,
	ttl time.Duration,
	c codec.Codec[T],
) error {
	data, err := c.Encode(value, nil)
	if err != nil {
		return err
	}
	return b.Set(ctx, key, data, ttl)
}

// Typed returns a TypedCache[T] backed by the named backend in m.
// If name is empty the default backend is used.
func Typed[T any](m *manager.CacheManager, name string, c codec.Codec[T]) (*TypedCache[T], error) {
	b, err := m.Backend(name)
	if err != nil {
		return nil, err
	}
	return NewTypedCache(b, c), nil
}
