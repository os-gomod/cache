# Architecture Guide

## System Overview

`os-gomod/cache v2` is an enterprise-grade, high-performance caching platform for Go applications. It provides a unified API surface over pluggable storage backends, composable middleware for cross-cutting concerns, and a rich set of enterprise extensions. The system is built on Go 1.22+ generics for type-safe cache operations while maintaining backward compatibility and zero raw error exposure.

### Design Principles

1. **Unified Execution Runtime**: Every cache operation flows through a single execution pipeline regardless of backend. This eliminates code duplication and ensures consistent behavior for metrics, tracing, error handling, and middleware.

2. **Capability-Based Contracts**: Instead of a monolithic interface, the system decomposes cache capabilities into fine-grained interfaces (`Reader`, `Writer`, `AtomicOps`, `Scanner`, `Lifecycle`, `StatsProvider`). Implementations choose which capabilities to support.

3. **Middleware Pipeline**: Cross-cutting concerns (retry, circuit breaking, rate limiting, metrics, logging, tracing) are implemented as composable middleware that wrap the execution pipeline without modifying core logic.

4. **Type Safety Through Generics**: The `TypedCache[T]` wrapper provides compile-time type safety for cache values, using pluggable codecs for serialization. Specialized codecs for `string`, `int64`, and `[]byte` avoid JSON overhead.

5. **Zero Raw Errors**: All errors are created through a centralized `ErrorFactory` that produces structured `CacheError` values with operation context, machine-readable codes, and chainable causes.

---

## Package Ownership Map

| Package | Responsibility |
|---------|---------------|
| `cache` | **Public API** — the clean facade consumers import. Provides `Backend`, `TypedCache[T]`, `Manager`, `Namespace`, resilience helpers, and enterprise extensions. |
| `internal/contracts` | **Capability Interfaces** — defines `Reader`, `Writer`, `AtomicOps`, `Scanner`, `Lifecycle`, `StatsProvider`, the composite `Cache` interface, and shared types (`Operation`, `Result`, `StatsSnapshot`). |
| `memory` | **Memory Backend** — in-process sharded cache with pluggable eviction policies (LRU, LFU, FIFO, TinyLFU). Uses `sync.RWMutex` per shard for high concurrency. |
| `redis` | **Redis Backend** — distributed cache backed by Redis. Supports pipelining, connection pooling, key prefixing, and cluster mode. |
| `layered` | **Layered Backend** — two-tier cache (L1 memory + L2 Redis) with read-through, write-through, and write-back modes. Automatic L1 promotion on L2 hits. |
| `internal/middleware` | **Middleware Framework** — provides `Middleware` type, `Chain` builder, and production middleware: retry, circuit breaker, rate limiter, metrics, logging, tracing. |
| `internal/serialization` | **Serialization Pipeline** — defines the `Codec[T]` interface and provides `RawCodec`, `StringCodec`, `Int64Codec`, `Float64Codec`, `JSONCodec[T]`. Includes `BufPool` for zero-allocation encoding. |
| `internal/errors` | **Error Factory** — centralized error creation with structured `CacheError` type. Provides semantic error methods: `NotFound`, `EncodeFailed`, `DecodeFailed`, `BackendNotFound`, etc. |
| `internal/lifecycle` | **Lifecycle Management** — provides `Guard` for safe close semantics, preventing double-close and operations-after-close. |
| `internal/policy` | **Eviction Policies** — pluggable eviction algorithm implementations. Each policy implements a `Policy` interface with `Access()` and `Evict()` methods. |

---

## Dependency Graph

```
cache
├── internal/contracts        (interfaces + shared types)
├── memory     (memory backend)
├── redis      (redis backend)
├── layered    (layered backend)
├── internal/middleware        (middleware framework)
├── internal/serialization    (codec framework)
├── internal/errors           (error factory)
└── internal/lifecycle        (close guard)

layered
├── internal/contracts
├── memory     (as L1)
└── redis      (as L2)

internal/middleware
└── internal/contracts

memory
├── internal/contracts
├── internal/serialization
├── internal/errors
├── internal/lifecycle
└── internal/policy
```

The dependency graph is strictly acyclic. The `internal/` packages never depend on `cache`, ensuring the public API remains a thin facade with no circular dependencies.

---

## Capability-Based Contracts

The `Cache` interface composes six fine-grained capability interfaces:

```go
type Cache interface {
    Reader        // Get, GetMulti, Exists, TTL
    Writer        // Set, SetMulti, Delete, DeleteMulti
    AtomicOps     // CompareAndSwap, SetNX, Increment, Decrement, GetSet
    Scanner       // Keys, Clear, Size
    Lifecycle     // Ping, Close, Closed, Name
    StatsProvider // Stats()
}
```

This design enables:
- **Partial implementations**: A read-only cache can implement only `Reader` + `Lifecycle`.
- **Testing**: Mock implementations only need to implement the capabilities used by the test.
- **Documentation**: Each capability group has clear semantic boundaries.

---

## Unified Execution Runtime Flow

Every cache operation follows the same execution path:

```
Client Call
    │
    ▼
┌─────────────┐
│  Middleware  │  (rate limiter → circuit breaker → retry → metrics → tracing)
│   Pipeline   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Serialization│  (TypedCache[T] only: encode on write, decode on read)
│   Layer      │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Backend    │  (memory / redis / layered)
│  Execution   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Error      │  (wrap in CacheError via ErrorFactory)
│  Handling    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Statistics  │  (record operation: hit/miss/error/latency)
│  Recording   │
└─────────────┘
```

