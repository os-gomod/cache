// Package layer implements a layered cache with two tiers: L1 (fast, local)
package layer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/config"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/stampede"
)

type atomicScanBackend interface {
	backendiface.AtomicBackend
	backendiface.ScanBackend
}

type (
	atomicBackend = backendiface.AtomicBackend
	scanBackend   = backendiface.ScanBackend
)

type wbJob struct {
	key   string
	value []byte
	ttl   time.Duration
}

// Cache implements a two-tier cache with L1 (fast, local) and L2 (slower,
// shared) backends. Both tiers are accessed through the atomicScanBackend
// interface — no concrete types are used in the struct definition.
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

	noPromote sync.Map
	wbCh      chan wbJob
	wg        sync.WaitGroup
}

// New creates a layered cache with default L1 memory + L2 Redis backends.
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a layered cache with default L1 memory + L2 Redis
// backends, propagating ctx to backend construction and worker lifecycles.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return newFromConfigWithDefaults(ctx, cfg)
}

// NewWithBackends creates a layered cache from pre-constructed backends,
// enabling full dependency injection. Both l1 and l2 must implement
// atomicBackend and scanBackend.
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

	// Verify that both backends also implement scanBackend.
	l1Scan, ok := l1.(scanBackend)
	if !ok {
		return nil, _errors.New(
			"layered.new_with_backends",
			"",
			fmt.Errorf("L1 backend must implement ScanBackend"),
		)
	}
	l2Scan, ok := l2.(scanBackend)
	if !ok {
		return nil, _errors.New(
			"layered.new_with_backends",
			"",
			fmt.Errorf("L2 backend must implement ScanBackend"),
		)
	}

	// Wrap into tierBackend adapters.
	return newFromBackends(ctx, cfg, &tierAdapter{l1, l1Scan}, &tierAdapter{l2, l2Scan})
}

// NewFromConfig creates a layered cache from a config struct and two
// pre-constructed backends.
func NewFromConfig(
	cfg *config.Layered,
	l1 atomicScanBackend,
	l2 atomicScanBackend,
) (*Cache, error) {
	return newFromBackends(context.Background(), cfg, l1, l2)
}

// tierAdapter combines separate atomicBackend and scanBackend into a
// single atomicScanBackend interface.
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

func buildLayeredChain(interceptors any) *observability.Chain {
	var ics []observability.Interceptor
	if ic, ok := interceptors.([]observability.Interceptor); ok {
		ics = append(ics, ic...)
	}
	if len(ics) > 0 {
		return observability.NewChain(ics...)
	}
	return observability.NopChain()
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
		chain:    buildLayeredChain(cfg.Interceptors),
		sg:       singlefght.NewGroup(),
		detector: stampede.NewDetector(stampede.DefaultBeta, buildLayeredChain(cfg.Interceptors)),
	}
	if cfg.WriteBack {
		c.wbCh = make(chan wbJob, cfg.WriteBackQueueSize)
		c.startWriteBackWorkers(ctx)
	}
	return c, nil
}

// newFromConfigWithDefaults creates a layered cache from config.
func newFromConfigWithDefaults(ctx context.Context, cfg *config.Layered) (*Cache, error) {
	// Construct L1 (memory) backend from config.
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

	// Construct L2 (redis) backend from config.
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

	// Both memory.Cache and redis.Cache implement all the methods in
	// atomicScanBackend via duck typing. We just need to adapt them.
	var l1Scan scanBackend = l1
	var l2Scan scanBackend = l2

	return newFromBackends(
		ctx,
		cfg,
		&tierAdapter{atomicBackend: l1, scan: l1Scan},
		&tierAdapter{atomicBackend: l2, scan: l2Scan},
	)
}

// HealthCheck pings both tiers and returns any errors.
func (c *Cache) HealthCheck(ctx context.Context) (l1Err, l2Err error) {
	l1Err = c.l1.Ping(ctx)
	l2Err = c.l2.Ping(ctx)
	return l1Err, l2Err
}

func (c *Cache) checkClosed(op string) error {
	return c.guard.CheckClosed(op)
}

// SetInterceptors replaces the interceptor chain.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	if len(interceptors) > 0 {
		c.chain = observability.NewChain(interceptors...)
	} else {
		c.chain = observability.NopChain()
	}
}

func (c *Cache) Ping(ctx context.Context) error {
	if err := c.checkClosed("layered.ping"); err != nil {
		return err
	}
	if err := c.l1.Ping(ctx); err != nil {
		return err
	}
	return c.l2.Ping(ctx)
}

func (c *Cache) Name() string          { return "layered" }
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }
func (c *Cache) Closed() bool          { return c.guard.IsClosed() }

func (c *Cache) Close(ctx context.Context) error {
	if !c.guard.Close() {
		// guard.Close() returns false = was already closed; nothing to do.
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
