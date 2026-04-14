package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/internal/keyutil"
	"github.com/os-gomod/cache/internal/pooling"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/layer"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
)

// ---------------------------------------------------------------------------
// TypedCache[T] — first-class typed cache API
// ---------------------------------------------------------------------------

// TypedCache wraps a Backend with a codec to provide type-safe cache
// operations. It does NOT implement Backend directly (because Get
// returns T, not []byte), but delegates all operations to the underlying
// backend with automatic Encode/Decode wrapping.
//
// The encoding buffer pool reduces allocations on the hot path. For codecs
// that support scratch-buffer reuse (Int64Codec, Float64Codec), the pool
// provides a pre-allocated []byte that Encode can write into.
type TypedCache[T any] struct {
	backend    Backend
	codec      codec.Codec[T]
	sg         *singlefght.Group
	bufPool    *pooling.BufPool
	onSetError func(key string, err error)
}

// NewTypedCache creates a typed cache that wraps b with the given codec.
func NewTypedCache[T any](
	b Backend,
	c codec.Codec[T],
	opts ...func(*TypedCache[T]),
) *TypedCache[T] {
	tc := &TypedCache[T]{
		backend: b,
		codec:   c,
		sg:      singlefght.NewGroup(),
		bufPool: pooling.NewBufPool(64),
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

// Backend returns the underlying backend.
func (tc *TypedCache[T]) Backend() Backend { return tc.backend }

// Codec returns the codec used by this typed cache.
func (tc *TypedCache[T]) Codec() codec.Codec[T] { return tc.codec }

// Name returns the underlying backend's name prefixed with "typed:".
func (tc *TypedCache[T]) Name() string { return "typed:" + tc.backend.Name() }

// encode uses the codec with a pooled scratch buffer.
func (tc *TypedCache[T]) encode(v T) ([]byte, error) {
	bp := tc.bufPool.Get()
	scratch := *bp
	data, err := tc.codec.Encode(v, scratch)
	if err != nil {
		tc.bufPool.Put(bp)
		return nil, err
	}
	// If the codec wrote into our scratch buffer, copy out before
	// returning the buffer to the pool.
	if len(data) > 0 && len(scratch) > 0 && &data[0] == &scratch[0] {
		out := make([]byte, len(data))
		copy(out, data)
		tc.bufPool.Put(bp)
		return out, nil
	}
	// Codec returned its own allocation — return scratch buffer.
	tc.bufPool.Put(bp)
	return data, nil
}

// ---------------------------------------------------------------------------
// Core KV
// ---------------------------------------------------------------------------

// Get retrieves a value and decodes it with the codec.
func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	if err := keyutil.ValidateKey("typed.get", key); err != nil {
		return zero, err
	}
	data, err := tc.backend.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	return tc.codec.Decode(data)
}

// Set encodes value with the codec and stores it.
func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	if err := keyutil.ValidateKey("typed.set", key); err != nil {
		return err
	}
	data, err := tc.encode(value)
	if err != nil {
		return fmt.Errorf("typed.set encode: %w", err)
	}
	return tc.backend.Set(ctx, key, data, ttl)
}

// Delete removes a key from the cache.
func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	if err := keyutil.ValidateKey("typed.delete", key); err != nil {
		return err
	}
	return tc.backend.Delete(ctx, key)
}

// Exists checks whether a key exists in the cache.
func (tc *TypedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	if err := keyutil.ValidateKey("typed.exists", key); err != nil {
		return false, err
	}
	return tc.backend.Exists(ctx, key)
}

// TTL returns the remaining time-to-live for a key.
func (tc *TypedCache[T]) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := keyutil.ValidateKey("typed.ttl", key); err != nil {
		return 0, err
	}
	return tc.backend.TTL(ctx, key)
}

// ---------------------------------------------------------------------------
// Batch
// ---------------------------------------------------------------------------

