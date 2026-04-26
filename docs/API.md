# Public API Reference

This document provides a comprehensive reference for the public API of `os-gomod/cache/v2`.

---

## Package: `cache` (`github.com/os-gomod/cache/v2`)

The primary public API surface. Import as:

```go
import "github.com/os-gomod/cache/v2"
```

---

## Core Types

### Backend Interface

`Backend` is the primary cache interface. It composes all capability interfaces into a single type:

```go
type Backend interface {
    contracts.Reader        // Get, GetMulti, Exists, TTL
    contracts.Writer        // Set, SetMulti, Delete, DeleteMulti
    contracts.AtomicOps     // CompareAndSwap, SetNX, Increment, Decrement, GetSet
    contracts.Scanner       // Keys, Clear, Size
    contracts.Lifecycle     // Ping, Close, Closed, Name
    contracts.StatsProvider // Stats()
}
```

All backend implementations returned by `NewMemory`, `NewRedis`, and `NewLayered` satisfy this interface.

### TypedCache[T]

Type-safe cache wrapper for a specific value type `T`:

```go
type TypedCache[T any] struct { ... }
```

#### Construction

```go
// Generic constructor with custom codec
tc := cache.NewTyped[User](backend, serialization.NewJSONCodec[User]())

// Convenience constructors
tc, err := cache.NewMemoryJSON[User](cache.WithMaxEntries(1000))
tc, err := cache.NewMemoryString(cache.WithMaxEntries(1000))
tc, err := cache.NewMemoryInt64(cache.WithMaxEntries(1000))
tc, err := cache.NewRedisJSON[User](redis.WithAddr("localhost:6379"))
tc, err := cache.NewRedisString(redis.WithAddr("localhost:6379"))
tc, err := cache.NewLayeredJSON[User](layered.WithL1(...), layered.WithL2(...))
```

#### Read Operations

| Method | Signature | Description |
|--------|-----------|-------------|
| `Get` | `(ctx, key) (T, error)` | Retrieve and decode a single value |
| `GetMulti` | `(ctx, ...keys) (map[string]T, error)` | Batch retrieve multiple values |
| `Exists` | `(ctx, key) (bool, error)` | Check if key exists |
| `TTL` | `(ctx, key) (time.Duration, error)` | Get remaining time-to-live |
| `GetOrSet` | `(ctx, key, fn, ttl) (T, error)` | Get or load-and-cache (cache-aside pattern) |

#### Write Operations

| Method | Signature | Description |
|--------|-----------|-------------|
| `Set` | `(ctx, key, value, ttl) error` | Store a value |
| `SetMulti` | `(ctx, items, ttl) error` | Batch store multiple values |
| `Delete` | `(ctx, key) error` | Remove a key |
| `DeleteMulti` | `(ctx, ...keys) error` | Batch remove multiple keys |

#### Atomic Operations

| Method | Signature | Description |
|--------|-----------|-------------|
| `CompareAndSwap` | `(ctx, key, old, new, ttl) (bool, error)` | Atomic compare-and-swap |
| `SetNX` | `(ctx, key, value, ttl) (bool, error)` | Set if not exists |
| `GetSet` | `(ctx, key, value, ttl) (T, error)` | Set and return old value |
| `Increment` | `(ctx, key, delta) (int64, error)` | Atomic increment |
| `Decrement` | `(ctx, key, delta) (int64, error)` | Atomic decrement |

#### Scan & Lifecycle

| Method | Signature | Description |
|--------|-----------|-------------|
| `Keys` | `(ctx, pattern) ([]string, error)` | List keys matching pattern |
| `Clear` | `(ctx) error` | Remove all entries |
| `Size` | `(ctx) (int64, error)` | Approximate entry count |
| `Close` | `(ctx) error` | Graceful shutdown |
| `Ping` | `(ctx) error` | Health check |
| `Stats` | `() StatsSnapshot` | Statistics snapshot |
| `Closed` | `() bool` | Check if closed |
| `Name` | `() string` | Backend identifier |
| `Backend` | `() Backend` | Underlying raw backend |

---

## Manager

Multi-backend cache manager:

```go
mgr, err := cache.NewManager(
    cache.WithNamedBackend("primary", primaryCache),
    cache.WithNamedBackend("sessions", sessionCache),
    cache.WithDefaultBackend(primaryCache),
)
defer mgr.Close(ctx)
```

| Method | Signature | Description |
|--------|-----------|-------------|
| `Backend` | `(name) (Backend, error)` | Get named backend |
| `Default` | `() (Backend, error)` | Get default backend |
| `Get` | `(ctx, key) ([]byte, error)` | Get from default backend |
| `Set` | `(ctx, key, value, ttl) error` | Set on default backend |
| `Delete` | `(ctx, key) error` | Delete from default backend |
| `HealthCheck` | `(ctx) map[string]error` | Ping all backends |
| `Stats` | `() map[string]StatsSnapshot` | Stats from all backends |
| `Close` | `(ctx) error` | Close all backends |
| `Closed` | `() bool` | Check if manager is closed |

### Manager Options

| Option | Description |
|--------|-------------|
| `WithDefaultBackend(b)` | Set the default backend |
| `WithNamedBackend(name, b)` | Register a named backend |
| `WithMiddleware(mws...)` | Apply middleware to all backends |
| `WithResilience(opts...)` | Apply resilience stack to all backends |

---

## Namespace

Key-prefixed namespace isolation:

