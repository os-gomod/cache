// Package cache provides a unified, high-performance caching library with support
// for in-memory, Redis, and layered (L1+L2) cache backends. It offers typed and
// untyped APIs, circuit breaker resilience, observability interceptors, and
// stampede protection.
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

// Policy is an alias for resilience.Policy, providing retry, circuit breaker,
// rate limiting, and timeout configuration for cache operations.
type Policy = resilience.Policy

// Manager is an alias for manager.CacheManager, the top-level cache manager
// that manages named backends with resilience policies.
type Manager = manager.CacheManager

// Namespace is an alias for manager.Namespace, providing key-prefixed access
// to a cache backend.
type Namespace = manager.Namespace

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

// New creates a new CacheManager with the given options. It is the primary
// entry point for configuring and managing multiple named cache backends.
func New(opts ...manager.Option) (*manager.CacheManager, error) {
	return manager.New(opts...)
}

// Memory creates a new in-memory cache backend with the provided options.
func Memory(opts ...memory.Option) (*memory.Cache, error) {
	return memory.New(opts...)
}

// MemoryWithContext creates a new in-memory cache backend with the given
// context and options.
func MemoryWithContext(ctx context.Context, opts ...memory.Option) (*memory.Cache, error) {
	return memory.NewWithContext(ctx, opts...)
}

// Redis creates a new Redis cache backend with the provided options.
func Redis(opts ...redis.Option) (*redis.Cache, error) {
	return redis.New(opts...)
}

// RedisWithContext creates a new Redis cache backend with the given
// context and options.
func RedisWithContext(ctx context.Context, opts ...redis.Option) (*redis.Cache, error) {
	return redis.NewWithContext(ctx, opts...)
}

// Layered creates a new layered cache backend (L1 in-memory + L2 Redis)
// with the provided options.
func Layered(opts ...layer.Option) (*layer.Cache, error) {
	return layer.New(opts...)
}

// LayeredWithContext creates a new layered cache backend with the given
// context and options.
func LayeredWithContext(ctx context.Context, opts ...layer.Option) (*layer.Cache, error) {
	return layer.NewWithContext(ctx, opts...)
}

// Get retrieves a typed value from the given backend using the provided codec
// to deserialize the raw bytes.
func Get[T any](ctx context.Context, b Backend, key string, c codec.Codec[T]) (T, error) {
	var zero T
	data, err := b.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	return c.Decode(data)
}

// Set stores a typed value into the given backend using the provided codec
// to serialize the value.
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

// Typed creates a TypedCache wrapping the named backend from the manager
// with the given codec for automatic serialization and deserialization.
func Typed[T any](m *manager.CacheManager, name string, c codec.Codec[T]) (*TypedCache[T], error) {
	b, err := m.Backend(name)
	if err != nil {
		return nil, err
	}
	return NewTypedCache(b, c), nil
}