// GetMulti retrieves multiple values and decodes them.
func (tc *TypedCache[T]) GetMulti(ctx context.Context, keys ...string) (map[string]T, error) {
	for _, k := range keys {
		if err := keyutil.ValidateKey("typed.get_multi", k); err != nil {
			return nil, err
		}
	}
	raw, err := tc.backend.GetMulti(ctx, keys...)
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

// SetMulti encodes and stores multiple values.
func (tc *TypedCache[T]) SetMulti(
	ctx context.Context,
	items map[string]T,
	ttl time.Duration,
) error {
	raw := make(map[string][]byte, len(items))
	for k, v := range items {
		if err := keyutil.ValidateKey("typed.set_multi", k); err != nil {
			return err
		}
		data, err := tc.encode(v)
		if err != nil {
			return fmt.Errorf("typed.set_multi encode key=%s: %w", k, err)
		}
		raw[k] = data
	}
	return tc.backend.SetMulti(ctx, raw, ttl)
}

// DeleteMulti removes multiple keys.
func (tc *TypedCache[T]) DeleteMulti(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if err := keyutil.ValidateKey("typed.delete_multi", k); err != nil {
			return err
		}
	}
	return tc.backend.DeleteMulti(ctx, keys...)
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Ping checks the underlying backend's health.
func (tc *TypedCache[T]) Ping(ctx context.Context) error { return tc.backend.Ping(ctx) }

// Close closes the underlying backend.
func (tc *TypedCache[T]) Close(ctx context.Context) error { return tc.backend.Close(ctx) }

// Stats returns the underlying backend's statistics.
func (tc *TypedCache[T]) Stats() stats.Snapshot { return tc.backend.Stats() }

// Closed reports whether the underlying backend is closed.
func (tc *TypedCache[T]) Closed() bool { return tc.backend.Closed() }

// ---------------------------------------------------------------------------
// Extended typed operations
// ---------------------------------------------------------------------------

// GetOrSet combines Get with a loader function, using singleflight to
// prevent cache stampede.
func (tc *TypedCache[T]) GetOrSet(
	ctx context.Context,
	key string,
	fn func() (T, error),
	ttl time.Duration,
) (T, error) {
	var zero T
	if err := keyutil.ValidateKey("typed.get_or_set", key); err != nil {
		return zero, err
	}
	data, err := tc.sg.Do(ctx, key, func() ([]byte, error) {
		// Try the cache first.
		if d, getErr := tc.backend.Get(ctx, key); getErr == nil {
			return d, nil
		}
		// Cache miss — call the loader.
		v, fnErr := fn()
		if fnErr != nil {
			return nil, fnErr
		}
		encoded, encErr := tc.encode(v)
		if encErr != nil {
			return nil, encErr
		}
		if setErr := tc.backend.Set(ctx, key, encoded, ttl); setErr != nil {
			if tc.onSetError != nil {
				tc.onSetError(key, setErr)
			}
		}
		return encoded, nil
	})
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

// CompareAndSwap atomically swaps a value if the old value matches.
// Requires the underlying backend to implement AtomicBackend.
func (tc *TypedCache[T]) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal T,
	ttl time.Duration,
) (bool, error) {
	if err := keyutil.ValidateKey("typed.cas", key); err != nil {
		return false, err
	}
	ab, ok := tc.backend.(AtomicBackend)
	if !ok {
		return false, fmt.Errorf(
			"typed.cas: backend %q does not implement AtomicBackend",
			tc.backend.Name(),
		)
	}
	oldData, err := tc.encode(oldVal)
	if err != nil {
		return false, fmt.Errorf("typed.cas encode old: %w", err)
	}
	newData, err := tc.encode(newVal)
	if err != nil {
		return false, fmt.Errorf("typed.cas encode new: %w", err)
	}
	return ab.CompareAndSwap(ctx, key, oldData, newData, ttl)
}

// SetNX sets a value only if the key does not exist.
// Requires the underlying backend to implement AtomicBackend.
func (tc *TypedCache[T]) SetNX(
	ctx context.Context,
	key string,
	value T,
	ttl time.Duration,
) (bool, error) {
	if err := keyutil.ValidateKey("typed.setnx", key); err != nil {
		return false, err
	}
	ab, ok := tc.backend.(AtomicBackend)
	if !ok {
		return false, fmt.Errorf(
			"typed.setnx: backend %q does not implement AtomicBackend",
			tc.backend.Name(),
		)
	}
	data, err := tc.encode(value)
	if err != nil {
		return false, fmt.Errorf("typed.setnx encode: %w", err)
	}
	return ab.SetNX(ctx, key, data, ttl)
}

// GetSet atomically sets a new value and returns the old value.
// Requires the underlying backend to implement AtomicBackend.
func (tc *TypedCache[T]) GetSet(
	ctx context.Context,
	key string,
	value T,
	ttl time.Duration,
) (T, error) {
	var zero T
	if err := keyutil.ValidateKey("typed.getset", key); err != nil {
		return zero, err
	}
	ab, ok := tc.backend.(AtomicBackend)
	if !ok {
		return zero, fmt.Errorf(
			"typed.getset: backend %q does not implement AtomicBackend",
			tc.backend.Name(),
		)
	}
	data, err := tc.encode(value)
	if err != nil {
		return zero, fmt.Errorf("typed.getset encode: %w", err)
	}
	oldData, err := ab.GetSet(ctx, key, data, ttl)
	if err != nil {
		return zero, err
	}
	if oldData == nil {
		return zero, nil
	}
	return tc.codec.Decode(oldData)
}

// Increment increments a numeric key.
// Requires the underlying backend to implement AtomicBackend.
//
//nolint:dupl // Increment/Decrement keep separate method names for API clarity.
func (tc *TypedCache[T]) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := keyutil.ValidateKey("typed.increment", key); err != nil {
		return 0, err
	}
	ab, ok := tc.backend.(AtomicBackend)
	if !ok {
		return 0, fmt.Errorf(
			"typed.increment: backend %q does not implement AtomicBackend",
			tc.backend.Name(),
		)
	}
	return ab.Increment(ctx, key, delta)
}

