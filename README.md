# github.com/os-gomod/cache

A high-performance, production-grade caching library for Go with multi-tier support, advanced eviction policies, distributed stampede protection, and comprehensive observability.

## Features

### Core Capabilities
- **Multi-Tier Caching** - L1 (memory) + L2 (Redis) with configurable promotion policies
- **Multiple Eviction Policies** - LRU, LFU, FIFO, LIFO, MRU, Random, TinyLFU
- **Atomic Operations** - CAS, SetNX, Increment/Decrement, GetSet
- **Batch Operations** - GetMulti, SetMulti, DeleteMulti
- **Typed Cache** - Type-safe generics support with automatic JSON/MessagePack serialization

### Resilience & Reliability
- **Circuit Breaker** - Prevents cascading failures with automatic recovery
- **Rate Limiting** - Token bucket implementation with separate read/write limits
- **Retry with Backoff** - Configurable exponential backoff with jitter
- **Health Checking** - Built-in health checks for all backends

### Performance Optimizations
- **Sharded Architecture** - Reduces lock contention for high concurrency
- **Singleflight Deduplication** - Prevents cache stampedes on cache misses
- **Write-Back Mode** - Asynchronous L2 writes with configurable queue
- **Zero-Allocation Codecs** - String, Int64, Float64 codecs with unsafe optimizations

### Observability
- **Structured Logging** - slog integration with configurable levels and slow operation warnings
- **Prometheus Metrics** - Hit/miss counters, latency histograms, byte size distribution
- **OpenTelemetry Tracing** - Distributed tracing support with span attributes
- **Stats API** - Comprehensive statistics (hit rates, item counts, memory usage, QPS)

### Advanced Features
- **Distributed Stampede Protection** - Redis-based locking for coordinated cache refreshes
- **Cache Invalidation Bus** - Local pub/sub for invalidation events
- **Namespace Support** - Automatic key prefixing for multi-tenant isolation
- **Negative Caching** - Configurable TTL for cache misses to prevent thundering herd

## Installation

```bash
go get github.com/os-gomod/cache
```

## Quick Start

### Memory Cache

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/os-gomod/cache"
)

func main() {
    ctx := context.Background()

    // Create memory cache with default settings
    memCache, err := cache.Memory()
    if err != nil {
        panic(err)
    }
    defer memCache.Close(ctx)

    // Basic operations
    memCache.Set(ctx, "greeting", []byte("Hello, World!"), 5*time.Minute)

    val, _ := memCache.Get(ctx, "greeting")
    fmt.Printf("Value: %s\n", val) // Value: Hello, World!

    exists, _ := memCache.Exists(ctx, "greeting")
    fmt.Printf("Exists: %v\n", exists) // Exists: true
}
```

### Typed Cache with Generics

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    ctx := context.Background()

    // Create typed cache
    memCache, _ := cache.Memory()
    userCache := cache.NewJSONTypedCache[User](memCache)

    // Type-safe operations
    user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
    userCache.Set(ctx, "user:1", user, time.Hour)

    retrieved, _ := userCache.Get(ctx, "user:1")
    fmt.Printf("User: %+v\n", retrieved) // User: {ID:1 Name:Alice Email:alice@example.com}

    // Atomic operations
    newUser := User{ID: 1, Name: "Alice B", Email: "alice@example.com"}
    swapped, _ := userCache.CompareAndSwap(ctx, "user:1", user, newUser, time.Hour)
    fmt.Printf("CAS swapped: %v\n", swapped)
}
```

### Layered Cache (L1 Memory + L2 Redis)

```go
func main() {
    ctx := context.Background()

    layered, err := cache.Layered(
        layer.WithL1MaxEntries(10000),
        layer.WithL1TTL(5*time.Minute),
        layer.WithL2Address("localhost:6379"),
        layer.WithL2TTL(time.Hour),
        layer.WithPromoteOnHit(true),
        layer.WithWriteBack(true),
    )
    if err != nil {
        panic(err)
    }
    defer layered.Close(ctx)

    // Get promotes from L2 to L1 on hit
    val, _ := layered.Get(ctx, "popular-key")
}
```

### Redis with Distributed Stampede Protection

```go
func main() {
    ctx := context.Background()

    redisCache, err := cache.Redis(
        redis.WithAddress("localhost:6379"),
        redis.WithKeyPrefix("myapp:"),
        redis.WithDistributedStampedeProtection(true),
        redis.WithStampedeLockTTL(5*time.Second),
    )
    if err != nil {
        panic(err)
    }
    defer redisCache.Close(ctx)

    // GetOrSet with distributed lock protection
    val, _ := redisCache.GetOrSet(ctx, "expensive-key",
        func() ([]byte, error) {
            // Expensive computation here
            return []byte("computed-value"), nil
        },
        time.Minute,
    )
}
```

