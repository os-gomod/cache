// Package backendiface defines the core Backend interface and related contracts.
package backendiface

import (
	"context"
	"time"

	"github.com/os-gomod/cache/internal/stats"
)

// Backend is the single contract all cache implementations must satisfy.
// Every method must be safe for concurrent use and must respect ctx
// cancellation.
type Backend interface {
	// Core KV
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Batch
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error

	// Lifecycle
	Ping(ctx context.Context) error
	Close(ctx context.Context) error

	// Observability
	Stats() stats.Snapshot
	Closed() bool

	// Name returns the backend identifier (e.g., "memory", "redis",
	// "layered", "resilience"). This replaces type-assertion-based
	// detection and is safe for hot-path use.
	Name() string
}

// AtomicBackend extends Backend with compare-and-swap semantics.
type AtomicBackend interface {
	Backend

	CompareAndSwap(
		ctx context.Context,
		key string,
		oldVal, newVal []byte,
		ttl time.Duration,
	) (bool, error)
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)
	GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error)
}

// ScanBackend extends Backend with key enumeration.
type ScanBackend interface {
	Backend

	Keys(ctx context.Context, pattern string) ([]string, error)
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
}