// Decrement decrements a numeric key.
// Requires the underlying backend to implement AtomicBackend.
//
//nolint:dupl // Increment/Decrement keep separate method names for API clarity.
func (tc *TypedCache[T]) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	if err := keyutil.ValidateKey("typed.decrement", key); err != nil {
		return 0, err
	}
	ab, ok := tc.backend.(AtomicBackend)
	if !ok {
		return 0, fmt.Errorf(
			"typed.decrement: backend %q does not implement AtomicBackend",
			tc.backend.Name(),
		)
	}
	return ab.Decrement(ctx, key, delta)
}

// Keys returns all keys matching the pattern.
// Requires the underlying backend to implement ScanBackend.
func (tc *TypedCache[T]) Keys(ctx context.Context, pattern string) ([]string, error) {
	sb, ok := tc.backend.(ScanBackend)
	if !ok {
		return nil, fmt.Errorf(
			"typed.keys: backend %q does not implement ScanBackend",
			tc.backend.Name(),
		)
	}
	return sb.Keys(ctx, pattern)
}

// Clear removes all keys.
// Requires the underlying backend to implement ScanBackend.
func (tc *TypedCache[T]) Clear(ctx context.Context) error {
	sb, ok := tc.backend.(ScanBackend)
	if !ok {
		return fmt.Errorf(
			"typed.clear: backend %q does not implement ScanBackend",
			tc.backend.Name(),
		)
	}
	return sb.Clear(ctx)
}

// Size returns the number of items in the cache.
// Requires the underlying backend to implement ScanBackend.
func (tc *TypedCache[T]) Size(ctx context.Context) (int64, error) {
	sb, ok := tc.backend.(ScanBackend)
	if !ok {
		return 0, fmt.Errorf(
			"typed.size: backend %q does not implement ScanBackend",
			tc.backend.Name(),
		)
	}
	return sb.Size(ctx)
}

