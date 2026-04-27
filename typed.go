package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/serialization"
	"github.com/os-gomod/cache/v2/layered"
	"github.com/os-gomod/cache/v2/memory"
	"github.com/os-gomod/cache/v2/redis"
)

// TypedCache provides a type-safe wrapper around a Backend. All values
// are transparently encoded/decoded using the configured Codec, so
// consumers work with native Go types instead of raw []byte.
//
// TypedCache is generic over the value type T. Common instantiations
// include TypedCache[string], TypedCache[int64], TypedCache[[]byte],
// and TypedCache[any] (with JSON codec for arbitrary structs).
//
// Example:
//
//	tc, _ := cache.NewMemoryString()
//	tc.Set(ctx, "name", "Alice", time.Minute)
//	name, err := tc.Get(ctx, "name") // name is string
type TypedCache[T any] struct {
	backend            contracts.Reader
	writer             contracts.Writer
	atomic             contracts.AtomicOps
	scanner            contracts.Scanner
	life               contracts.Lifecycle
	stats              contracts.StatsProvider
	codec              serialization.Codec[T]
	bufPool            *serialization.BufPool
	singleflightGroups sync.Map // key -> *singleflight.Group
}

// TypedOption is a functional option for configuring a TypedCache.
type TypedOption[T any] func(*TypedCache[T])

// TypedCacheOptionFunc is an adapter to allow the use of ordinary
// functions as TypedOption values. It is provided for convenience
// when options do not depend on the type parameter.
func TypedCacheOptionFunc[T any](fn func(*TypedCache[T])) TypedOption[T] {
	return fn
}

// NewTyped creates a new type-safe cache wrapper around the given backend.
// The codec is used to serialize/deserialize values of type T to/from []byte.
// A buffer pool is created automatically for efficient encoding/decoding.
//
// Example:
//
//	tc := cache.NewTyped[User](backend, serialization.NewJSONCodec[User]())
func NewTyped[T any](backend Backend, codec serialization.Codec[T], opts ...TypedOption[T]) *TypedCache[T] {
	tc := &TypedCache[T]{
		backend: backend,
		writer:  backend,
		atomic:  backend,
		scanner: backend,
		life:    backend,
		stats:   backend,
		codec:   codec,
		bufPool: serialization.NewBufPool(4096),
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

// Get retrieves the value for the given key, decoding it from []byte to T.
// Returns errors.NotFound if the key does not exist.
func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	data, err := tc.backend.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	val, err := tc.codec.Decode(data)
	if err != nil {
		return zero, errors.Factory.DecodeFailed(key, tc.codec.Name(), err)
	}
	return val, nil
}

// Set stores the given value under the key, encoding it from T to []byte.
// The TTL controls how long the entry remains valid.
func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	buf := tc.bufPool.Get()
	defer tc.bufPool.Put(buf)

	encoded, err := tc.codec.Encode(value, *buf)
	if err != nil {
		return errors.Factory.EncodeFailed(key, tc.codec.Name(), err)
	}
	return tc.writer.Set(ctx, key, encoded, ttl)
}

// Delete removes the entry for the given key.
func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	return tc.writer.Delete(ctx, key)
}

// Exists checks whether the key exists in the cache.
func (tc *TypedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	return tc.backend.Exists(ctx, key)
}

// TTL returns the remaining time-to-live for the given key.
func (tc *TypedCache[T]) TTL(ctx context.Context, key string) (time.Duration, error) {
	return tc.backend.TTL(ctx, key)
}

// GetMulti retrieves multiple values in a single batch operation,
// decoding each from []byte to T. Missing keys are omitted from the
// returned map.
func (tc *TypedCache[T]) GetMulti(ctx context.Context, keys ...string) (map[string]T, error) {
	dataMap, err := tc.backend.GetMulti(ctx, keys...)
	if err != nil {
		return nil, err
	}
	result := make(map[string]T, len(dataMap))
	for k, data := range dataMap {
		val, decErr := tc.codec.Decode(data)
		if decErr != nil {
			return nil, errors.Factory.DecodeFailed(k, tc.codec.Name(), decErr)
		}
		result[k] = val
	}
	return result, nil
}

// SetMulti stores multiple values in a single batch operation,
// encoding each from T to []byte.
func (tc *TypedCache[T]) SetMulti(ctx context.Context, items map[string]T, ttl time.Duration) error {
	encoded := make(map[string][]byte, len(items))
	buf := tc.bufPool.Get()
	defer tc.bufPool.Put(buf)

	for k, v := range items {
		data, err := tc.codec.Encode(v, *buf)
		if err != nil {
			return errors.Factory.EncodeFailed(k, tc.codec.Name(), err)
		}
		encoded[k] = data
	}
	return tc.writer.SetMulti(ctx, encoded, ttl)
}

