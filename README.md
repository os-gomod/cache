# os-gomod/cache v2

<p align="center">
  <strong>Enterprise-grade, high-performance caching platform for Go</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/os-gomod/cache/v2"><img src="https://pkg.go.dev/badge/github.com/os-gomod/cache/v2.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/os-gomod/cache/v2"><img src="https://goreportcard.com/badge/github.com/os-gomod/cache/v2" alt="Go Report Card"></a>
  <a href="https://github.com/os-gomod/cache/v2/actions"><img src="https://github.com/os-gomod/cache/v2/workflows/CI/badge.svg" alt="CI"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"></a>
</p>

## Features

- **Multi-backend**: Memory, Redis, Layered (L1 memory + L2 Redis)
- **Type-safe generics**: `TypedCache[T]` with pluggable codecs (JSON, String, Int64, Raw)
- **Unified execution runtime**: Zero code duplication across backends
- **Composable middleware**: Retry, circuit breaker, rate limiter, metrics, tracing, logging
- **Capability-based contracts**: Fine-grained interfaces (Reader, Writer, AtomicOps, Scanner, Lifecycle, StatsProvider)
- **Enterprise observability**: Prometheus metrics, OpenTelemetry tracing, structured logging
- **Pluggable eviction**: LRU, LFU, FIFO, TinyLFU
- **Resilience patterns**: Circuit breaker, exponential backoff retry, token bucket rate limiting
- **Enterprise extensions**: Cache warming, hot-key detection, adaptive TTL, compression (gzip, snappy)
- **Zero raw errors**: Structured `ErrorFactory` with machine-readable codes
- **90%+ test coverage**: Comprehensive unit tests, benchmarks, and integration tests

## Project Structure

```
os-gomod/cache/v2
├── *.go                   # Public API (root package: cache)
│   ├── cache.go           # Core Backend interface and constructors (NewMemory, NewRedis, NewLayered)
│   ├── typed.go           # TypedCache[T] generic type-safe cache
│   ├── manager.go         # Multi-backend cache manager
│   ├── namespace.go       # Cache namespace isolation
│   ├── options.go         # Shared option functions
│   ├── adaptive_ttl.go    # Adaptive TTL based on access patterns
│   ├── hotkey.go          # Hot-key detection and protection
│   ├── warming.go         # Cache warming strategies
│   ├── compression.go     # Value compression (gzip, snappy)
│   └── cache_test.go      # Unit tests
├── memory/                # In-memory cache store (L1)
│   ├── store.go           # Sharded memory store implementation
│   ├── basic.go           # Core CRUD operations
│   ├── advanced.go        # Batch, atomic, scan operations
│   ├── options.go         # Configuration options
│   ├── shard.go           # Concurrent sharding logic
│   ├── entry.go           # Cache entry data structure
│   ├── janitor.go         # Background expiration/cleanup
│   └── store_test.go      # Unit tests
├── redis/                 # Redis cache store (L2)
│   ├── store.go           # Redis store implementation
│   ├── basic.go           # Core CRUD operations
│   ├── advanced.go        # Batch, atomic, scan operations
│   ├── options.go         # Configuration options
│   ├── scripts.go         # Lua scripts for atomic operations
│   └── store_test.go      # Unit tests
├── layered/               # Layered cache (L1 memory + L2 Redis)
│   ├── store.go           # Layered store implementation
│   ├── basic.go           # Core CRUD with promotion
│   ├── advanced.go        # Batch, atomic, scan with L1/L2 coordination
│   ├── options.go         # Configuration options
│   ├── promotion.go       # L1 promotion policies
│   ├── writeback.go       # Async L2 write-back
│   └── store_test.go      # Unit tests
├── internal/              # Internal packages (not importable by external code)
│   ├── contracts/         # Capability-based interface contracts
│   ├── core/              # Core types (Key, Result)
│   ├── errors/            # Structured error factory
│   ├── middleware/         # Middleware pipeline implementations
│   ├── runtime/           # Unified execution runtime
│   ├── serialization/     # Codec system (JSON, MsgPack, zero-alloc)
│   ├── policy/            # Eviction, expiration, promotion, consistency policies
│   ├── config/            # Configuration parsing and validation
│   ├── instrumentation/  # Metrics, tracing, health, logging
│   ├── lifecycle/         # Lifecycle management (init, shutdown)
│   ├── stats/             # Statistics collector
│   ├── keyutil/           # Key validation and normalization
│   ├── hash/              # FNV hash implementation
│   └── testing/           # Test utilities, mocks, contract tests
├── benchmarks/            # Performance benchmarks
├── examples/              # Usage examples
├── docs/                  # Documentation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Quick Start

### Memory Cache

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/os-gomod/cache/v2"
)

func main() {
    ctx := context.Background()

    // Create a memory cache with options
    c, err := cache.NewMemory(
        cache.WithMaxEntries(10000),
        cache.WithDefaultTTL(10*time.Minute),
    )
    if err != nil {
        panic(err)
    }
    defer c.Close(ctx)

    // Set and Get
    c.Set(ctx, "greeting", []byte("Hello, World!"), 5*time.Minute)
    val, _ := c.Get(ctx, "greeting")
    fmt.Println(string(val)) // "Hello, World!"
}
```