// ---------------------------------------------------------------------------
// TypedCache option functions
// ---------------------------------------------------------------------------

// WithOnSetError sets a callback for set errors on the typed cache.
func WithOnSetError[T any](fn func(key string, err error)) func(*TypedCache[T]) {
	return func(tc *TypedCache[T]) { tc.onSetError = fn }
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

// NewJSONTypedCache creates a typed cache with JSON encoding.
func NewJSONTypedCache[T any](b Backend, opts ...func(*TypedCache[T])) *TypedCache[T] {
	return NewTypedCache(b, codec.NewJSONCodec[T](), opts...)
}

// NewRawTypedCache creates a typed cache for raw []byte values (zero-copy).
func NewRawTypedCache(b Backend, opts ...func(*TypedCache[[]byte])) *TypedCache[[]byte] {
	return NewTypedCache(b, codec.RawCodec{}, opts...)
}

// NewStringTypedCache creates a typed cache for string values (zero-alloc decode).
func NewStringTypedCache(b Backend, opts ...func(*TypedCache[string])) *TypedCache[string] {
	return NewTypedCache(b, codec.StringCodec{}, opts...)
}

// NewInt64TypedCache creates a typed cache for int64 values.
func NewInt64TypedCache(b Backend, opts ...func(*TypedCache[int64])) *TypedCache[int64] {
	return NewTypedCache(b, codec.Int64Codec{}, opts...)
}

// NewFloat64TypedCache creates a typed cache for float64 values.
func NewFloat64TypedCache(b Backend, opts ...func(*TypedCache[float64])) *TypedCache[float64] {
	return NewTypedCache(b, codec.Float64Codec{}, opts...)
}

// ---------------------------------------------------------------------------
// Legacy type aliases for backward compatibility
// ---------------------------------------------------------------------------

// TypedInt64Cache is a typed cache for int64 values.
type TypedInt64Cache = TypedCache[int64]

// TypedStringCache is a typed cache for string values.
type TypedStringCache = TypedCache[string]

// TypedBytesCache is a typed cache for raw []byte values.
type TypedBytesCache = TypedCache[[]byte]

// NewTypedInt64Cache creates a typed cache for int64 values.
func NewTypedInt64Cache(b Backend, opts ...func(*TypedCache[int64])) *TypedCache[int64] {
	return NewInt64TypedCache(b, opts...)
}

// NewMemoryInt64Cache creates a memory-backed int64 cache.
func NewMemoryInt64Cache(opts ...memory.Option) (*TypedCache[int64], error) {
	mc, err := memory.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewInt64TypedCache(mc), nil
}

// NewRedisInt64Cache creates a redis-backed int64 cache.
func NewRedisInt64Cache(opts ...redis.Option) (*TypedCache[int64], error) {
	rc, err := redis.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewInt64TypedCache(rc), nil
}

// NewMemoryStringCache creates a memory-backed string cache.
func NewMemoryStringCache(opts ...memory.Option) (*TypedCache[string], error) {
	mc, err := memory.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewStringTypedCache(mc), nil
}

// NewRedisStringCache creates a redis-backed string cache.
func NewRedisStringCache(opts ...redis.Option) (*TypedCache[string], error) {
	rc, err := redis.New(opts...)
	if err != nil {
		return nil, err
	}
	return NewStringTypedCache(rc), nil
}

// NewMemoryTypedCache creates a memory-backed JSON-typed cache.
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

// NewRedisTypedCache creates a redis-backed JSON-typed cache.
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

// NewLayeredTypedCache creates a layered-backed JSON-typed cache.
func NewLayeredTypedCache[T any](
	layeredOpts []layer.Option,
	opts ...func(*TypedCache[T]),
) (*TypedCache[T], error) {
	lc, err := layer.New(layeredOpts...)
	if err != nil {
		return nil, err
	}
	return NewJSONTypedCache[T](lc, opts...), nil
}