// DeleteMulti removes multiple keys in a single batch operation.
func (tc *TypedCache[T]) DeleteMulti(ctx context.Context, keys ...string) error {
	return tc.writer.DeleteMulti(ctx, keys...)
}

// GetOrSet retrieves the value for the given key. If the key is missing
// or expired, fn is called to produce the value, which is then stored
// with the given TTL. This implements the cache-aside pattern.
//
// The function fn is guaranteed to be called at most once per cache miss,
// even under concurrent access for the same key. Concurrent callers for
// the same key will wait for the first caller's fn result, preventing
// thundering herd and duplicate loads.
func (tc *TypedCache[T]) GetOrSet(ctx context.Context, key string, fn func() (T, error), ttl time.Duration) (T, error) {
	var zero T

	val, err := tc.Get(ctx, key)
	if err == nil {
		return val, nil
	}

	// Only call fn on cache miss (not on other errors).
	if !errors.Factory.IsNotFound(err) {
		return zero, err
	}

	// Use singleflight to deduplicate concurrent loads for the same key.
	// Get or create a singleflight.Group for this key (lazily created).
	sfGroupInterface, _ := tc.singleflightGroups.LoadOrStore(key, &singleflight.Group{})
	sfGroup := sfGroupInterface.(*singleflight.Group)

	// Execute fn only once, even with concurrent callers for the same key.
	// All concurrent callers will receive the same result.
	result, err, _ := sfGroup.Do("load", func() (interface{}, error) {
		return fn()
	})

	if err != nil {
		return zero, errors.Factory.CallbackFailed(key, err)
	}

	newVal := result.(T)

	// Try to set (even if another goroutine set it concurrently)
	// This is a benign race - both sets have the same value
	if setErr := tc.Set(ctx, key, newVal, ttl); setErr != nil {
		return newVal, nil // return the value even if set fails
	}

	return newVal, nil
}

// CompareAndSwap atomically compares the current value with oldVal and,
// if they match, replaces it with newVal. Returns true if the swap
// was performed. Both oldVal and newVal are encoded/decoded using
// the configured codec.
func (tc *TypedCache[T]) CompareAndSwap(ctx context.Context, key string, oldVal, newVal T, ttl time.Duration) (bool, error) {
	buf := tc.bufPool.Get()
	defer tc.bufPool.Put(buf)

	oldEncoded, err := tc.codec.Encode(oldVal, *buf)
	if err != nil {
		return false, errors.Factory.EncodeFailed(key, tc.codec.Name(), err)
	}

	newEncoded, err := tc.codec.Encode(newVal, *buf)
	if err != nil {
		return false, errors.Factory.EncodeFailed(key, tc.codec.Name(), err)
	}

	return tc.atomic.CompareAndSwap(ctx, key, oldEncoded, newEncoded, ttl)
}

// SetNX sets the key to the given value only if it does not already exist.
// Returns true if the value was set, false if the key already existed.
func (tc *TypedCache[T]) SetNX(ctx context.Context, key string, value T, ttl time.Duration) (bool, error) {
	buf := tc.bufPool.Get()
	defer tc.bufPool.Put(buf)

	encoded, err := tc.codec.Encode(value, *buf)
	if err != nil {
		return false, errors.Factory.EncodeFailed(key, tc.codec.Name(), err)
	}
	return tc.atomic.SetNX(ctx, key, encoded, ttl)
}

// GetSet atomically sets the key to the given value and returns the
// previous value. If the key did not exist, the zero value of T is
// returned.
func (tc *TypedCache[T]) GetSet(ctx context.Context, key string, value T, ttl time.Duration) (T, error) {
	var zero T

	buf := tc.bufPool.Get()
	defer tc.bufPool.Put(buf)

	encoded, err := tc.codec.Encode(value, *buf)
	if err != nil {
		return zero, errors.Factory.EncodeFailed(key, tc.codec.Name(), err)
	}

	oldEncoded, err := tc.atomic.GetSet(ctx, key, encoded, ttl)
	if err != nil {
		return zero, err
	}

	if len(oldEncoded) == 0 {
		return zero, nil
	}

	val, err := tc.codec.Decode(oldEncoded)
	if err != nil {
		return zero, errors.Factory.DecodeFailed(key, tc.codec.Name(), err)
	}
	return val, nil
}

