// Package backendiface defines the core interfaces that all cache backends must implement.
package backendiface

import (
	"context"
	"time"

	"github.com/os-gomod/cache/internal/stats"
)

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
type ScanBackend interface {
	Backend
	Keys(ctx context.Context, pattern string) ([]string, error)
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
}
