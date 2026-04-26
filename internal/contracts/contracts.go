// Package contracts defines the core interfaces and shared types used across
// all cache backends in the cache/v2 module. It provides the canonical Cache
// interface along with operation and result types for the execution pipeline.
package contracts

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Operation describes a cache operation flowing through the execution pipeline.
// It is used by the runtime.Executor and middleware.Chain to track what
// operation is being performed, on which key(s), and against which backend.
type Operation struct {
	// Name is the operation identifier (e.g., "get", "set", "delete").
	Name string
	// Key is the primary key being operated on (empty for batch operations).
	Key string
	// KeyCount is the number of keys involved in a batch operation.
	KeyCount int
	// Backend identifies the cache backend (e.g., "memory", "redis", "layered").
	Backend string
}

// Result describes the outcome of a cache operation, used by middleware
// interceptors to record metrics, traces, and other observability data.
type Result struct {
	// Value is the data returned by the operation (nil for non-read operations).
	Value []byte
	// Hit indicates whether a read operation found the key in the cache.
	Hit bool
	// ByteSize is the size in bytes of the value.
	ByteSize int
	// Latency is the time taken to execute the operation.
	Latency time.Duration
	// Err is any error that occurred during the operation.
	Err error
}

// StatsSnapshot is an immutable point-in-time capture of cache statistics.
// It is safe to read from any goroutine without synchronization.
type StatsSnapshot struct {
	Hits        int64
	Misses      int64
	Sets        int64
	Deletes     int64
	Evictions   int64
	Errors      int64
	Items       int64
	MemoryBytes int64
	MaxMemory   int64
	StartTime   time.Time
}

// Reader defines read-only cache operations.
type Reader interface {
	// Get retrieves a value by key. Returns ErrNotFound if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// GetMulti retrieves multiple values by key. Missing keys are omitted from
	// the returned map.
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)

	// Exists reports whether a key exists and is not expired.
	Exists(ctx context.Context, key string) (bool, error)

	// TTL returns the remaining time-to-live for a key.
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// Writer defines mutation operations on the cache.
type Writer interface {
	// Set stores a key-value pair with the given TTL. A zero or negative TTL
	// means the entry never expires.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// SetMulti stores multiple key-value pairs atomically with the given TTL.
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// Delete removes a key from the cache. Deleting a non-existent key is a no-op.
	Delete(ctx context.Context, key string) error

	// DeleteMulti removes multiple keys from the cache.
	DeleteMulti(ctx context.Context, keys ...string) error
}

// AtomicOps defines atomic operations for concurrency-safe cache manipulation.
type AtomicOps interface {
	// CompareAndSwap atomically replaces oldVal with newVal if the current value
	// matches oldVal. Returns true if the swap was performed.
	CompareAndSwap(
		ctx context.Context,
		key string,
		oldVal, newVal []byte,
		ttl time.Duration,
	) (bool, error)

	// SetNX sets a key-value pair only if the key does not already exist.
	// Returns true if the key was set.
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)

	// Increment atomically increments a numeric value by delta and returns the
	// new value. If the key does not exist, it is initialized to 0 before
	// incrementing.
	Increment(ctx context.Context, key string, delta int64) (int64, error)

	// Decrement atomically decrements a numeric value by delta and returns the
	// new value.
	Decrement(ctx context.Context, key string, delta int64) (int64, error)

	// GetSet atomically sets a new value and returns the old value.
	GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error)
}

// Scanner defines bulk inspection and management operations.
type Scanner interface {
	// Keys returns all non-expired keys matching the given glob pattern.
	// An empty pattern matches all keys.
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Clear removes all entries from the cache and resets statistics.
	Clear(ctx context.Context) error

	// Size returns the number of non-expired entries in the cache.
	Size(ctx context.Context) (int64, error)
}

// Lifecycle defines the start/stop lifecycle of a cache backend.
type Lifecycle interface {
	// Ping checks whether the cache is alive and reachable. Returns an error
	// if the cache is closed or the backend is unreachable.
	Ping(ctx context.Context) error

	// Close gracefully shuts down the cache, releasing all resources.
	// It is safe to call Close multiple times.
	Close(ctx context.Context) error

	// Closed reports whether the cache has been closed.
	Closed() bool

	// Name returns the backend identifier string (e.g., "memory", "redis").
	Name() string
}

// HealthChecker defines a lightweight health probe interface for backends.
// It is used by the instrumentation health checker to determine liveness.
type HealthChecker interface {
	// Check performs a lightweight health probe. Returns nil if the
	// backend is reachable and operational.
	Check(ctx context.Context) error
}

// StatsProvider defines the interface for retrieving cache statistics.
type StatsProvider interface {
	// Stats returns an immutable snapshot of the cache's current statistics.
	Stats() StatsSnapshot
}

// HitRate returns the cache hit rate as a float between 0.0 and 1.0.
// Returns 0 if no get operations have been performed.
func (s *StatsSnapshot) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// ReadOnly is a convenience alias for the read-only subset of cache operations.
type ReadOnly = Reader

// ReadWrite is a convenience alias for read and write operations combined.
type ReadWrite interface {
	Reader
	Writer
}

// Cache is the composite interface that all cache backends must implement.
// It combines read, write, atomic, scan, lifecycle, and stats operations.
type Cache interface {
	Reader
	Writer
	AtomicOps
	Scanner
	Lifecycle
	StatsProvider
}

// ---------------------------------------------------------------------------
// No-op test helpers
// ---------------------------------------------------------------------------

// NoopTracerProvider is a no-op OpenTelemetry TracerProvider for use in
// unit tests where real tracing is not needed.
type NoopTracerProvider struct{}

// Tracer returns a no-op tracer.
func (NoopTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return noop.NewTracerProvider().Tracer(name, opts...)
}
