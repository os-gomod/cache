// Package redis provides a Redis-backed cache with support for batch
// operations, singleflight-assisted GetOrSet, optional distributed locks,
// compare-and-swap, counters, SetNX, and Redis collection helpers. It also
// exposes observability hooks for monitoring cache usage and latency.
package redis

import (
	"context"
	"sync/atomic"

	goredis "github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/keyutil"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/stampede"
)

type Cache struct {
	cfg                  config.Redis
	client               goredis.UniversalClient
	guard                lifecycle.Guard
	stats                *stats.Stats
	chain                *observability.Chain
	explicitInterceptors []observability.Interceptor
	sg                   *singlefght.Group
	detector             *stampede.Detector
	casScript            *goredis.Script
	getSetScript         *goredis.Script
	unlockScript         *goredis.Script
	stampedeTokenSeq     atomic.Uint64
}

func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

func NewWithConfig(cfg *config.Redis) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

func NewWithConfigContext(ctx context.Context, cfg *config.Redis) (*Cache, error) {
	if cfg == nil {
		cfg = config.DefaultRedis()
	}
	cfg = cfg.Clone()
	if err := config.Apply(cfg); err != nil {
		return nil, err
	}
	client := buildClient(cfg)
	if err := client.Ping(cachectx.NormalizeContext(ctx)).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	chain := buildChain(cfg.Interceptors)

	return &Cache{
		cfg:          *cfg,
		client:       client,
		stats:        stats.NewStats(),
		chain:        chain,
		sg:           singlefght.NewGroup(),
		detector:     stampede.NewDetector(stampede.DefaultBeta, chain),
		casScript:    goredis.NewScript(casLuaScript),
		getSetScript: goredis.NewScript(getSetPersistLuaScript),
		unlockScript: goredis.NewScript(unlockIfValueMatchesLuaScript),
	}, nil
}

func buildChain(interceptors any) *observability.Chain {
	var ics []observability.Interceptor
	if ic, ok := interceptors.([]observability.Interceptor); ok {
		ics = append(ics, ic...)
	}
	if len(ics) > 0 {
		return observability.NewChain(ics...)
	}
	return observability.NopChain()
}

func buildClient(cfg *config.Redis) goredis.UniversalClient {
	opts := &goredis.UniversalOptions{
		Addrs:        splitAddrs(cfg.Addr),
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return goredis.NewUniversalClient(opts)
}

func splitAddrs(addr string) []string {
	if addr == "" {
		return []string{"localhost:6379"}
	}
	var addrs []string
	start := 0
	for i := 0; i < len(addr); i++ {
		if addr[i] == ',' {
			addrs = append(addrs, addr[start:i])
			start = i + 1
		}
	}
	addrs = append(addrs, addr[start:])
	return addrs
}

func (c *Cache) buildKey(key string) string {
	return keyutil.BuildKey(c.cfg.KeyPrefix, key)
}

func (c *Cache) stripPrefix(key string) string {
	return keyutil.StripPrefix(c.cfg.KeyPrefix, key)
}

func (c *Cache) buildStampedeLockKey(key string) string {
	return keyutil.StampedeLockKey(c.cfg.KeyPrefix, key)
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

func (c *Cache) Name() string                    { return "redis" }
func (c *Cache) Stats() stats.Snapshot           { return c.stats.TakeSnapshot() }
func (c *Cache) Closed() bool                    { return c.guard.IsClosed() }
func (c *Cache) Client() goredis.UniversalClient { return c.client }

func (c *Cache) Close(_ context.Context) error {
	if c.guard.Close() {
		return nil
	}
	c.detector.Close()
	return c.client.Close()
}