### Resilience Policies

```go
func main() {
    ctx := context.Background()

    // Create resilient cache with circuit breaker and retries
    policy := resilience.Policy{
        CircuitBreaker: resilience.NewCircuitBreaker(5, 30*time.Second),
        Limiter:        resilience.NewLimiter(1000, 100),
        Retry: resilience.RetryConfig{
            MaxAttempts:  3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     5 * time.Second,
            Multiplier:   2.0,
            Jitter:       true,
        },
        Timeout: 5 * time.Second,
    }

    memCache, _ := cache.Memory()
    resilient := resilience.NewCacheWithPolicy(memCache, policy)

    // Automatically retries on transient failures
    val, err := resilient.Get(ctx, "key")
}
```

### Cache Manager with Multiple Backends

```go
func main() {
    ctx := context.Background()

    primary, _ := cache.Memory(memory.WithMaxEntries(10000))
    secondary, _ := cache.Memory(memory.WithMaxEntries(5000))

    mgr, _ := cache.New(
        manager.WithDefaultBackend(primary),
        manager.WithBackend("secondary", secondary),
        manager.WithPolicy(resilience.DefaultPolicy()),
    )
    defer mgr.Close(ctx)

    // Use default backend
    mgr.Set(ctx, "key", []byte("value"), time.Hour)

    // Access specific backend
    sec, _ := mgr.Backend("secondary")
    sec.Set(ctx, "isolated", []byte("data"), time.Hour)

    // Namespaced operations
    tenantNS := mgr.Namespace("tenant:acme:")
    tenantNS.Set(ctx, "config", []byte("settings"), 0)
}
```

### Observability Setup

```go
import (
    "log/slog"
    "os"
    "github.com/prometheus/client_golang/prometheus"
    "go.opentelemetry.io/otel/trace"
)

func setupObservability() {
    // Structured logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    loggingInterceptor := observability.NewLoggingInterceptor(
        logger,
        observability.WithSlowThreshold(100*time.Millisecond),
        observability.WithLoggingLevel(slog.LevelDebug),
    )

    // Prometheus metrics
    promReg := prometheus.NewRegistry()
    promInterceptor, _ := observability.NewPrometheusInterceptor(promReg)

    // OpenTelemetry tracing
    tracer := otel.Tracer("cache")
    otelInterceptor := observability.NewOTelInterceptor(tracer)

    // Chain interceptors
    chain := observability.NewChain(loggingInterceptor, promInterceptor, otelInterceptor)

    // Apply to cache
    memCache, _ := cache.Memory(memory.WithInterceptors(chain))
}
```

## Configuration Options

### Memory Cache Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMaxEntries(n)` | Maximum number of items | 10000 |
| `WithMaxMemoryMB(mb)` | Maximum memory in MB | 100 |
| `WithTTL(d)` | Default TTL for entries | 30m |
| `WithCleanupInterval(d)` | Expired entry cleanup interval | 5m |
| `WithShards(n)` | Number of shards (power of two) | 32 |
| `WithLRU()` / `WithLFU()` / `WithFIFO()` | Eviction policy | LRU |
| `WithOnEvictionPolicy(fn)` | Eviction callback | nil |
| `WithEnableMetrics(b)` | Enable metrics collection | false |

### Redis Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithAddress(addr)` | Redis address(es) | localhost:6379 |
| `WithPassword(pwd)` | Redis password | "" |
| `WithDB(db)` | Database number | 0 |
| `WithPoolSize(n)` | Connection pool size | 10 |
| `WithTTL(d)` | Default TTL | 1h |
| `WithKeyPrefix(prefix)` | Key prefix for isolation | "" |
| `WithDistributedStampedeProtection(b)` | Enable distributed locking | false |
| `WithEnablePipeline(b)` | Enable pipeline mode | true |

### Layered Cache Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithL1Config(cfg)` | L1 memory configuration | DefaultMemory() |
| `WithL2Config(cfg)` | L2 Redis configuration | DefaultRedis() |
| `WithPromoteOnHit(b)` | Promote L2 hits to L1 | true |
| `WithWriteBack(b)` | Async L2 writes | false |
| `WithNegativeTTL(d)` | TTL for negative cache | 30s |
| `WithL1TTLOverride(d)` | Override L1 TTL for promotions | 0 (use L2 TTL) |

## Architecture