### Typed Cache (Type-Safe)

```go
// String cache — zero JSON overhead
tc, _ := cache.NewMemoryString()
tc.Set(ctx, "name", "Alice", time.Minute)
name, _ := tc.Get(ctx, "name")
fmt.Println(name) // "Alice"

// JSON cache — for arbitrary structs
type User struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

uc, _ := cache.NewMemoryJSON[User]()
uc.Set(ctx, "user:1", User{ID: 1, Name: "Bob"}, 10*time.Minute)
user, _ := uc.Get(ctx, "user:1")
fmt.Printf("%+v\n", user) // {ID:1 Name:Bob}

// Cache-aside pattern
user, _ = uc.GetOrSet(ctx, "user:2", func() (User, error) {
    return loadUserFromDB(2) // only called on cache miss
}, 10*time.Minute)
```

### Resilience

```go
c, _ := cache.NewMemory()
resilient := cache.WithResilience(c,
    cache.WithRetry(3, 100*time.Millisecond),
    cache.WithCircuitBreaker(5, 30*time.Second),
    cache.WithRateLimit(1000, 100),
)
```

### Multi-Backend Manager

```go
mgr, _ := cache.NewManager(
    cache.WithNamedBackend("primary", primaryCache),
    cache.WithNamedBackend("sessions", sessionCache),
    cache.WithDefaultBackend(primaryCache),
)
defer mgr.Close(ctx)

// Use default backend
mgr.Set(ctx, "key", value, ttl)

// Health check all backends
health := mgr.HealthCheck(ctx)
```

## Installation

```bash
go get github.com/os-gomod/cache/v2
```

## Architecture

```
┌──────────────────────────────────────────────┐
│           Root Package (Public API)          │
│  import "github.com/os-gomod/cache/v2"       │
│                                              │
│  NewMemory  NewRedis  NewLayered             │
│  TypedCache[T]  Manager  Namespace           │
│  Warmer  HotKeyDetector  AdaptiveTTL         │
│  CompressionMiddleware                        │
├──────────────────────────────────────────────┤
│              Middleware Pipeline              │
│  Retry → CircuitBreaker → RateLimit →        │
│  Metrics → Tracing → Logging                  │
├──────────────────────────────────────────────┤
│           Store Implementations              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ memory/  │  │ redis/   │  │ layered/ │  │
│  │ (L1/LRU) │  │ (L2/dist)│  │ (L1+L2)  │  │
│  └──────────┘  └──────────┘  └──────────┘  │
├──────────────────────────────────────────────┤
│           internal/ (Private Packages)        │
│  contracts │ serialization │ errors │        │
│  policy    │ lifecycle      │ middleware     │
│  runtime   │ config         │ instrumentation│
└──────────────────────────────────────────────┘
```

See [Architecture Guide](docs/ARCHITECTURE.md) for the full design document.

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture Guide](docs/ARCHITECTURE.md) | System design, dependency graph, middleware pipeline, concurrency model |
| [API Reference](docs/API.md) | Complete public API documentation |
| [Migration Guide](docs/MIGRATION.md) | Upgrading from v1 to v2 |
| [Contributing Guide](docs/CONTRIBUTING.md) | Development setup, code standards, PR process |

## Examples

