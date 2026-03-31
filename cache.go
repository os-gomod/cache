// Package cache provides a high-performance, multi-tier caching library.
// It supports in-memory, Redis, and layered (L1 memory + L2 Redis) caches
// with pluggable eviction policies, resilience wrappers, and typed helpers.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/layered"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
)

// ----------------------------------------------------------------------------
// Core interfaces
// ----------------------------------------------------------------------------

// KV defines basic key/value operations.
type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// Batch defines multi-key operations.
type Batch interface {
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error
	GetOrSet(
		ctx context.Context,
		key string,
		fn func() ([]byte, error),
		ttl time.Duration,
	) ([]byte, error)
}

// Atomic defines compare-and-swap / counter operations.
type Atomic interface {
	CompareAndSwap(
		ctx context.Context,
		key string,
		oldVal, newVal []byte,
		ttl time.Duration,
	) (bool, error)
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error)
}

// Admin defines lifecycle and inspection operations.
type Admin interface {
	Keys(ctx context.Context, pattern string) ([]string, error)
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
	Stats() stats.Snapshot
	Closed() bool
	Close(ctx context.Context) error
}

// CoreCache is the full unified interface implemented by all backends.
type CoreCache interface {
	KV
	Batch
	Atomic
	Admin
}

// Cache is an alias of CoreCache for brevity.
type Cache = CoreCache

// RedisCache extends CoreCache with Redis data-structure operations.
type RedisCache interface {
	CoreCache
	Ping(ctx context.Context) error
	HSet(ctx context.Context, key, field string, value any) error
	HGet(ctx context.Context, key, field string) ([]byte, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error
	LPush(ctx context.Context, key string, values ...any) error
	RPush(ctx context.Context, key string, values ...any) error
	LPop(ctx context.Context, key string) ([]byte, error)
	RPop(ctx context.Context, key string) ([]byte, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	SAdd(ctx context.Context, key string, members ...any) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...any) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRem(ctx context.Context, key string, members ...any) error
}

// ----------------------------------------------------------------------------
// CacheType
// ----------------------------------------------------------------------------

// CacheType identifies the backing store of a cache instance.
type CacheType int

const (
	CacheMemory CacheType = iota
	CacheRedis
	CacheLayered
)

func (c CacheType) String() string {
	switch c {
	case CacheMemory:
		return "memory"
	case CacheRedis:
		return "redis"
	case CacheLayered:
		return "layered"
	default:
		return "unknown"
	}
}

// ----------------------------------------------------------------------------
// Convenience constructors (re-exporting sub-package constructors)
// ----------------------------------------------------------------------------

// NewMemory creates an in-process cache.
func NewMemory(opts ...memory.Option) (*memory.Cache, error) {
	return memory.New(opts...)
}

// NewRedis creates a Redis-backed cache.
func NewRedis(opts ...redis.Option) (*redis.Cache, error) {
	return redis.New(opts...)
}

// NewLayered creates a two-level (L1 memory + L2 Redis) cache.
func NewLayered(opts ...layered.Option) (*layered.Cache, error) {
	return layered.New(opts...)
}

// NewResilient wraps a backend with circuit breaking and rate limiting.
func NewResilient(backend resilience.Backend, opts resilience.Options) *resilience.Cache {
	return resilience.NewCache(backend, opts)
}

// ----------------------------------------------------------------------------
// Codec
// ----------------------------------------------------------------------------

// Codec serializes and deserialises typed values to/from []byte.
type Codec[T any] interface {
	Encode(value T) ([]byte, error)
	Decode(data []byte) (T, error)
}

// JSONCodec serializes using encoding/json.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Encode(value T) ([]byte, error) { return json.Marshal(value) }
func (JSONCodec[T]) Decode(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

// StringCodec converts strings to/from raw bytes.
type StringCodec struct{}

func (StringCodec) Encode(value string) ([]byte, error) { return []byte(value), nil }
func (StringCodec) Decode(data []byte) (string, error)  { return string(data), nil }

// BytesCodec is a no-op codec for raw byte slices.
type BytesCodec struct{}

func (BytesCodec) Encode(value []byte) ([]byte, error) { return value, nil }
func (BytesCodec) Decode(data []byte) ([]byte, error)  { return data, nil }

// Int64Codec serializes int64 values as decimal strings (Redis-compatible).
type Int64Codec struct{}

func (Int64Codec) Encode(value int64) ([]byte, error) {
	return []byte(strconv.FormatInt(value, 10)), nil
}

func (Int64Codec) Decode(data []byte) (int64, error) {
	return strconv.ParseInt(string(data), 10, 64)
}

// ----------------------------------------------------------------------------
// TypedCache — generic type-safe wrapper
// ----------------------------------------------------------------------------

// TypedCache wraps any CoreCache with type-safe get/set methods.
type TypedCache[T any] struct {
	cache      CoreCache
	codec      Codec[T]
	cacheType  CacheType
	name       string
	onSetError func(key string, err error)
}

// NewTypedCache creates a TypedCache from any CoreCache and Codec.
func NewTypedCache[T any](
	c CoreCache,
	codec Codec[T],
	opts ...func(*TypedCache[T]),
) *TypedCache[T] {
	tc := &TypedCache[T]{
		cache:     c,
		codec:     codec,
		cacheType: detectCacheType(c),
		name:      fmt.Sprintf("%T", c),
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

// NewJSONTypedCache creates a TypedCache with JSON serialization.
func NewJSONTypedCache[T any](c CoreCache, opts ...func(*TypedCache[T])) *TypedCache[T] {
	return NewTypedCache(c, JSONCodec[T]{}, opts...)
}

// NewMemoryTypedCache creates a JSON-typed memory cache.
func NewMemoryTypedCache[T any](
	memOpts []memory.Option,
	opts ...func(*TypedCache[T]),
) (*TypedCache[T], error) {
	mc, err := memory.New(memOpts...)
	if err != nil {
		return nil, err
	}
	return NewJSONTypedCache[T](mc, opts...), nil
}

// NewRedisTypedCache creates a JSON-typed Redis cache.
func NewRedisTypedCache[T any](
	redisOpts []redis.Option,
	opts ...func(*TypedCache[T]),
) (*TypedCache[T], error) {
	rc, err := redis.New(redisOpts...)
	if err != nil {
		return nil, err
	}
	return NewJSONTypedCache[T](rc, opts...), nil
}

// NewLayeredTypedCache creates a JSON-typed layered cache.
func NewLayeredTypedCache[T any](
	layeredOpts []layered.Option,
	opts ...func(*TypedCache[T]),
) (*TypedCache[T], error) {
	lc, err := layered.New(layeredOpts...)
	if err != nil {
		return nil, err
	}
	return NewJSONTypedCache[T](lc, opts...), nil
}

// WithOnSetError attaches a callback invoked when encoding fails during Set.
func WithOnSetError[T any](fn func(key string, err error)) func(*TypedCache[T]) {
	return func(tc *TypedCache[T]) { tc.onSetError = fn }
}

func detectCacheType(c CoreCache) CacheType {
	switch c.(type) {
	case *redis.Cache:
		return CacheRedis
	case *layered.Cache:
		return CacheLayered
	default:
		return CacheMemory
	}
}

// --- TypedCache methods ---

func (tc *TypedCache[T]) Type() CacheType { return tc.cacheType }
func (tc *TypedCache[T]) Name() string    { return tc.name }

func validateKey(op, key string) error {
	if key == "" {
		return _errors.EmptyKey(op)
	}
	return nil
}

func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	if err := validateKey("typed.get", key); err != nil {
		return zero, err
	}
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	return tc.codec.Decode(data)
}

func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	if err := validateKey("typed.set", key); err != nil {
		return err
	}
	data, err := tc.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("typed.set encode: %w", err)
	}
	return tc.cache.Set(ctx, key, data, ttl)
}

func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	if err := validateKey("typed.delete", key); err != nil {
		return err
	}
	return tc.cache.Delete(ctx, key)
}