// Increment atomically increments a numeric value stored at key by delta.
// This is a convenience method for integer-typed caches.
func (tc *TypedCache[T]) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return tc.atomic.Increment(ctx, key, delta)
}

// Decrement atomically decrements a numeric value stored at key by delta.
// This is a convenience method for integer-typed caches.
func (tc *TypedCache[T]) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return tc.atomic.Decrement(ctx, key, delta)
}

// Keys returns all keys matching the given pattern. The pattern syntax
// depends on the backend (e.g., Redis-style glob patterns for Redis).
func (tc *TypedCache[T]) Keys(ctx context.Context, pattern string) ([]string, error) {
	return tc.scanner.Keys(ctx, pattern)
}

// Clear removes all entries from the cache.
func (tc *TypedCache[T]) Clear(ctx context.Context) error {
	return tc.scanner.Clear(ctx)
}

// Size returns the approximate number of entries in the cache.
func (tc *TypedCache[T]) Size(ctx context.Context) (int64, error) {
	return tc.scanner.Size(ctx)
}

// Close gracefully shuts down the underlying backend, releasing resources.
func (tc *TypedCache[T]) Close(ctx context.Context) error {
	return tc.life.Close(ctx)
}

// Ping checks connectivity to the backend. Returns nil if the backend
// is reachable and healthy.
func (tc *TypedCache[T]) Ping(ctx context.Context) error {
	return tc.life.Ping(ctx)
}

// Stats returns a snapshot of the cache's current statistics including
// hit rate, miss rate, eviction count, and more.
func (tc *TypedCache[T]) Stats() contracts.StatsSnapshot {
	return tc.stats.Stats()
}

// Closed returns true if the underlying backend has been closed.
func (tc *TypedCache[T]) Closed() bool {
	return tc.life.Closed()
}

// Name returns the name/identifier of the underlying backend.
func (tc *TypedCache[T]) Name() string {
	return tc.life.Name()
}

// Backend returns the underlying Backend instance. This is useful
// for advanced operations that are not exposed through the TypedCache API.
func (tc *TypedCache[T]) Backend() Backend {
	return tc.backend.(Backend)
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

// NewMemoryJSON creates a memory-backed typed cache with JSON codec
// for arbitrary Go types (typically structs).
func NewMemoryJSON[T any](opts ...memory.Option) (*TypedCache[T], error) {
	backend, err := NewMemory(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[T](backend, serialization.NewJSONCodec[T]()), nil
}

// NewMemoryString creates a memory-backed typed cache optimized for
// string values. Uses the efficient StringCodec (no JSON overhead).
func NewMemoryString(opts ...memory.Option) (*TypedCache[string], error) {
	backend, err := NewMemory(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[string](backend, &serialization.StringCodec{}), nil
}

// NewMemoryInt64 creates a memory-backed typed cache optimized for
// int64 values. Uses the efficient Int64Codec (no JSON overhead).
func NewMemoryInt64(opts ...memory.Option) (*TypedCache[int64], error) {
	backend, err := NewMemory(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[int64](backend, &serialization.Int64Codec{}), nil
}

// NewRedisJSON creates a Redis-backed typed cache with JSON codec
// for arbitrary Go types.
func NewRedisJSON[T any](opts ...redis.Option) (*TypedCache[T], error) {
	backend, err := NewRedis(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[T](backend, serialization.NewJSONCodec[T]()), nil
}

// NewRedisString creates a Redis-backed typed cache optimized for
// string values.
func NewRedisString(opts ...redis.Option) (*TypedCache[string], error) {
	backend, err := NewRedis(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[string](backend, &serialization.StringCodec{}), nil
}

// NewLayeredJSON creates a layered (L1+L2) typed cache with JSON codec.
func NewLayeredJSON[T any](opts ...layered.Option) (*TypedCache[T], error) {
	backend, err := NewLayered(opts...)
	if err != nil {
		return nil, err
	}
	return NewTyped[T](backend, serialization.NewJSONCodec[T]()), nil
}

// Verify interface satisfaction at compile time.
var (
	_ Backend = (*memory.Store)(nil)  //nolint:errcheck // compile-time interface satisfaction
	_ Backend = (*redis.Store)(nil)   //nolint:errcheck // compile-time interface satisfaction
	_ Backend = (*layered.Store)(nil) //nolint:errcheck // compile-time interface satisfaction
)

// Ensure sync is imported (used implicitly by typed cache operations).
var _ sync.Locker = (*sync.Mutex)(nil)
