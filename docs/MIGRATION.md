# Migration Guide: v1 → v2

This guide helps you migrate from `os-gomod/cache v1` to `os-gomod/cache/v2`. Version 2 is a ground-up rewrite with a new architecture, capability-based interfaces, generics support, and enterprise-grade middleware. While some APIs are similar, there are significant breaking changes.

---

## Breaking Changes

### Module Path

```go
// v1
import "github.com/os-gomod/cache"

// v2
import "github.com/os-gomod/cache/v2"
```

Update all import paths to include the `/v2` major version suffix. This is required by Go's module system for major version bumps.

### Go Version Requirement

v2 requires **Go 1.22+** due to the use of generics (`TypedCache[T]`). If your project uses Go < 1.22, you must upgrade first.

---

## Interface Changes

### From Monolithic to Capability-Based

v1 used a single monolithic `Cache` interface:

```go
// v1
type Cache interface {
    Get(ctx, key) ([]byte, error)
    Set(ctx, key, value, ttl)
    Delete(ctx, key) error
    Close() error
    // ... 20+ methods
}
```

v2 decomposes into capability interfaces:

```go
// v2
type Backend interface {
    contracts.Reader        // Get, GetMulti, Exists, TTL
    contracts.Writer        // Set, SetMulti, Delete, DeleteMulti
    contracts.AtomicOps     // CompareAndSwap, SetNX, Increment, Decrement, GetSet
    contracts.Scanner       // Keys, Clear, Size
    contracts.Lifecycle     // Ping, Close, Closed, Name
    contracts.StatsProvider // Stats()
}
```

**Migration**: If your code only uses a subset of methods, update to reference the appropriate capability interface. For most use cases, the composite `Backend` interface (in `cache`) provides full compatibility.

### Context Parameter Added

All v2 methods require a `context.Context` as the first parameter:

```go
// v1
val, err := cache.Get("key")

// v2
val, err := cache.Get(ctx, "key")
```

**Migration**: Add `context.Background()` (or your request context) as the first argument to every cache method call.

---

## Constructor Changes

### Memory Cache

```go
// v1
c := cache.NewMemory()
c.SetMaxSize(10000)

// v2
c, err := cache.NewMemory(cache.WithMaxEntries(10000))
if err != nil {
    // handle error
}
```

Key differences:
- Constructors now return errors (fail fast on invalid configuration).
- Configuration is via functional options, not setter methods.
- `SetMaxSize` → `WithMaxEntries`
- Eviction policy is configurable: `WithEvictionPolicy("lfu")`

### Redis Cache

```go
// v1
c := cache.NewRedis("localhost:6379", "")

// v2
c, err := cache.NewRedis(
    redis.WithAddr("localhost:6379"),
    redis.WithPassword(""),
)
```

### Layered Cache

```go
// v1
c := cache.NewLayered(l1Cache, l2Cache)

// v2
c, err := cache.NewLayered(
    layered.WithL1(memory.WithMaxEntries(1000)),
    layered.WithL2(redis.WithAddr("localhost:6379")),
    layered.WithTTL(10 * time.Minute),
)
```

---

## Type-Safe API (New in v2)

v2 introduces `TypedCache[T]` for type-safe cache operations:

```go
// v1 (raw bytes)
data, err := cache.Get(ctx, "user:1")
var user User
json.Unmarshal(data, &user)

// v2 (type-safe)
tc, _ := cache.NewMemoryJSON[User]()
user, err := tc.Get(ctx, "user:1") // user is *User, no manual unmarshal
```

Specialized codecs for common types:

```go
// String cache (zero JSON overhead)
tc, _ := cache.NewMemoryString()
tc.Set(ctx, "greeting", "hello", time.Minute)

// Int64 cache (zero JSON overhead)
tc, _ := cache.NewMemoryInt64()
tc.Set(ctx, "counter", 42, 0)
newVal, _ := tc.Increment(ctx, "counter", 1)
```

---

## Middleware Migration

### v1 Middleware (Custom)

v1 had ad-hoc middleware patterns:

```go
// v1
c := cache.NewMemory()
c.Use(cache.Retry(3))
```

