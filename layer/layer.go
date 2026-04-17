// Package layer implements a layered (L1 memory + L2 Redis) cache with promotion,
// write-back support, stampede protection, and negative caching.
package layer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/config"
	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/chain"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/stampede"
)

// atomicScanBackend combines AtomicBackend and ScanBackend into a single interface
// for the layered cache's tier adapters.
type atomicScanBackend interface {
	backendiface.AtomicBackend
	backendiface.ScanBackend
}

type (
	atomicBackend = backendiface.AtomicBackend
	scanBackend   = backendiface.ScanBackend
)

// wbJob represents a write-back operation queued for asynchronous L2 writes.
type wbJob struct {
	key   string
	value []byte
	ttl   time.Duration
}

// Cache is a layered (L1 memory + L2 Redis) cache with optional write-back
// support, L1 promotion on L2 hits, stampede protection, and negative caching.
//
// The cache is safe for concurrent use by multiple goroutines.
type Cache struct {
	cfg                  config.Layered
	l1                   atomicScanBackend
	l2                   atomicScanBackend
	guard                lifecycle.Guard
	stats                *stats.Stats
	chain                *observability.Chain
	explicitInterceptors []observability.Interceptor
	sg                   *singlefght.Group
	detector             *stampede.Detector
	noPromote            sync.Map
	wbCh                 chan wbJob
	wg                   sync.WaitGroup
}

// New creates a new layered cache with the given options.
// It is equivalent to NewWithContext(context.Background(), opts...).
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a new layered cache with the given context and options.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return newFromConfigWithDefaults(ctx, cfg)
}

// NewWithBackends creates a layered cache with explicitly provided L1 and L2 backends.
// Both backends must implement ScanBackend; otherwise an error is returned.
func NewWithBackends(
	ctx context.Context,
	l1 atomicBackend,
	l2 atomicBackend,
	opts ...Option,
) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	l1Scan, ok := l1.(scanBackend)
	if !ok {
		return nil, cacheerrors.New(
			"layered.new_with_backends",
			"",
			fmt.Errorf("L1 backend must implement ScanBackend"),
		)
	}
	l2Scan, ok := l2.(scanBackend)
	if !ok {
		return nil, cacheerrors.New(
			"layered.new_with_backends",
			"",
			fmt.Errorf("L2 backend must implement ScanBackend"),
		)
	}
	return newFromBackends(ctx, cfg, &tierAdapter{l1, l1Scan}, &tierAdapter{l2, l2Scan})
}

// NewFromConfig creates a layered cache from an existing config and two backends.
// This is useful when backends are created externally (e.g., for testing).
func NewFromConfig(
	cfg *config.Layered,
	l1 atomicScanBackend,
	l2 atomicScanBackend,
) (*Cache, error) {
	return newFromBackends(context.Background(), cfg, l1, l2)
}

// tierAdapter wraps an AtomicBackend and adds ScanBackend capabilities.
type tierAdapter struct {
	atomicBackend
	scan scanBackend
}

func (t *tierAdapter) Keys(ctx context.Context, pattern string) ([]string, error) {
	return t.scan.Keys(ctx, pattern)
}

func (t *tierAdapter) Clear(ctx context.Context) error {
	return t.scan.Clear(ctx)
}

func (t *tierAdapter) Size(ctx context.Context) (int64, error) {
	return t.scan.Size(ctx)
}

func newFromBackends(
	ctx context.Context,
	cfg *config.Layered,
	l1 atomicScanBackend,
	l2 atomicScanBackend,
) (*Cache, error) {
	c := &Cache{
		cfg:      *cfg,
		l1:       l1,
		l2:       l2,
		guard:    lifecycle.Guard{},
		stats:    stats.NewStats(),
		chain:    chain.BuildChain(cfg.Interceptors),
		sg:       singlefght.NewGroup(),
		detector: stampede.NewDetector(stampede.DefaultBeta, chain.BuildChain(cfg.Interceptors)),
	}
	if cfg.WriteBack {
		c.wbCh = make(chan wbJob, cfg.WriteBackQueueSize)
		c.startWriteBackWorkers(ctx)
	}
	return c, nil
}

func newFromConfigWithDefaults(ctx context.Context, cfg *config.Layered) (*Cache, error) {
	l1, err := memory.NewWithContext(
		ctx,
		memory.WithMaxEntries(cfg.L1Config.MaxEntries),
		memory.WithTTL(cfg.L1Config.DefaultTTL),
		memory.WithCleanupInterval(cfg.L1Config.CleanupInterval),
		memory.WithShards(cfg.L1Config.ShardCount),
	)
	if err != nil {
		return nil, fmt.Errorf("layered: failed to create L1 backend: %w", err)
	}
	l2, err := redis.NewWithContext(
		ctx,
		redis.WithAddress(cfg.L2Config.Addr),
		redis.WithPassword(cfg.L2Config.Password),
		redis.WithDB(cfg.L2Config.DB),
		redis.WithPoolSize(cfg.L2Config.PoolSize),
		redis.WithTTL(cfg.L2Config.DefaultTTL),
	)
	if err != nil {
		_ = l1.Close(ctx)
		return nil, fmt.Errorf("layered: failed to create L2 backend: %w", err)
	}
	var l1Scan scanBackend = l1
	var l2Scan scanBackend = l2
	return newFromBackends(
		ctx,
		cfg,
		&tierAdapter{atomicBackend: l1, scan: l1Scan},
		&tierAdapter{atomicBackend: l2, scan: l2Scan},
	)
}

// HealthCheck pings both L1 and L2 backends, returning any errors.
func (c *Cache) HealthCheck(ctx context.Context) (l1Err, l2Err error) {
	l1Err = c.l1.Ping(ctx)
	l2Err = c.l2.Ping(ctx)
	return l1Err, l2Err
}

func (c *Cache) checkClosed(op string) error {
	return c.guard.CheckClosed(op)
}

// SetInterceptors replaces the observability interceptors for this cache.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	c.chain = chain.SetInterceptors(c.chain, interceptors)
}

// Ping checks whether both L1 and L2 backends are reachable.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.checkClosed("layered.ping"); err != nil {
		return err
	}
	if err := c.l1.Ping(ctx); err != nil {
		return err
	}
	return c.l2.Ping(ctx)
}

// Name returns the backend identifier for this cache ("layered").
func (c *Cache) Name() string { return "layered" }

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.guard.IsClosed() }

// Close flushes pending write-back jobs, stops all background workers,
// and closes both L1 and L2 backends. It is safe to call multiple times.
func (c *Cache) Close(ctx context.Context) error {
	if !c.guard.Close() {
		return nil
	}
	if c.wbCh != nil {
		close(c.wbCh)
	}
	c.wg.Wait()
	c.detector.Close()
	var firstErr error
	if err := c.l1.Close(ctx); err != nil {
		firstErr = err
	}
	if err := c.l2.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