func (tc *TypedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	if err := validateKey("typed.exists", key); err != nil {
		return false, err
	}
	return tc.cache.Exists(ctx, key)
}

func (tc *TypedCache[T]) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := validateKey("typed.ttl", key); err != nil {
		return 0, err
	}
	return tc.cache.TTL(ctx, key)
}

func (tc *TypedCache[T]) GetMulti(ctx context.Context, keys ...string) (map[string]T, error) {
	for _, k := range keys {
		if err := validateKey("typed.get_multi", k); err != nil {
			return nil, err
		}
	}
	raw, err := tc.cache.GetMulti(ctx, keys...)
	if err != nil {
		return nil, err
	}
	out := make(map[string]T, len(raw))
	for k, data := range raw {
		v, decErr := tc.codec.Decode(data)
		if decErr != nil {
			return nil, fmt.Errorf("typed.get_multi decode key=%s: %w", k, decErr)
		}
		out[k] = v
	}
	return out, nil
}

func (tc *TypedCache[T]) SetMulti(
	ctx context.Context,
	items map[string]T,
	ttl time.Duration,
) error {
	raw := make(map[string][]byte, len(items))
	for k, v := range items {
		if err := validateKey("typed.set_multi", k); err != nil {
			return err
		}
		data, err := tc.codec.Encode(v)
		if err != nil {
			return fmt.Errorf("typed.set_multi encode key=%s: %w", k, err)
		}
		raw[k] = data
	}
	return tc.cache.SetMulti(ctx, raw, ttl)
}

func (tc *TypedCache[T]) DeleteMulti(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if err := validateKey("typed.delete_multi", k); err != nil {
			return err
		}
	}
	return tc.cache.DeleteMulti(ctx, keys...)
}

func (tc *TypedCache[T]) GetOrSet(
	ctx context.Context,
	key string,
	fn func() (T, error),
	ttl time.Duration,
) (T, error) {
	var zero T
	if err := validateKey("typed.get_or_set", key); err != nil {
		return zero, err
	}
	data, err := tc.cache.GetOrSet(ctx, key, func() ([]byte, error) {
		v, fnErr := fn()
		if fnErr != nil {
			return nil, fnErr
		}
		return tc.codec.Encode(v)
	}, ttl)
	if err != nil {
		return zero, err
	}
	v, decErr := tc.codec.Decode(data)
	if decErr != nil {
		if tc.onSetError != nil {
			tc.onSetError(key, decErr)
		}
		return zero, fmt.Errorf("typed.get_or_set decode: %w", decErr)
	}
	return v, nil
}