### v2 Middleware (Standardized)

v2 uses a composable middleware chain:

```go
// v2
c, _ := cache.NewMemory()
wrapped := cache.WithMiddleware(c,
    middleware.RetryMiddleware(middleware.RetryConfig{MaxAttempts: 3}),
    middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
        FailureThreshold: 5,
        Timeout:          30 * time.Second,
    }),
)
```

Or use the convenience `WithResilience` helper:

```go
wrapped := cache.WithResilience(c,
    cache.WithRetry(3, 100*time.Millisecond),
    cache.WithCircuitBreaker(5, 30*time.Second),
    cache.WithRateLimit(1000, 100),
)
```

---

## Error Handling Migration

### v1 Errors

```go
// v1
val, err := cache.Get(ctx, "key")
if err == cache.ErrNotFound {
    // handle not found
}
```

### v2 Errors

v2 uses structured errors via `ErrorFactory`:

```go
// v2
val, err := cache.Get(ctx, "key")
if errors.Factory.IsNotFound(err) {
    // handle not found
}

// Or use errors.Is with sentinel checking:
var cacheErr *errors.CacheError
if errors.As(err, &cacheErr) && cacheErr.Code == "NOT_FOUND" {
    // handle not found
}
```

Key differences:
- No more `fmt.Errorf` in application code — always use `ErrorFactory`.
- Errors carry operation context, codes, and chainable causes.
- Supports `errors.Is()` and `errors.As()` from the standard library.

---

## Config Changes

### Eviction Policy

```go
// v1 — LRU only
c := cache.NewMemory()

// v2 — configurable
c, _ := cache.NewMemory(cache.WithEvictionPolicy("tinylfu"))
// Options: "lru", "lfu", "fifo", "tinylfu"
```

### Statistics

```go
// v1
hits := cache.Stats().Hits

// v2
stats := cache.Stats()
fmt.Printf("Hits: %d, Hit Rate: %.2f%%\n",
    stats.Hits, stats.HitRate())
```

---

## New Features in v2

The following features are new in v2 and have no v1 equivalent:

| Feature | Package | Description |
|---------|---------|-------------|
| `TypedCache[T]` | `cache` | Type-safe generic cache wrapper |
| `Manager` | `cache` | Multi-backend cache manager |
| `Namespace` | `cache` | Key-prefixed namespace isolation |
| `Warmer` | `cache` | Cache warming with batch loading |
| `HotKeyDetector` | `cache` | Hot-key detection and monitoring |
| `AdaptiveTTL` | `cache` | Dynamic TTL based on access patterns |
| `Compression` | `cache` | Gzip and Snappy compression middleware |
| Circuit Breaker | `internal/middleware` | Automatic circuit breaking |
| Rate Limiter | `internal/middleware` | Token bucket rate limiting |
| Tracing | `internal/middleware` | OpenTelemetry integration |

---

## Timeline and Support Policy

| Version | Status | Support Until |
|---------|--------|---------------|
| v1.x | **Maintenance** | Security patches only, no new features |
| v2.x | **Active** | Full support, active development |
| v3.x | Future | Not yet planned |

**Recommendation**: New projects should use v2 directly. Existing v1 projects should plan migration within 6 months. v1 will receive critical security fixes for 12 months after v2 GA.

---

## Quick Migration Checklist

- [ ] Update `go.mod` to `github.com/os-gomod/cache/v2`
- [ ] Update all import paths to `/v2`
- [ ] Upgrade Go to 1.22+
- [ ] Add `context.Context` to all cache method calls
- [ ] Replace setters with functional options in constructors
- [ ] Handle constructor errors (`err != nil`)
- [ ] Replace `ErrNotFound` with `errors.Factory.IsNotFound(err)`
- [ ] Replace `fmt.Errorf` with `errors.Factory` methods
- [ ] Consider migrating to `TypedCache[T]` for type safety
- [ ] Update middleware to use the new `Middleware` chain API
- [ ] Run `go vet ./...` and `golangci-lint run ./...`
- [ ] Verify test coverage meets 90%+ threshold