### Layered Cache Flow

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────┐
│                     Layered Cache                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐   Get Miss    ┌─────────────┐             │
│  │    L1       │ ─────────────►│    L2       │             │
│  │   Memory    │               │   Redis     │             │
│  │  (hot tier) │ ◄─────────────│  (cold tier)│             │
│  └─────────────┘   Promote     └─────────────┘             │
│         │                              │                    │
│         ▼                              ▼                    │
│  ┌─────────────────────────────────────────────┐           │
│  │         WriteBack Queue (async)             │           │
│  └─────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

### Sharded Memory Architecture

```
┌─────────────────────────────────────────────┐
│                 Cache                        │
├─────────────────────────────────────────────┤
│  ┌─────────┐ ┌─────────┐ ┌─────────┐        │
│  │ Shard 0 │ │ Shard 1 │ │ Shard N │        │
│  │  mutex  │ │  mutex  │ │  mutex  │        │
│  │  map    │ │  map    │ │  map    │        │
│  │ evictor │ │ evictor │ │ evictor │        │
│  └─────────┘ └─────────┘ └─────────┘        │
└─────────────────────────────────────────────┘
```

### Eviction Policies Comparison

| Policy | Description | Use Case |
|--------|-------------|----------|
| **LRU** | Least Recently Used | General purpose, good temporal locality |
| **LFU** | Least Frequently Used | Frequently accessed items should persist |
| **FIFO** | First In First Out | Time-based access patterns |
| **LIFO** | Last In First Out | Stack-like access patterns |
| **MRU** | Most Recently Used | Access patterns with rapid churn |
| **Random** | Random selection | Simplicity, avoids pathological cases |
| **TinyLFU** | Approximate LFU with reset | High throughput, low memory overhead |

## Advanced Examples

### Invalidation Bus

```go
bus := invalidation.NewLocalBus()

// Subscribe to events
unsubscribe := bus.Subscribe(invalidation.HandlerFunc(func(evt invalidation.Event) {
    fmt.Printf("Event: %s key=%s\n", evt.Kind, evt.Key)
}))

// Publish events
bus.Publish(invalidation.Event{
    Kind:    invalidation.KindDelete,
    Key:     "user:123",
    Backend: "cache",
})
```

### Stampede Protection

```go
// Local singleflight protection
val, _ := memCache.GetOrSet(ctx, "expensive-key",
    func() ([]byte, error) {
        // Only one goroutine executes this
        return computeExpensiveValue()
    },
    time.Minute,
)

// Distributed protection with Redis
val, _ := redisCache.GetOrSet(ctx, "global-key",
    func() ([]byte, error) {
        // Only one instance executes this
        return computeGlobalValue()
    },
    time.Minute,
)
```

### Health Checks

```go
// Single backend health
if err := cache.Ping(ctx); err != nil {
    log.Printf("Cache unhealthy: %v", err)
}

// Manager health across all backends
health := mgr.HealthCheck(ctx)
for name, err := range health {
    if err != nil {
        log.Printf("Backend %s unhealthy: %v", name, err)
    }
}
```

### Statistics Collection

```go
snap := cache.Stats()
fmt.Printf("Hits: %d, Misses: %d, Hit Rate: %.2f%%\n",
    snap.Hits, snap.Misses, snap.HitRate)
fmt.Printf("Items: %d, Memory: %d bytes\n", snap.Items, snap.Memory)
fmt.Printf("Ops/sec: %.2f, Uptime: %v\n", snap.OpsPerSecond, snap.Uptime)

// Layered cache specific stats
fmt.Printf("L1 Hits: %d, L2 Hits: %d, Promotions: %d\n",
    snap.L1Hits, snap.L2Hits, snap.L2Promotions)
```

## Performance Benchmarks

Typical performance on modern hardware (8-core, 16GB):

| Operation | Memory Cache | Redis (local) | Layered (L1 hit) |
|-----------|--------------|---------------|------------------|
| Get (hit) | ~200ns | ~500µs | ~250ns |
| Get (miss) | ~50ns | ~500µs | ~500µs |
| Set | ~300ns | ~500µs | ~350ns |
| Delete | ~150ns | ~500µs | ~200ns |
| CAS | ~350ns | ~600µs | ~400ns |

## Testing

```bash
# Run all tests with race detector
make test

# Run with coverage
make coverage

# Run benchmarks
make benchmark

# Run integration tests (requires Redis)
make test-integration

# Start Redis for testing
make docker-up
```

## Requirements

- Go 1.26.1 or higher
- Redis 6.0+ (for Redis and Layered backends)
- Optional: Prometheus (for metrics), OpenTelemetry (for tracing)

## License

MIT License

## Contributing

Contributions are welcome! Please submit pull requests with:
- Comprehensive test coverage
- Updated documentation
- Adherence to existing code style

Run validation before submitting:
```bash
make validate
```