func (tc *TypedCache[T]) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal T,
	ttl time.Duration,
) (bool, error) {
	if err := validateKey("typed.cas", key); err != nil {
		return false, err
	}
	oldData, err := tc.codec.Encode(oldVal)
	if err != nil {
		return false, fmt.Errorf("typed.cas encode old: %w", err)
	}
	newData, err := tc.codec.Encode(newVal)
	if err != nil {
		return false, fmt.Errorf("typed.cas encode new: %w", err)
	}
	return tc.cache.CompareAndSwap(ctx, key, oldData, newData, ttl)
}

func (tc *TypedCache[T]) SetNX(
	ctx context.Context,
	key string,
	value T,
	ttl time.Duration,
) (bool, error) {
	if err := validateKey("typed.setnx", key); err != nil {
		return false, err
	}
	data, err := tc.codec.Encode(value)
	if err != nil {
		return false, fmt.Errorf("typed.setnx encode: %w", err)
	}
	return tc.cache.SetNX(ctx, key, data, ttl)
}

func (tc *TypedCache[T]) GetSet(
	ctx context.Context,
	key string,
	value T,
	ttl time.Duration,
) (T, error) {
	var zero T
	if err := validateKey("typed.getset", key); err != nil {
		return zero, err
	}
	data, err := tc.codec.Encode(value)
	if err != nil {
		return zero, fmt.Errorf("typed.getset encode: %w", err)
	}
	oldData, err := tc.cache.GetSet(ctx, key, data, ttl)
	if err != nil {
		return zero, err
	}
	if oldData == nil {
		return zero, nil
	}
	return tc.codec.Decode(oldData)
}

func (tc *TypedCache[T]) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := validateKey("typed.increment", key); err != nil {
		return 0, err
	}
	return tc.cache.Increment(ctx, key, delta)
}

func (tc *TypedCache[T]) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	if err := validateKey("typed.decrement", key); err != nil {
		return 0, err
	}
	return tc.cache.Decrement(ctx, key, delta)
}

func (tc *TypedCache[T]) Keys(ctx context.Context, pattern string) ([]string, error) {
	return tc.cache.Keys(ctx, pattern)
}

func (tc *TypedCache[T]) Clear(ctx context.Context) error         { return tc.cache.Clear(ctx) }
func (tc *TypedCache[T]) Size(ctx context.Context) (int64, error) { return tc.cache.Size(ctx) }
func (tc *TypedCache[T]) Stats() stats.Snapshot                   { return tc.cache.Stats() }
func (tc *TypedCache[T]) Closed() bool                            { return tc.cache.Closed() }
func (tc *TypedCache[T]) Close(ctx context.Context) error         { return tc.cache.Close(ctx) }

// ----------------------------------------------------------------------------
// Typed numeric convenience wrappers
// ----------------------------------------------------------------------------

// TypedInt64Cache provides type-safe int64 operations with atomic counter support.
type TypedInt64Cache struct{ *TypedCache[int64] }

// NewTypedInt64Cache wraps any CoreCache with int64 type safety.
func NewTypedInt64Cache(c CoreCache) *TypedInt64Cache {
	return &TypedInt64Cache{NewTypedCache(c, Int64Codec{})}
}

// NewMemoryInt64Cache creates a typed int64 memory cache.
func NewMemoryInt64Cache(opts ...memory.Option) (*TypedInt64Cache, error) {
	mc, err := memory.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewTypedInt64Cache(mc), nil
}

// NewRedisInt64Cache creates a typed int64 Redis cache.
func NewRedisInt64Cache(opts ...redis.Option) (*TypedInt64Cache, error) {
	rc, err := redis.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewTypedInt64Cache(rc), nil
}

// TypedStringCache provides type-safe string operations.
type TypedStringCache struct{ *TypedCache[string] }

// NewTypedStringCache wraps any CoreCache with string type safety.
func NewTypedStringCache(c CoreCache) *TypedStringCache {
	return &TypedStringCache{NewTypedCache(c, StringCodec{})}
}

// NewMemoryStringCache creates a typed string memory cache.
func NewMemoryStringCache(opts ...memory.Option) (*TypedStringCache, error) {
	mc, err := memory.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewTypedStringCache(mc), nil
}

// NewRedisStringCache creates a typed string Redis cache.
func NewRedisStringCache(opts ...redis.Option) (*TypedStringCache, error) {
	rc, err := redis.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewTypedStringCache(rc), nil
}

// TypedBytesCache provides type-safe []byte operations.
type TypedBytesCache struct{ *TypedCache[[]byte] }

// NewTypedBytesCache wraps any CoreCache with []byte type safety.
func NewTypedBytesCache(c CoreCache) *TypedBytesCache {
	return &TypedBytesCache{NewTypedCache(c, BytesCodec{})}
}
