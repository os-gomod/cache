// Package layered provides a two-level (L1 in-memory + L2 Redis) cache with
// optional write-back, negative caching, and Pub/Sub invalidation.
package layered

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/cache/config"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
)

// wbJob is a single pending async write to L2.
type wbJob struct {
	key   string
	value []byte
	ttl   time.Duration
}

// Cache is a two-level (L1 + L2) cache.
//
// Concurrency model:
//   - l1 and l2 are independently concurrency-safe.
//   - noPromote is a sync.Map tracking keys that must not be promoted after a
//     CAS on L2, preventing a stale L1 entry from being re-promoted.
//   - wbCh is closed by Close(), which signals write-back workers to drain.
//   - closed is an atomic bool so checkClosed never acquires a lock.
type Cache struct {
	cfg config.Layered
	l1  *memory.Cache
	l2  *redis.Cache

	stats     *stats.Stats
	sg        *singlefght.Group
	noPromote sync.Map // key → struct{}, cleared on first read after CAS
	wbCh      chan wbJob
	wg        sync.WaitGroup
	closed    atomic.Bool
}

// ----------------------------------------------------------------------------
// Constructors
// ----------------------------------------------------------------------------

// New creates a layered cache with functional options.
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a layered cache with functional options and a context.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

// NewWithConfig creates a layered cache from an explicit config struct.
func NewWithConfig(cfg *config.Layered) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

// NewWithConfigContext is the canonical constructor; all other New* functions
// delegate here.
func NewWithConfigContext(ctx context.Context, cfg *config.Layered) (*Cache, error) {
	if cfg == nil {
		cfg = config.DefaultLayered()
	}
	cfg = cfg.Clone()
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ctx = cachectx.NormalizeContext(ctx)

	l1, err := memory.NewWithConfigContext(ctx, cfg.L1Config)
	if err != nil {
		return nil, err
	}

	l2, err := redis.NewWithConfigContext(ctx, cfg.L2Config)
	if err != nil {
		_ = l1.Close(ctx)
		return nil, err
	}

	c := &Cache{
		cfg:   *cfg,
		l1:    l1,
		l2:    l2,
		stats: stats.NewStats(),
		sg:    singlefght.NewGroup(),
	}

	if cfg.WriteBack {
		c.wbCh = make(chan wbJob, cfg.WriteBackQueueSize)
		c.startWriteBackWorkers(ctx)
	}

	return c, nil
}

// checkClosed returns a cache-closed error when the cache has been shut down.
func (c *Cache) checkClosed(op string) error {
	if c.closed.Load() {
		return _errors.Closed(op)
	}
	return nil
}

// Stats returns a point-in-time snapshot of layered cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.closed.Load() }

// Close shuts down both layers and waits for write-back workers to drain.
func (c *Cache) Close(ctx context.Context) error {
	if c.closed.Swap(true) {
		return nil // already closed
	}

	// Close the write-back channel so workers drain remaining jobs and exit.
	if c.wbCh != nil {
		close(c.wbCh)
	}
	c.wg.Wait()

	// Close L1 and L2; collect first non-nil error.
	var firstErr error
	if err := c.l1.Close(ctx); err != nil {
		firstErr = err
	}
	if err := c.l2.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// L1 returns the underlying L1 (memory) cache.
func (c *Cache) L1() *memory.Cache { return c.l1 }

// L2 returns the underlying L2 (Redis) cache.
func (c *Cache) L2() *redis.Cache { return c.l2 }
