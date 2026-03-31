# cache

A high-performance, multi-tier caching library for Go. It provides in-memory, Redis, and layered (L1 memory + L2 Redis) caches with pluggable eviction policies, configuration options, and advanced features like singleflight, circuit breakers, and rate limiting.

---

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Cache Types](#cache-types)
  - [Configuration](#configuration)
  - [Key Building](#key-building)
- [Usage Examples](#usage-examples)
  - [Memory Cache](#memory-cache)
  - [Redis Cache](#redis-cache)
  - [Layered Cache (L1 + L2)](#layered-cache-l1--l2)
  - [Builder Pattern](#builder-pattern)
  - [Typed Caches](#typed-caches)
  - [Resilience (Circuit Breaker & Rate Limiter)](#resilience-circuit-breaker--rate-limiter)
  - [Eviction Policies](#eviction-policies)
- [Advanced Features](#advanced-features)
  - [Singleflight (Cache Stampede Protection)](#singleflight-cache-stampede-protection)
  - [Distributed Stampede Protection](#distributed-stampede-protection)
  - [Write-Back Mode](#write-back-mode)
  - [Cache Synchronization (Pub/Sub)](#cache-synchronization-pubsub)
- [Monitoring and Stats](#monitoring-and-stats)
- [Observability](#observability)
- [Error Handling](#error-handling)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **Multiple Cache Backends**: In-memory, Redis, and a layered combination.
- **Rich Eviction Policies**: LRU, LFU, FIFO, LIFO, MRU, Random, and TinyLFU.
- **Configurable**: Fine-grained control over TTLs, memory limits, entry counts, sharding, and more.
- **Type Safety**: Generic `TypedCache` for working with Go structs, strings, ints, and bytes.
- **Concurrency Safe**: All caches are safe for concurrent use.
- **Cache Stampede Protection**: Singleflight for in-memory operations and optional distributed locking for Redis.
- **Resilience Patterns**: Built-in circuit breaker and rate limiter wrappers.
- **Performance**: Lock-free statistics, sharded memory cache, and optimized eviction algorithms.
- **Observability**: Pluggable tracing, logging, and metrics with no-op defaults.
- **Structured Errors**: Rich error types with codes, metadata, and cause chains.
- **Layered Cache Features**: L1 promotion, negative caching, write-back mode, and Pub/Sub invalidation.

---

## Getting Started

### Installation

```bash
go get github.com/os-gomod/cache
```

### Quick Start

Here's a basic example of creating and using an in-memory cache.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/os-gomod/cache"
)

func main() {
    ctx := context.Background()

    // Create a memory cache
    c, err := cache.NewMemory()
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close(ctx)

    // Set a value
    err = c.Set(ctx, "greeting", []byte("Hello, World!"), 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Get a value
    val, err := c.Get(ctx, "greeting")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(val)) // Output: Hello, World!
}
```

---

## Core Concepts

### Cache Types

The library offers three primary cache types, all implementing the `CoreCache` interface:

- **`memory.Cache`**: A fast, sharded in-process cache with pluggable eviction policies.
- **`redis.Cache`**: A Redis-backed cache, supporting standalone and cluster modes.
- **`layered.Cache`**: A two-level cache that uses an in-memory L1 (fast) and a Redis L2 (persistent, shared), with optional write-back and cross-instance invalidation.

### Configuration

Each cache type can be configured using:

- **Functional Options**: Provide a fluent, immutable builder API.
- **Config Structs**: Offer a way to define the entire configuration as a single object, useful for loading from a file or environment variables.
- **Builder Pattern**: A unified entry point for constructing all cache types with an immutable, chainable API.

### Key Building

The `builder.KeyBuilder` provides a convenient way to create hierarchical, colon-delimited keys with validation.

```go
import "github.com/os-gomod/cache/builder"

// Create a validated key builder
kb, err := builder.NewKey("api")
if err != nil {
    log.Fatal(err)
}

key := kb.
    MustAdd("v2").
    MustAdd("users").
    MustAdd("123").
    Build()
// key is "api:v2:users:123"
```

The builder validates segments for length (max 512 characters total) and rejects control characters or whitespace.

---

## Usage Examples

### Memory Cache

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/os-gomod/cache"
    "github.com/os-gomod/cache/memory"
)

func main() {
    ctx := context.Background()

    // Create a memory cache with options
    c, err := cache.NewMemory(
        memory.WithMaxEntries(1000),
        memory.WithMaxMemoryMB(50),
        memory.WithTTL(10*time.Minute),
        memory.WithLRU(),
        memory.WithShards(32),
        memory.WithOnEvictionPolicy(func(key, reason string) {
            fmt.Printf("Evicted: %s due to %s\n", key, reason)
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close(ctx)

    // Use the cache...
    _ = c.Set(ctx, "key", []byte("value"), 0)
    val, _ := c.Get(ctx, "key")
    fmt.Println(string(val)) // Output: value
}
```

### Redis Cache

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/os-gomod/cache"
    "github.com/os-gomod/cache/redis"
)

func main() {
    ctx := context.Background()

    // Create a Redis cache
    c, err := cache.NewRedis(
        redis.WithAddress("localhost:6379"),
        redis.WithPassword(""),
        redis.WithDB(0),
        redis.WithKeyPrefix("myapp:"),
        redis.WithTTL(time.Hour),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close(ctx)

    // Use Redis-specific features
    _ = c.HSet(ctx, "user:1", "name", "Alice")
    name, _ := c.HGet(ctx, "user:1", "name")
    log.Println(string(name)) // Output: Alice
}
```

### Layered Cache (L1 + L2)

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/os-gomod/cache"
    "github.com/os-gomod/cache/layered"
)

func main() {
    ctx := context.Background()

    // Create a layered cache
    c, err := cache.NewLayered(
        layered.WithL1MaxEntries(1000),
        layered.WithL1TTL(2*time.Minute),
        layered.WithL1LRU(),
        layered.WithL2Address("localhost:6379"),
        layered.WithL2TTL(time.Hour),
        layered.WithPromoteOnHit(true),
        layered.WithNegativeTTL(30*time.Second), // Cache negative lookups
    )
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close(ctx)

    // Set a value (stored in L1 and L2)
    _ = c.Set(ctx, "key", []byte("value"), 0)
    // Get will serve from L1 if possible
    val, _ := c.Get(ctx, "key")
    log.Println(string(val))
}
```

### Builder Pattern

The `builder` package provides a unified and immutable way to construct caches.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/os-gomod/cache/builder"
)

func main() {
    ctx := context.Background()

    // Build a memory cache
    memCache, err := builder.New(ctx).
        Memory().
        MaxEntries(1000).
        MaxMemoryMB(50).
        LRU().
        Build()
    if err != nil {
        log.Fatal(err)
    }
    defer memCache.Close(ctx)

    // Build a Redis cache with a custom config struct
    redisCache, err := builder.New(ctx).
        Redis().
        Addr("localhost:6379").
        TTL(time.Hour).
        KeyPrefix("myapp:").
        Build()
    if err != nil {
        log.Fatal(err)
    }
    defer redisCache.Close(ctx)

    // Build a layered cache
    layeredCache, err := builder.New(ctx).
        Layered().
        L1MaxEntries(1000).
        L1TTL(2*time.Minute).
        L2Addr("localhost:6379").
        PromoteOnHit(true).
        WriteBack(true).      // Asynchronous writes to L2
        Build()
    if err != nil {
        log.Fatal(err)
    }
    defer layeredCache.Close(ctx)
}
```

### Typed Caches

The `TypedCache[T]` provides a type-safe wrapper around a `CoreCache`, handling serialization (JSON, string, int64, bytes) automatically.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/os-gomod/cache"
    "github.com/os-gomod/cache/memory"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

func main() {
    ctx := context.Background()

    // Create a typed cache for User structs using JSON serialization
    memCache, _ := memory.New()
    userCache := cache.NewJSONTypedCache[User](memCache)

    // Store a user
    user := User{ID: 1, Name: "Alice"}
    err := userCache.Set(ctx, "user:1", user, 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve the user
    retrieved, err := userCache.Get(ctx, "user:1")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("User: %+v\n", retrieved)

    // Use the built-in typed caches for primitives
    intCache := cache.NewTypedInt64Cache(memCache)
    _, _ = intCache.Increment(ctx, "counter", 1)
    count, _ := intCache.Get(ctx, "counter")
    fmt.Println("Counter:", count) // Output: Counter: 1
}
```

### Resilience (Circuit Breaker & Rate Limiter)

Wrap any `CoreCache` with `resilience.Cache` to add circuit breaking and rate limiting.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/os-gomod/cache"
    "github.com/os-gomod/cache/resilience"
)

func main() {
    ctx := context.Background()

    // Create a backend (e.g., Redis)
    backend, _ := cache.NewRedis()

    // Configure a circuit breaker
    cb := resilience.NewCircuitBreaker(5, 10*time.Second)

    // Configure a rate limiter with independent read/write limits
    limiter := resilience.NewLimiterWithConfig(resilience.LimiterConfig{
        ReadRPS:    100,
        ReadBurst:  20,
        WriteRPS:   50,
        WriteBurst: 10,
    })

    // Wrap the backend
    resilientCache := cache.NewResilient(backend, resilience.Options{
        CircuitBreaker: cb,
        Limiter:        limiter,
        Hooks: &resilience.Hooks{
            OnGet: func(ctx context.Context, key string, hit bool, errKind string, d time.Duration) {
                log.Printf("Get %s: hit=%v latency=%v", key, hit, d)
            },
        },
    })
    defer resilientCache.Close(ctx)

    // Use the cache as usual; it now has protection
    _, err := resilientCache.Get(ctx, "key")
    if err != nil {
        log.Println("Error (possibly circuit open or rate limited):", err)
    }
}
```

### Eviction Policies

The library supports several eviction policies for the memory cache.

```go
package main

import (
    "context"

    "github.com/os-gomod/cache/memory"
)

func main() {
    ctx := context.Background()

    // Create a cache with LFU (Least Frequently Used) eviction
    lfuCache, _ := memory.New(
        memory.WithMaxEntries(3),
        memory.WithLFU(),
    )
    defer lfuCache.Close(ctx)

    // Create a cache with TinyLFU
    tinylfuCache, _ := memory.New(
        memory.WithMaxEntries(1000),
        memory.WithTinyLFU(),
    )
    defer tinylfuCache.Close(ctx)

    // Use the caches...
    _ = lfuCache.Set(ctx, "key1", []byte("value1"), 0)
    _ = tinylfuCache.Set(ctx, "key2", []byte("value2"), 0)
}
```

---

## Advanced Features

### Singleflight (Cache Stampede Protection)

The `GetOrSet` method in all cache types uses a `singleflight` group to ensure that when many requests miss the cache for the same key at the same time, only one goroutine performs the expensive computation. All others wait for that single result.

```go
// memory.Cache, redis.Cache, and layered.Cache all have this method.
val, err := cache.GetOrSet(ctx, "key", func() ([]byte, error) {
    // This expensive function will be called only once.
    return computeSomethingExpensive(), nil
}, 10*time.Minute)
```

### Distributed Stampede Protection

For Redis, you can enable distributed stampede protection. When enabled, a Redis lock is used to coordinate computation across multiple application instances, ensuring that only one instance computes the value for a given key at a time.

```go
redisCache, _ := cache.NewRedis(
    redis.WithDistributedStampedeProtection(true),
    redis.WithStampedeLockTTL(5*time.Second),
    redis.WithStampedeWaitTimeout(2*time.Second),
)
```

### Write-Back Mode

The layered cache can be configured in write-back mode. Writes are written to the L1 cache immediately and then asynchronously written to L2 (Redis) in the background. This can significantly improve write latency and is ideal for write-heavy workloads.

```go
layeredCache, _ := cache.NewLayered(
    layered.WithWriteBack(true),
    layered.WithL1MaxEntries(1000),
    layered.WithL2Address("localhost:6379"),
    layered.WithWriteBackConfig(512, 4), // queue size, worker count
)
```

### Cache Synchronization (Pub/Sub)

For layered caches, you can enable synchronization. When an entry is updated in L2 (Redis), a message is published to a Redis channel. Other instances of the layered cache subscribe to this channel and invalidate the corresponding key in their L1 cache, keeping L1 caches consistent across a cluster.

```go
layeredCache, _ := cache.NewLayered(
    layered.WithSyncEnabled(true),
    layered.WithSyncChannel("cache:invalidate"),
    layered.WithSyncBufferSize(1000),
    layered.WithL2Address("localhost:6379"),
)
```

### Negative Caching

The layered cache can store negative results (e.g., "key not found") with a configurable TTL. This prevents repeated cache misses from hitting the backend and is controlled by the `NegativeTTL` setting.

```go
layeredCache, _ := cache.NewLayered(
    layered.WithNegativeTTL(30*time.Second),
    layered.WithL2Address("localhost:6379"),
)
```

---

## Monitoring and Stats

All cache implementations provide a `Stats()` method that returns a `Snapshot` of cache statistics.

```go
stats := layeredCache.Stats()
fmt.Println("Hits:", stats.Hits)
fmt.Println("Misses:", stats.Misses)
fmt.Println("Hit Rate:", stats.HitRate, "%")
fmt.Println("Items:", stats.Items)
fmt.Println("Memory:", stats.Memory, "bytes")
fmt.Println("Uptime:", stats.Uptime)
```

For layered caches, the `Snapshot` also includes per-layer stats:

```go
fmt.Println("L1 Hits:", stats.L1Hits)
fmt.Println("L1 Hit Rate:", stats.L1HitRate, "%")
fmt.Println("L2 Hits:", stats.L2Hits)
fmt.Println("L2 Promotions:", stats.L2Promotions)

// Write-back stats (if enabled)
fmt.Println("Write-back Enqueued:", stats.WriteBackEnqueued)
fmt.Println("Write-back Flushed:", stats.WriteBackFlushed)
fmt.Println("Write-back Dropped:", stats.WriteBackDropped)
```

---

## Observability

The library includes a pluggable observability system (`internal/obs`) that provides tracing, logging, and metrics hooks. By default, these are no-op, but you can inject your own implementations at application startup.

```go
import "github.com/os-gomod/cache/internal/obs"

// Set a custom provider
obs.SetProvider(&obs.Provider{
    Tracer:  myTracer,
    Logger:  myLogger,
    Metrics: myMetrics,
})
```

The package-level helpers (`obs.Start`, `obs.Info`, `obs.RecordHit`, etc.) are used throughout the library, and all cache operations will emit observability data when configured.

---

## Error Handling

The library provides structured errors with codes, metadata, and cause chains.

```go
import _errors "github.com/os-gomod/cache/errors"

// Check for specific errors
if _errors.IsNotFound(err) {
    // Handle cache miss
}

if _errors.IsTimeout(err) {
    // Handle timeout
}

// Extract error code
code := _errors.CodeOf(err)

// Check if error is retryable (timeout, connection, rate limit)
if _errors.Retryable(err) {
    // Retry the operation
}

// Get typed error with metadata
if ce, ok := _errors.AsType(err); ok {
    fmt.Printf("Operation: %s, Key: %s, Code: %s\n", 
        ce.Operation, ce.Key, ce.Code)
    
    // Access metadata
    if traceID, ok := ce.Metadata["trace_id"]; ok {
        fmt.Println("Trace ID:", traceID)
    }
}
```

**Available error codes:**

- `CodeNotFound` – Key not found
- `CodeCacheClosed` – Cache has been closed
- `CodeInvalidKey` – Key is invalid (empty, contains invalid characters)
- `CodeTimeout` – Operation timed out
- `CodeConnection` – Connection error
- `CodeRateLimited` – Rate limit exceeded
- `CodeCircuitOpen` – Circuit breaker is open
- `CodeSerialize` / `CodeDeserialize` – Serialization errors

---

## Development

The project includes a comprehensive Makefile for development tasks.

```bash
# Install dependencies
make deps

# Run linting
make lint

# Run tests with race detector
make test

# Run tests with coverage
make coverage

# Run benchmarks
make benchmark

# Start Redis with Docker (for integration testing)
make docker-up

# Full validation (deps, fmt, vet, lint, test, coverage)
make validate

# Clean build artifacts
make clean
```

### Development Tools

```bash
# Install all development tools
make install-tools
```

---

## Contributing

Contributions are welcome! Please ensure that your changes pass all quality gates:

- `make validate` – Full validation pipeline
- `make lint` – Code style and static analysis
- `make test` – Test suite with race detection
- `make coverage` – Coverage report (threshold: 30%)

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

**Maintained by [OS-Gomod](https://github.com/os-gomod)** | [Report Bug](https://github.com/os-gomod/cache/issues) | [Request Feature](https://github.com/os-gomod/cache/issues)