```go
ns, err := cache.NewNamespace("tenant:a", backend)
ns.Set(ctx, "key", value, ttl)  // stores "tenant:a:key"
```

`Namespace` implements the full `Backend` interface, transparently prepending the prefix to all keys. Nested namespaces are supported for hierarchical isolation (e.g., `"app:tenant:a"`).

---

## Resilience Helpers

### WithResilience

Applies retry + circuit breaker + rate limiter:

```go
wrapped := cache.WithResilience(backend,
    cache.WithRetry(3, 100*time.Millisecond),
    cache.WithCircuitBreaker(5, 30*time.Second),
    cache.WithRateLimit(1000, 100),
)
```

### WithMiddleware

Apply arbitrary middleware chain:

```go
wrapped := cache.WithMiddleware(backend,
    middleware.MetricsMiddleware(recorder),
    middleware.RetryMiddleware(cfg),
)
```

### Resilience Options

| Option | Parameters | Description |
|--------|-----------|-------------|
| `WithRetry` | `maxAttempts int, initialDelay time.Duration` | Exponential backoff with jitter |
| `WithCircuitBreaker` | `threshold int, timeout time.Duration` | Opens after N failures |
| `WithRateLimit` | `rate float64, burst int` | Token bucket rate limiter |

---

## Enterprise Extensions

### Warmer

Pre-populate caches from external data sources:

```go
warmer := cache.NewWarmer(backend, loaderFunc,
    cache.WithWarmerBatchSize(100),
    cache.WithWarmerConcurrency(10),
)
err := warmer.Warm(ctx, "key1", "key2", "key3")
err := warmer.WarmAll(ctx, sourceFunc)
```

### HotKeyDetector

Detect frequently accessed keys:

```go
detector := cache.NewHotKeyDetector(
    cache.WithHotKeyThreshold(100),
    cache.WithHotKeyCallback(func(key string, count int64) {
        log.Printf("hot key: %s (%d)", key, count)
    }),
)
detector.Record(key)
if detector.IsHot(key) { /* react */ }
top := detector.TopKeys(10)
```

### AdaptiveTTL

Dynamically adjust TTL based on access frequency:

```go
adaptive := cache.NewAdaptiveTTL(30*time.Second, 10*time.Minute)
ttl := adaptive.TTL("user:123", 5*time.Minute)
adaptive.RecordAccess("user:123")
```

### Compression

Transparent value compression:

```go
gzipComp := cache.NewGzipCompressor(6)
cm := cache.NewCompressionMiddleware(gzipComp, 1024) // min 1KB
compressed, _ := cm.Compress(largeValue)
original, _ := cm.Decompress(compressed)
```

Available compressors: `GzipCompressor` (configurable level), `SnappyCompressor` (high speed).

---

## Backend Constructors

| Constructor | Signature | Description |
|-------------|-----------|-------------|
| `NewMemory` | `(opts ...Option) (Backend, error)` | In-process sharded cache |
| `NewRedis` | `(opts ...redis.Option) (Backend, error)` | Redis distributed cache |
| `NewLayered` | `(opts ...layered.Option) (Backend, error)` | Two-tier L1+L2 cache |

### Common Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithMaxEntries` | `int` | 0 (unlimited) | Maximum cache entries |
| `WithDefaultTTL` | `time.Duration` | 0 (no expiry) | Default TTL |
| `WithEvictionPolicy` | `string` | `"lru"` | Eviction algorithm |
| `WithShards` | `int` | 64 | Internal shard count |
| `WithKeyPrefix` | `string` | `""` | Key namespace prefix |
| `WithStatsEnabled` | `bool` | `true` | Enable statistics |

---

## Error Handling

All errors are created via `errors.Factory` and implement the `CacheError` interface:

```go
val, err := backend.Get(ctx, "key")
if errors.Factory.IsNotFound(err) {
    // key does not exist
}
```

Common error checking patterns:

```go
errors.Factory.IsNotFound(err)     // key not found
errors.Factory.IsClosed(err)       // backend is closed
errors.Factory.IsRateLimited(err)  // rate limit exceeded
errors.Factory.IsCircuitOpen(err)  // circuit breaker is open
```

---

## Quick Reference

| Task | Code |
|------|------|
| Create memory cache | `c, _ := cache.NewMemory(cache.WithMaxEntries(10000))` |
| Create typed cache | `tc, _ := cache.NewMemoryJSON[User]()` |
| Set value | `c.Set(ctx, "k", []byte("v"), ttl)` |
| Get value | `val, _ := c.Get(ctx, "k")` |
| Cache-aside | `val, _ := tc.GetOrSet(ctx, "k", loadFn, ttl)` |
| Multi-backend | `mgr, _ := cache.NewManager(cache.WithNamedBackend(...))` |
| Namespace | `ns, _ := cache.NewNamespace("prefix", c)` |
| Resilience | `c = cache.WithResilience(c, cache.WithRetry(...))` |
| Middleware | `c = cache.WithMiddleware(c, middleware.MetricsMiddleware(...))` |
| Cache warming | `w := cache.NewWarmer(c, loader); w.Warm(ctx, keys...)` |
| Hot-key detection | `d := cache.NewHotKeyDetector(); d.Record(key)` |
| Adaptive TTL | `a := cache.NewAdaptiveTTL(min, max); ttl := a.TTL(key, base)` |
| Compression | `cm := cache.NewCompressionMiddleware(cache.NewGzipCompressor(6), 1024)` |