---

## Middleware Pipeline Architecture

Middleware is implemented using a standard chain-of-responsibility pattern:

```go
type Middleware func(Handler) Handler
type Chain struct { middlewares []Middleware }
```

Middleware is applied in order — the first middleware in the chain is the outermost wrapper. Common stacks:

```
Request → RateLimiter → CircuitBreaker → Retry → Metrics → Tracing → Backend → Response
```

Each middleware can:
- **Short-circuit**: Reject a request before it reaches the backend (rate limiter, circuit breaker).
- **Modify**: Transform the request or response (compression).
- **Observe**: Record metrics without modifying behavior (Prometheus, logging).
- **Retry**: Replay a failed request (retry middleware).

---

## Store Implementations

### Memory Store
- **Sharding**: Data is partitioned across N internal shards (default: 64) using FNV-1a hashing of the key. Each shard has its own `sync.RWMutex` for maximum parallelism.
- **Eviction**: Pluggable policies (`internal/policy`): LRU (doubly-linked list + hashmap), LFU (frequency counter + min-heap), FIFO (queue), TinyLFU (count-min sketch + LRU).
- **TTL**: Lazy expiration — expired entries are removed on access. A background goroutine periodically scans for expired entries.
- **Memory safety**: Hard limit on total bytes stored. Eviction is triggered when the limit is reached.

### Redis Store
- **Connection pooling**: Uses `github.com/redis/go-redis/v9` for production-grade pooling, pipelining, and cluster support.
- **Key prefixing**: All keys are automatically prefixed (configurable) for namespace isolation.
- **Batch operations**: Uses Redis MGET/MSET pipelines for efficient multi-key operations.

### Layered Store
- **Read path**: Check L1 → miss? Check L2 → hit? Promote to L1 → return.
- **Write path**: Write to L1 and L2 simultaneously (write-through mode).
- **Write-back mode**: Writes go to L1 only; dirty entries are flushed to L2 asynchronously.
- **Promotion**: L2 hits are automatically promoted to L1 with the L1 TTL set to a fraction of the L2 TTL.

---

## Policy Engine Architecture

Eviction policies implement a common interface:

```go
type Policy interface {
    Access(key string)           // record access for ordering
    Evict(candidates []Entry) Entry  // select entry to evict
    Name() string
}
```

Policies are stateless per shard — each shard has its own policy instance. The TinyLFU policy uses a Count-Min Sketch for frequency estimation (bounded memory) combined with a LRU list for recency.

---

## Serialization Pipeline

The `Codec[T]` interface supports two-pass encoding with buffer reuse:

```go
type Codec[T any] interface {
    Encode(T, []byte) ([]byte, error)  // encode value, optionally into provided buffer
    Decode([]byte) (T, error)          // decode from bytes
    Name() string
}
```

The `BufPool` provides a sync.Pool of byte buffers to reduce GC pressure during high-throughput encoding. Specialized codecs (`StringCodec`, `Int64Codec`) use direct byte conversion without JSON overhead.

---

## Error Handling Strategy

All errors flow through the `ErrorFactory`:

```go
var Factory ErrorFactory

func (f ErrorFactory) NotFound(key string) *CacheError
func (f ErrorFactory) EncodeFailed(key, codec string, err error) *CacheError
func (f ErrorFactory) DecodeFailed(key, codec string, err error) *CacheError
func (f ErrorFactory) BackendNotFound(name string) *CacheError
func (f ErrorFactory) InvalidConfig(msg string) *CacheError
func (f ErrorFactory) AlreadyClosed(component string) *CacheError
```

`CacheError` provides:
- `Code`: Machine-readable error code for programmatic handling.
- `Operation`: The cache operation that failed.
- `Message`: Human-readable description.
- `Err`: Underlying cause (implements `errors.Unwrap`).
- `Is()`: Supports `errors.Is()` for type-based error checking.

---

## Concurrency Model

- **Memory store**: Sharded `sync.RWMutex` — reads within the same shard are concurrent, writes are exclusive per shard.
- **TypedCache**: No additional locking — concurrency is delegated to the underlying backend.
- **Manager**: `sync.RWMutex` protects the backend map. Backend operations are lock-free after lookup.
- **HotKeyDetector**: `sync.RWMutex` for counter map + `atomic.Int64` for per-key counters.
- **AdaptiveTTL**: `sync.Map` for lock-free access tracking + `atomic.Int64` for scores.
- **Lifecycle Guard**: Uses atomic CAS for single-writer close semantics.

---

## Observability Pipeline

The observability pipeline is built into the middleware layer:

1. **Metrics Middleware**: Records operation counts, latencies, hit/miss rates. Pluggable `MetricsRecorder` interface supports Prometheus, StatsD, or in-memory recorders.
2. **Tracing Middleware**: Creates OpenTelemetry spans for each operation with attributes (`cache.operation`, `cache.key`, `cache.backend`, `cache.hit`).
3. **Logging Middleware**: Structured JSON logging with operation context. Pluggable logger interface supports slog, zap, or zerolog.
4. **Stats**: Every backend implements `StatsProvider` for point-in-time statistics snapshots.
