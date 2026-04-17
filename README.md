# README.md - Comprehensive Documentation

## Cache - High-Performance Multi-Layer Caching for Go

[![Go Version](https://img.shields.io/badge/Go-1.26.1-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A production-grade, high-performance caching library for Go applications featuring multi-tier caching, atomic operations, distributed stampede protection, comprehensive observability, and resilience patterns.

---

# Table of Contents

1. [Overview](#overview)
2. [Core Features](#core-features)
3. [Quick Start](#quick-start)
4. [Architecture](#architecture)
5. [Installation](#installation)
6. [Backends](#backends)
   - [Memory Cache](#memory-cache)
   - [Redis Cache](#redis-cache)
   - [Layered Cache](#layered-cache)
7. [Typed Cache](#typed-cache)
8. [Atomic Operations](#atomic-operations)
9. [Batch Operations](#batch-operations)
10. [Resilience Patterns](#resilience-patterns)
   - [Circuit Breaker](#circuit-breaker)
   - [Rate Limiter](#rate-limiter)
   - [Retry with Backoff](#retry-with-backoff)
11. [Stampede Protection](#stampede-protection)
12. [Cache Manager](#cache-manager)
13. [Invalidation Bus](#invalidation-bus)
14. [Observability](#observability)
   - [Logging](#logging)
   - [Prometheus Metrics](#prometheus-metrics)
   - [OpenTelemetry Tracing](#opentelemetry-tracing)
15. [Error Handling](#error-handling)
16. [Codecs](#codecs)
17. [Configuration](#configuration)
18. [Best Practices](#best-practices)
19. [API Reference](#api-reference)
20. [Docker Development](#docker-development)

---

## Overview

Cache is a comprehensive caching solution for Go applications, designed to handle complex caching scenarios in modern distributed systems. It supports multiple storage backends, sophisticated eviction policies, atomic operations, distributed coordination, and enterprise-grade observability.

### Key Capabilities

- **Multi-Tier Caching**: L1 (memory) + L2 (Redis) with automatic promotion and write-back
- **Multiple Eviction Policies**: LRU, LFU, FIFO, LIFO, MRU, Random, TinyLFU
- **Atomic Operations**: CAS, SetNX, Increment/Decrement, GetSet
- **Distributed Stampede Protection**: Prevents cache stampede with distributed locks
- **Resilience Patterns**: Circuit breakers, rate limiters, retry with backoff
- **Typed Cache**: Type-safe operations with generics and multiple codecs
- **Comprehensive Observability**: Logging, Prometheus metrics, OpenTelemetry tracing
- **Event Bus**: Cache invalidation and expiration events
- **High Performance**: Sharded memory cache with zero-copy optimizations

---

## Quick Start

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
    // Create a memory cache with LRU eviction
    cache, err := cache.Memory(
        memory.WithMaxEntries(10000),
        memory.WithTTL(5*time.Minute),
        memory.WithLRU(),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer cache.Close(context.Background())

    ctx := context.Background()

    // Basic operations
    cache.Set(ctx, "user:123", []byte(`{"name":"Alice"}`), time.Hour)

    val, err := cache.Get(ctx, "user:123")
    if err == nil {
        fmt.Printf("User: %s\n", string(val))
    }

    // Typed cache with JSON
    type User struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }

    userCache := cache.NewJSONTypedCache[User](cache)

    userCache.Set(ctx, "user:456", User{Name: "Bob", Age: 30}, time.Hour)

    user, _ := userCache.Get(ctx, "user:456")
    fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
}
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Application Code                                  │
│                    (TypedCache, direct Backend access)                      │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Cache Manager                                    │
│              (Multi-backend orchestration, namespace isolation)             │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
        ┌─────────────┬───────────────┼───────────────┬─────────────┐
        ▼             ▼               ▼               ▼             ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌───────────────┐
│   Memory     │ │    Redis     │ │   Layered    │ │  Resilience  │ │  Invalidation │
│   Cache      │ │    Cache     │ │    Cache     │ │   Wrapper    │ │     Bus       │
│ (L1/TinyLFU) │ │ (Distributed)│ │ (L1 + L2)    │ │(CB/RateLimit)│ │  (Events)     │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └───────────────┘
                                      │
                                      ▼
┌───────────────────────────────────────────────────────────────────────────────┐
│                          Internal Components                                  │
│  ┌──────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ │
│  │ Singleflight │ │  Stats     │ │  Lifecycle │ │   Pooling  │ │  Chain     │ │
│  │ (Dedup)      │ │ (Metrics)  │ │ (Guard)    │ │ (Buffer)   │ │ (Observ)   │ │
│  └──────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘ │
└───────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Storage Backends                                  │
│           Memory (sharded)  │  Redis (cluster/sentinel/standalone)          │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Installation

```bash
go get github.com/os-gomod/cache
```

For Redis support (automatically included):

```bash
go get github.com/redis/go-redis/v9
```

For Prometheus metrics:

```bash
go get github.com/prometheus/client_golang
```

For OpenTelemetry tracing:

```bash
go get go.opentelemetry.io/otel
```

---

## Backends

### Memory Cache

High-performance in-memory cache with sharding and multiple eviction policies.

```go
import "github.com/os-gomod/cache/memory"

// Basic memory cache
memCache, err := memory.New(
    memory.WithMaxEntries(10000),
    memory.WithTTL(30*time.Minute),
    memory.WithCleanupInterval(5*time.Minute),
)

// Advanced configuration with TinyLFU
memCache, err := memory.New(
    memory.WithMaxEntries(50000),
    memory.WithMaxMemoryMB(512),
    memory.WithShards(64),              // 64 shards for concurrency
    memory.WithTinyLFU(),               // TinyLFU eviction policy
    memory.WithOnEvictionPolicy(func(key, reason string) {
        log.Printf("Evicted: %s due to %s", key, reason)
    }),
    memory.WithEnableMetrics(true),
)
```

#### Eviction Policies

| Policy          | Description           | Best For                           |
| --------------- | --------------------- | ---------------------------------- |
| `WithLRU()`     | Least Recently Used   | General purpose, temporal locality |
| `WithLFU()`     | Least Frequently Used | Frequently accessed items          |
| `WithFIFO()`    | First In First Out    | Simple, fair eviction              |
| `WithLIFO()`    | Last In First Out     | Stack-like access patterns         |
| `WithMRU()`     | Most Recently Used    | Opposite of LRU                    |
| `WithRandom()`  | Random eviction       | Simple, low overhead               |
| `WithTinyLFU()` | TinyLFU (LFU + Bloom) | High hit rate, low memory          |

### Redis Cache

Distributed cache with Redis support, pipeline optimization, and distributed stampede protection.

```go
import "github.com/os-gomod/cache/redis"

redisCache, err := redis.New(
    redis.WithAddress("localhost:6379"),
    redis.WithPassword(""),
    redis.WithDB(0),
    redis.WithPoolSize(50),
    redis.WithTTL(2*time.Hour),
    redis.WithKeyPrefix("myapp:"),
    redis.WithEnablePipeline(true),
    redis.WithDistributedStampedeProtection(true),
    redis.WithStampedeLockTTL(5*time.Second),
    redis.WithStampedeWaitTimeout(3*time.Second),
)

// Redis data structures
redisCache.HSet(ctx, "user:123", "name", "Alice")
redisCache.HSet(ctx, "user:123", "email", "alice@example.com")

name, _ := redisCache.HGet(ctx, "user:123", "name")

// List operations
redisCache.LPush(ctx, "recent:views", "item1", "item2", "item3")
items, _ := redisCache.LRange(ctx, "recent:views", 0, 9)

// Set operations
redisCache.SAdd(ctx, "tags:electronics", "laptop", "phone", "tablet")
members, _ := redisCache.SMembers(ctx, "tags:electronics")

// Sorted set (leaderboard)
redisCache.ZAdd(ctx, "leaderboard:sales", 1500.0, "product-001")
topProducts, _ := redisCache.ZRange(ctx, "leaderboard:sales", 0, 10)
```

### Layered Cache

Multi-tier cache with L1 (memory) for speed and L2 (Redis) for persistence and distribution.

```go
import "github.com/os-gomod/cache/layer"

layeredCache, err := layer.New(
    // L1 Memory configuration
    layer.WithL1MaxEntries(10000),
    layer.WithL1TTL(5*time.Minute),
    layer.WithL1Shards(32),
    layer.WithL1LRU(),

    // L2 Redis configuration
    layer.WithL2Address("localhost:6379"),
    layer.WithL2TTL(1*time.Hour),
    layer.WithL2KeyPrefix("layered:"),

    // Layered behavior
    layer.WithPromoteOnHit(true),    // Promote L2 hits to L1
    layer.WithWriteBack(true),       // Async write to L2
    layer.WithWriteBackConfig(1024, 8), // Queue size, workers
    layer.WithNegativeTTL(10*time.Second), // Cache negative responses
)

// L1 invalidation
layeredCache.InvalidateL1(ctx, "key1", "key2")

// Refresh L1 from L2
layeredCache.Refresh(ctx, "stale-key")
```

---

## Typed Cache

Type-safe cache operations with support for multiple serialization formats.

```go
import "github.com/os-gomod/cache"

type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

// JSON typed cache
productCache := cache.NewJSONTypedCache[Product](backend)

product := Product{ID: "p1", Name: "Laptop", Price: 1299.99}
productCache.Set(ctx, "product:p1", product, time.Hour)

retrieved, _ := productCache.Get(ctx, "product:p1")
fmt.Printf("%s: $%.2f\n", retrieved.Name, retrieved.Price)

// String typed cache (zero-allocation)
strCache := cache.NewStringTypedCache(backend)
strCache.Set(ctx, "greeting", "Hello, World!", 0)
greeting, _ := strCache.Get(ctx, "greeting")

// Int64 typed cache
intCache := cache.NewInt64TypedCache(backend)
intCache.Set(ctx, "counter", 42, 0)
counter, _ := intCache.Get(ctx, "counter")

// GetOrSet with typed values
user, err := userCache.GetOrSet(ctx, "user:123", func() (User, error) {
    return fetchUserFromDB("123"), nil
}, time.Hour)
```

### Custom Codec

```go
import "github.com/os-gomod/cache/codec"

type ProtobufCodec[T any] struct{}

func (c ProtobufCodec[T]) Encode(v T, buf []byte) ([]byte, error) {
    return proto.Marshal(v)
}

func (c ProtobufCodec[T]) Decode(data []byte) (T, error) {
    var v T
    err := proto.Unmarshal(data, &v)
    return v, err
}

func (c ProtobufCodec[T]) ContentType() string {
    return "application/x-protobuf"
}

// Use custom codec
typedCache := cache.NewTypedCache(backend, ProtobufCodec[User]{})
```

---

## Atomic Operations

```go
// Compare and Swap (CAS)
oldProduct := Product{ID: "p1", Version: 1}
newProduct := Product{ID: "p1", Version: 2}

swapped, err := cache.CompareAndSwap(ctx, "product:p1", oldProduct, newProduct, time.Hour)
if swapped {
    fmt.Println("Product updated successfully")
}

// Set if Not Exists (distributed lock)
acquired, err := cache.SetNX(ctx, "lock:order:123", []byte("processing"), 30*time.Second)
if acquired {
    defer cache.Delete(ctx, "lock:order:123")
    // Process order
}

// Increment/Decrement (counter)
views, _ := cache.Increment(ctx, "page:views", 1)
fmt.Printf("Page views: %d\n", views)

// GetSet (atomic replace)
oldVal, _ := cache.GetSet(ctx, "config:version", []byte("v2"), 0)
fmt.Printf("Previous version: %s\n", string(oldVal))
```

---

## Batch Operations

```go
// Set multiple keys
items := map[string][]byte{
    "user:1": []byte(`{"name":"Alice"}`),
    "user:2": []byte(`{"name":"Bob"}`),
    "user:3": []byte(`{"name":"Charlie"}`),
}
cache.SetMulti(ctx, items, time.Hour)

// Get multiple keys
results, _ := cache.GetMulti(ctx, "user:1", "user:2", "user:3", "user:4")
for key, val := range results {
    fmt.Printf("%s: %s\n", key, string(val))
}

// Delete multiple keys
cache.DeleteMulti(ctx, "user:1", "user:2", "user:3")
```

---

## Resilience Patterns

### Circuit Breaker

```go
import "github.com/os-gomod/cache/resilience"

// Create circuit breaker
cb := resilience.NewCircuitBreaker(
    5,                    // Failure threshold
    30*time.Second,       // Reset timeout
)

fmt.Printf("State: %s\n", cb.State()) // closed

// Record failures
for i := 0; i < 5; i++ {
    cb.Failure()
}
fmt.Printf("State: %s, Allow: %v\n", cb.State(), cb.Allow()) // open, false

// After timeout, half-open state allows probe
time.Sleep(30 * time.Second)
fmt.Printf("State: %s, Allow: %v\n", cb.State(), cb.Allow()) // half-open, true

// Record success to close circuit
cb.Success()
fmt.Printf("State: %s\n", cb.State()) // closed
```

### Rate Limiter

```go
// Token bucket rate limiter
limiter := resilience.NewLimiterWithConfig(resilience.LimiterConfig{
    ReadRPS:    1000,   // 1000 reads per second
    ReadBurst:  100,    // Burst up to 100
    WriteRPS:   500,    // 500 writes per second
    WriteBurst: 50,
})

if limiter.AllowRead(ctx) {
    // Perform read operation
} else {
    return cacheerrors.ErrRateLimited
}
```

### Retry with Backoff

```go
policy := resilience.Policy{
    CircuitBreaker: resilience.NewCircuitBreaker(5, 30*time.Second),
    Limiter:        resilience.NewLimiter(1000, 100),
    Retry: resilience.RetryConfig{
        MaxAttempts:  5,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     5 * time.Second,
        Multiplier:   2.0,
        Jitter:       true,
        RetryableErr: func(err error) bool {
            return cacheerrors.IsTimeout(err) || cacheerrors.IsConnectionError(err)
        },
    },
    Timeout: 5 * time.Second,
}

// Wrap any backend with resilience
resilientCache := resilience.NewCacheWithPolicy(backend, policy)
```

---

## Stampede Protection

### Local (Singleflight)

```go
// Automatic singleflight deduplication
val, err := cache.GetOrSet(ctx, "expensive:key", func() ([]byte, error) {
    // This function runs only once for concurrent requests
    return computeExpensiveValue(), nil
}, time.Minute)
```

### Distributed (Redis Locks)

```go
import "github.com/os-gomod/cache/stampede"

// Enable distributed stampede protection
redisCache, _ := redis.New(
    redis.WithDistributedStampedeProtection(true),
    redis.WithStampedeLockTTL(5*time.Second),
    redis.WithStampedeWaitTimeout(3*time.Second),
)

// Distributed lock manually
lockKey := "distributed:lock:critical-section"
token := stampede.GenerateToken()

lock, acquired, err := stampede.AcquireLock(ctx, redisClient, lockKey, token, 5*time.Second)
if acquired {
    defer lock.Release(ctx)
    // Perform critical section
}
```

### Early Refresh (XFetch Algorithm)

```go
// Detector for early refresh based on remaining TTL
detector := stampede.NewDetector(1.0, observabilityChain)

// Automatically triggers background refresh before expiration
val, err := detector.Do(ctx, key, currentValue, entry,
    func(ctx context.Context) ([]byte, error) {
        // Refresh function (runs in background)
        return fetchNewValue(), nil
    },
    func(newVal []byte) {
        // Optional callback when refresh completes
        cache.Set(ctx, key, newVal, ttl)
    },
)
```

---

## Cache Manager

Multi-backend management with namespace isolation.

```go
import "github.com/os-gomod/cache/manager"

// Create manager with multiple backends
hotCache, _ := cache.Memory(memory.WithMaxEntries(1000))
warmCache, _ := cache.Memory(memory.WithMaxEntries(10000))
coldCache, _ := cache.Redis(redis.WithAddress("localhost:6379"))

manager, err := cache.New(
    manager.WithDefaultBackend(hotCache),      // Default: hot tier
    manager.WithBackend("warm", warmCache),    // Warm tier
    manager.WithBackend("cold", coldCache),    // Cold tier
    manager.WithPolicy(resilience.DefaultPolicy()),
)

// Use default backend
manager.Set(ctx, "session:123", data, 15*time.Minute)

// Use specific backend
warm, _ := manager.Backend("warm")
warm.Set(ctx, "analytics:daily", analytics, 24*time.Hour)

// Health check
health := manager.HealthCheck(ctx)
for name, err := range health {
    if err != nil {
        log.Printf("Backend %s unhealthy: %v", name, err)
    }
}

// Namespace isolation
tenant1 := manager.Namespace("tenant:acme:")
tenant2 := manager.Namespace("tenant:beta:")

tenant1.Set(ctx, "config", []byte(`{"theme":"dark"}`), 0)
tenant2.Set(ctx, "config", []byte(`{"theme":"light"}`), 0)

// Get statistics per backend
stats := manager.Stats()
for name, snap := range stats {
    fmt.Printf("%s: hits=%d, hitRate=%.2f%%\n", name, snap.Hits, snap.HitRate)
}
```

---

## Invalidation Bus

Event-driven cache invalidation with pattern matching.

```go
import "github.com/os-gomod/cache/invalidation"

// Create event bus
bus := invalidation.NewLocalBus()

// Subscribe to events
unsubscribe := bus.Subscribe(invalidation.HandlerFunc(func(evt invalidation.Event) {
    fmt.Printf("Event: kind=%s key=%q\n", evt.Kind, evt.Key)
    // Invalidate local cache
    localCache.Delete(context.Background(), evt.Key)
}))
defer unsubscribe()

// Publish events
bus.Publish(invalidation.Event{
    Kind:      invalidation.KindDelete,
    Key:       "user:123",
    Backend:   "cache",
    Timestamp: time.Now(),
})

bus.Publish(invalidation.Event{
    Kind:      invalidation.KindInvalidate,
    Pattern:   "product:*",
    Backend:   "layered",
})

bus.Publish(invalidation.Event{
    Kind:      invalidation.KindExpire,
    Key:       "session:expired",
    Backend:   "redis",
})
```

---

## Observability

### Logging Interceptor

```go
import (
    "log/slog"
    "github.com/os-gomod/cache/observability"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

loggingInterceptor := observability.NewLoggingInterceptor(
    logger,
    observability.WithSlowThreshold(50*time.Millisecond),
    observability.WithLoggingLevel(slog.LevelDebug),
)

cache, _ := memory.New(
    memory.WithInterceptors(loggingInterceptor),
)
```

### Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/os-gomod/cache/observability"
)

reg := prometheus.NewRegistry()
promInterceptor, _ := observability.NewPrometheusInterceptor(reg)

cache, _ := memory.New(
    memory.WithInterceptors(promInterceptor),
)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
```

### OpenTelemetry Tracing

```go
import (
    "go.opentelemetry.io/otel"
    "github.com/os-gomod/cache/observability"
)

tracer := otel.Tracer("myapp-cache")
otelInterceptor := observability.NewOTelInterceptor(tracer)

cache, _ := memory.New(
    memory.WithInterceptors(otelInterceptor),
)

// Chain multiple interceptors
chain := observability.NewChain(
    loggingInterceptor,
    promInterceptor,
    otelInterceptor,
)

cache, _ := memory.New(
    memory.WithInterceptors(chain),
)
```

### Statistics Snapshot

```go
snap := cache.Stats()

fmt.Printf("Hits: %d\n", snap.Hits)
fmt.Printf("Misses: %d\n", snap.Misses)
fmt.Printf("Hit Rate: %.2f%%\n", snap.HitRate)
fmt.Printf("Sets: %d\n", snap.Sets)
fmt.Printf("Deletes: %d\n", snap.Deletes)
fmt.Printf("Evictions: %d\n", snap.Evictions)
fmt.Printf("Items: %d\n", snap.Items)
fmt.Printf("Memory: %d bytes (%.2f MB)\n", snap.Memory, float64(snap.Memory)/(1024*1024))
fmt.Printf("Ops/Sec: %.2f\n", snap.OpsPerSecond)
fmt.Printf("Uptime: %v\n", snap.Uptime)

// Layered cache specific stats
fmt.Printf("L1 Hits: %d, L1 Misses: %d\n", snap.L1Hits, snap.L1Misses)
fmt.Printf("L2 Hits: %d, L2 Misses: %d\n", snap.L2Hits, snap.L2Misses)
fmt.Printf("L2 Promotions: %d\n", snap.L2Promotions)

// Write-back stats
fmt.Printf("Write-back Enqueued: %d, Flushed: %d, Dropped: %d\n",
    snap.WriteBackEnqueued, snap.WriteBackFlushed, snap.WriteBackDropped)
```

### Hit Rate Analytics

```go
window := observability.NewHitRateWindow(10*time.Second, 60) // 60 buckets of 10s

// Record operations
window.Record(true)  // hit
window.Record(false) // miss
window.RecordLatency(5 * time.Millisecond)

fmt.Printf("Current Hit Rate: %.2f%%\n", window.HitRate()*100)
fmt.Printf("P50 Latency: %v\n", window.P50Latency())
fmt.Printf("P99 Latency: %v\n", window.P99Latency())

// Advance window periodically
go func() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        window.Advance()
    }
}()
```

---

## Error Handling

Comprehensive error types with codes and metadata.

```go
import cacheerrors "github.com/os-gomod/cache/errors"

_, err := cache.Get(ctx, "nonexistent")
if cacheerrors.IsNotFound(err) {
    fmt.Println("Key not found")
}

// Check error codes
if cacheerrors.Is(err, cacheerrors.CodeTimeout) {
    fmt.Println("Operation timed out")
}

// Get error metadata
if ce, ok := cacheerrors.AsType(err); ok {
    fmt.Printf("Code: %s, Op: %s, Key: %s\n", ce.Code, ce.Operation, ce.Key)

    if metadata, ok := cacheerrors.GetMetadata(err, "attempt"); ok {
        fmt.Printf("Attempt: %v\n", metadata)
    }
}

// Custom error with metadata
err := cacheerrors.New("payment.process", "order-123", fmt.Errorf("insufficient funds")).
    WithMetadata("attempt", 3).
    WithMetadata("payment_method", "credit_card")

// Retryable error detection
if cacheerrors.Retryable(err) {
    // Implement retry logic
}
```

### Error Codes

| Code               | Description             |
| ------------------ | ----------------------- |
| `CodeNotFound`     | Key not found           |
| `CodeCacheClosed`  | Cache instance closed   |
| `CodeTimeout`      | Operation timeout       |
| `CodeCancelled`    | Context cancelled       |
| `CodeRateLimited`  | Rate limit exceeded     |
| `CodeCircuitOpen`  | Circuit breaker open    |
| `CodeConnection`   | Connection error        |
| `CodeEmptyKey`     | Empty key provided      |
| `CodeExpired`      | Key expired             |
| `CodeNotSupported` | Operation not supported |
| `CodeLockFailed`   | Lock acquisition failed |

---

## Codecs

Multiple serialization formats with zero-allocation options.

### Built-in Codecs

```go
import "github.com/os-gomod/cache/codec"

// JSON (with sync.Pool for buffers)
jsonCodec := codec.NewJSONCodec[User]()
data, _ := jsonCodec.Encode(user, nil)
user, _ := jsonCodec.Decode(data)

// String (zero-allocation, unsafe conversion)
stringCodec := codec.StringCodec{}
data, _ := stringCodec.Encode("hello", nil)
str, _ := stringCodec.Decode(data)

// Int64 (zero-allocation)
int64Codec := codec.Int64Codec{}
data, _ := int64Codec.Encode(12345, nil)
val, _ := int64Codec.Decode(data)

// Float64
floatCodec := codec.Float64Codec{}
data, _ := floatCodec.Encode(3.14159, nil)
pi, _ := floatCodec.Decode(data)

// Raw (passthrough)
rawCodec := codec.RawCodec{}
data, _ := rawCodec.Encode([]byte{0x01, 0x02}, nil)

// MessagePack
msgpackCodec := codec.NewMsgPack[User]()
data, _ := msgpackCodec.Encode(user, nil)
user, _ := msgpackCodec.Decode(data)
```

### Buffer Pooling

```go
// Typed cache uses buffer pooling for zero-allocation encoding
bufPool := pooling.NewBufPool(64) // 64 byte initial capacity

buf := bufPool.Get()
defer bufPool.Put(buf)

// Use buffer for encoding
data, err := codec.Encode(value, *buf)
```

---

## Configuration

### Memory Cache Options

| Option                      | Description                     | Default |
| --------------------------- | ------------------------------- | ------- |
| `WithMaxEntries(n)`         | Maximum number of items         | 10000   |
| `WithMaxMemoryMB(mb)`       | Maximum memory in MB            | 100     |
| `WithMaxMemoryBytes(bytes)` | Maximum memory in bytes         | -       |
| `WithTTL(duration)`         | Default TTL                     | 30m     |
| `WithCleanupInterval(d)`    | Expired item cleanup interval   | 5m      |
| `WithShards(n)`             | Number of shards (power of two) | 32      |
| `WithLRU()`                 | LRU eviction policy             | default |
| `WithLFU()`                 | LFU eviction policy             | -       |
| `WithTinyLFU()`             | TinyLFU eviction policy         | -       |
| `WithFIFO()`                | FIFO eviction policy            | -       |
| `WithLIFO()`                | LIFO eviction policy            | -       |
| `WithMRU()`                 | MRU eviction policy             | -       |
| `WithRandom()`              | Random eviction policy          | -       |
| `WithOnEvictionPolicy(fn)`  | Eviction callback               | nil     |
| `WithEnableMetrics(bool)`   | Enable metrics collection       | false   |

### Redis Cache Options

| Option                                    | Description                            | Default        |
| ----------------------------------------- | -------------------------------------- | -------------- |
| `WithAddress(addr)`                       | Redis address                          | localhost:6379 |
| `WithPassword(pwd)`                       | Redis password                         | ""             |
| `WithDB(db)`                              | Database number                        | 0              |
| `WithPoolSize(n)`                         | Connection pool size                   | 10             |
| `WithMinIdleConns(n)`                     | Minimum idle connections               | 2              |
| `WithMaxRetries(n)`                       | Maximum retries                        | 3              |
| `WithTTL(duration)`                       | Default TTL                            | 1h             |
| `WithKeyPrefix(prefix)`                   | Key prefix                             | ""             |
| `WithEnablePipeline(bool)`                | Enable pipeline                        | true           |
| `WithDistributedStampedeProtection(bool)` | Enable distributed stampede protection | false          |
| `WithStampedeLockTTL(duration)`           | Stampede lock TTL                      | 5s             |
| `WithStampedeWaitTimeout(duration)`       | Wait timeout for value                 | 2s             |

### Layered Cache Options

| Option                                | Description                 | Default         |
| ------------------------------------- | --------------------------- | --------------- |
| `WithL1Config(cfg)`                   | L1 memory config            | DefaultMemory() |
| `WithL1MaxEntries(n)`                 | L1 max entries              | 10000           |
| `WithL1TTL(duration)`                 | L1 TTL                      | 30m             |
| `WithL1Shards(n)`                     | L1 shard count              | 32              |
| `WithL2Address(addr)`                 | L2 Redis address            | localhost:6379  |
| `WithL2TTL(duration)`                 | L2 TTL                      | 1h              |
| `WithPromoteOnHit(bool)`              | Promote L2 hits to L1       | true            |
| `WithWriteBack(bool)`                 | Async write to L2           | false           |
| `WithWriteBackConfig(queue, workers)` | Write-back configuration    | 512, 4          |
| `WithNegativeTTL(duration)`           | Negative response cache TTL | 30s             |
| `WithSyncEnabled(bool)`               | Enable sync invalidation    | false           |

---

## Best Practices

### 1. Choose the Right Cache Type

```go
// High QPS, low latency requirements → Memory Cache
memCache, _ := memory.New(memory.WithMaxEntries(10000))

// Distributed, persistent, shared cache → Redis Cache
redisCache, _ := redis.New(redis.WithAddress("cluster:6379"))

// Best of both worlds → Layered Cache
layeredCache, _ := layer.New(
    layer.WithL1MaxEntries(1000),   // Small, fast L1
    layer.WithL2Address("redis:6379"), // Large, distributed L2
)
```

### 2. Set Appropriate TTLs

```go
// Session data: short TTL
cache.Set(ctx, "session:123", data, 15*time.Minute)

// Configuration: long TTL
cache.Set(ctx, "app:config", config, 24*time.Hour)

// Negative cache: short TTL
cache.Set(ctx, "user:notfound", negativeValue, 30*time.Second)
```

### 3. Use Namespaces for Isolation

```go
tenantCache := manager.Namespace(fmt.Sprintf("tenant:%s:", tenantID))
userKey := fmt.Sprintf("user:%s", userID)
tenantCache.Set(ctx, userKey, userData, time.Hour)
```

### 4. Handle Errors Gracefully

```go
val, err := cache.Get(ctx, key)
if err != nil {
    if cacheerrors.IsNotFound(err) {
        // Compute value
        val = computeValue()
        cache.Set(ctx, key, val, ttl)
    } else if cacheerrors.IsTimeout(err) {
        // Use stale value or fallback
        return fallbackValue, nil
    } else {
        return nil, err
    }
}
```

### 5. Monitor Cache Performance

```go
// Periodically log statistics
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        snap := cache.Stats()
        log.Printf("Cache: hits=%d, misses=%d, rate=%.2f%%, items=%d",
            snap.Hits, snap.Misses, snap.HitRate, snap.Items)

        if snap.HitRate < 80.0 {
            log.Printf("Warning: Low hit rate (%.2f%%)", snap.HitRate)
        }
    }
}()
```

### 6. Use GetOrSet for Expensive Computations

```go
// Bad: Check-then-act (race condition)
if _, err := cache.Get(ctx, key); cacheerrors.IsNotFound(err) {
    val := expensiveCompute()
    cache.Set(ctx, key, val, ttl)
}

// Good: Atomic GetOrSet
val, err := cache.GetOrSet(ctx, key, func() ([]byte, error) {
    return expensiveCompute(), nil
}, ttl)
```

### 7. Implement Circuit Breakers for Remote Caches

```go
policy := resilience.Policy{
    CircuitBreaker: resilience.NewCircuitBreaker(5, 30*time.Second),
    Retry: resilience.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
    },
    Timeout: 2 * time.Second,
}

resilientCache := resilience.NewCacheWithPolicy(redisCache, policy)
```

## API Reference

### Core Interfaces

```go
type Backend interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    TTL(ctx context.Context, key string) (time.Duration, error)
    GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
    SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
    DeleteMulti(ctx context.Context, keys ...string) error
    Ping(ctx context.Context) error
    Close(ctx context.Context) error
    Stats() stats.Snapshot
    Closed() bool
    Name() string
}

type AtomicBackend interface {
    Backend
    CompareAndSwap(ctx context.Context, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error)
    SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
    Increment(ctx context.Context, key string, delta int64) (int64, error)
    Decrement(ctx context.Context, key string, delta int64) (int64, error)
    GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error)
}

type ScanBackend interface {
    Backend
    Keys(ctx context.Context, pattern string) ([]string, error)
    Clear(ctx context.Context) error
    Size(ctx context.Context) (int64, error)
}
```

### Statistics Snapshot

```go
type Snapshot struct {
    Hits, Misses, Gets, Sets   int64
    Deletes, Evictions         int64
    Errors                     int64
    Items, Memory              int64
    HitRate, OpsPerSecond      float64
    Uptime                     time.Duration
    L1Hits, L1Misses, L1Errors int64
    L2Hits, L2Misses, L2Errors int64
    L2Promotions               int64
    L1HitRate, L2HitRate       float64
    WriteBackEnqueued          int64
    WriteBackFlushed           int64
    WriteBackDropped           int64
}
```

## Docker Development

```bash
# Start Redis development environment
make docker-up

# Run Redis CLI
make docker-redis-cli

# View Redis stats
make docker-stats

# Monitor Redis commands
make docker-monitor

# Flush Redis database
make docker-flush

# Stop services
make docker-down

# Full cleanup
make docker-prune
```

## License

This project is licensed under the MIT License.

---

**Maintained by [os-gomod](https://github.com/os-gomod)** | [Report Bug](https://github.com/os-gomod/config/issues) | [Request Feature](https://github.com/os-gomod/config/issues)