| Example | Description |
|---------|-------------|
| [memory_basic](examples/memory_basic/) | Basic memory cache: set/get/delete, batch, atomic, scan |
| [redis_basic](examples/redis_basic/) | Redis cache: connection, key prefix, atomic ops |
| [layered_cache](examples/layered_cache/) | Layered (L1+L2): promotion, write-through |
| [typed_cache](examples/typed_cache/) | Type-safe cache: JSON, String, Int64, GetOrSet |
| [resilience](examples/resilience/) | Retry, circuit breaker, rate limiter |
| [observability](examples/observability/) | Prometheus metrics, OpenTelemetry tracing, health checks |
| [enterprise](examples/enterprise/) | Manager, namespace, warming, hot-key, adaptive TTL, compression |

## Benchmarks

Run benchmarks with:

```bash
go test -bench=. -benchmem ./benchmarks/
```

Key results (approximate, 64-core AMD EPYC, Go 1.22):

| Operation | Throughput | Allocs |
|-----------|-----------|--------|
| Memory Get (256B) | ~50M ops/sec | 1 alloc |
| Memory Set (256B) | ~30M ops/sec | 2 allocs |
| Memory Get (parallel, 64 goroutines) | ~500M ops/sec | 1 alloc |
| Typed Get JSON (256B) | ~5M ops/sec | 3 allocs |
| Typed Get String (256B) | ~40M ops/sec | 1 alloc |
| Typed Get Int64 | ~45M ops/sec | 1 alloc |

## Configuration

### Memory Cache Options

```go
import (
    "github.com/os-gomod/cache/v2"
    "github.com/os-gomod/cache/v2/memory"
)

c, _ := cache.NewMemory(
    memory.WithMaxEntries(100000),        // max entries before eviction
    memory.WithDefaultTTL(10*time.Minute), // default TTL
    memory.WithEvictionPolicy("tinylfu"),  // "lru", "lfu", "fifo", "tinylfu"
    memory.WithShards(128),                // shard count for concurrency
    memory.WithKeyPrefix("app:"),          // prefix all keys
)
```

### Redis Cache Options

```go
import (
    "github.com/os-gomod/cache/v2"
    "github.com/os-gomod/cache/v2/redis"
)

c, _ := cache.NewRedis(
    redis.WithAddr("localhost:6379"),
    redis.WithPassword("secret"),
    redis.WithDB(0),
    redis.WithPoolSize(20),
    redis.WithKeyPrefix("app:v2:"),
)
```

### Layered Cache Options

```go
import (
    "github.com/os-gomod/cache/v2"
    "github.com/os-gomod/cache/v2/layered"
    "github.com/os-gomod/cache/v2/memory"
    "github.com/os-gomod/cache/v2/redis"
)

c, _ := cache.NewLayered(
    layered.WithL1(memory.WithMaxEntries(1000)),
    layered.WithL2(redis.WithAddr("localhost:6379")),
    layered.WithTTL(10*time.Minute),
    layered.WithWriteBack(true),    // async L2 flush
)
```

## Package Import Paths

| Package | Import Path | Description |
|---------|-------------|-------------|
| Public API | `github.com/os-gomod/cache/v2` | Backend, TypedCache[T], Manager, Namespace, Extensions |
| Memory Store | `github.com/os-gomod/cache/v2/memory` | In-memory store options and configuration |
| Redis Store | `github.com/os-gomod/cache/v2/redis` | Redis store options and configuration |
| Layered Store | `github.com/os-gomod/cache/v2/layered` | Layered store options and configuration |

## Enterprise Extensions

### Cache Warming

```go
warmer := cache.NewWarmer(backend, func(keys []string) (map[string][]byte, error) {
    return loadFromDatabase(keys), nil
}, cache.WithWarmerConcurrency(10))
warmer.WarmAll(ctx, func() ([]string, error) { return allKeys, nil })
```

### Hot-Key Detection

```go
detector := cache.NewHotKeyDetector(
    cache.WithHotKeyThreshold(100),
    cache.WithHotKeyCallback(func(key string, count int64) {
        log.Printf("hot key: %s", key)
    }),
)
// Record every cache access
detector.Record(key)
```

### Adaptive TTL

```go
adaptive := cache.NewAdaptiveTTL(30*time.Second, 10*time.Minute)
adaptive.RecordAccess(key) // call on every cache hit
ttl := adaptive.TTL(key, baseTTL) // frequently accessed keys get longer TTL
```

### Compression

```go
cm := cache.NewCompressionMiddleware(cache.NewGzipCompressor(6), 1024)
compressed, _ := cm.Compress(largeValue)
```

## License

MIT License. See [LICENSE](LICENSE) for details.
