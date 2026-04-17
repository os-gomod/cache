// Package redis implements a Redis-backed cache with atomic operations, pipeline
// support, Lua scripts, and distributed stampede protection.
package redis

import (
	"context"
	"sync/atomic"

	goredis "github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/chain"
	"github.com/os-gomod/cache/internal/keyutil"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/stampede"
)

// Cache is a Redis-backed cache implementation that supports basic operations,
// atomic operations (CAS, SetNX, Increment), batch operations, and structured
// data operations (hash, list, set, sorted set).
//
// The cache is safe for concurrent use by multiple goroutines.
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

// New creates a new Redis cache with the given options.
// It is equivalent to NewWithContext(context.Background(), opts...).
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a new Redis cache with the given context and options.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

// NewWithConfig creates a new Redis cache from a config with a background context.
func NewWithConfig(cfg *config.Redis) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

// NewWithConfigContext creates a new Redis cache from a config with the given context.
// If cfg is nil, defaults are applied. The config is cloned to prevent mutation.
// It pings the Redis server to verify connectivity before returning.
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
	return &Cache{
		cfg:          *cfg,
		client:       client,
		stats:        stats.NewStats(),
		chain:        chain.BuildChain(cfg.Interceptors),
		sg:           singlefght.NewGroup(),
		detector:     stampede.NewDetector(stampede.DefaultBeta, chain.BuildChain(cfg.Interceptors)),
		casScript:    goredis.NewScript(casLuaScript),
		getSetScript: goredis.NewScript(getSetPersistLuaScript),
		unlockScript: goredis.NewScript(unlockIfValueMatchesLuaScript),
	}, nil
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

// SetInterceptors replaces the observability interceptors for this cache.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	c.chain = chain.SetInterceptors(c.chain, interceptors)
}

// Name returns the backend identifier for this cache ("redis").
func (c *Cache) Name() string { return "redis" }

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.guard.IsClosed() }

// Client returns the underlying Redis client for advanced operations.
func (c *Cache) Client() goredis.UniversalClient { return c.client }

// Close releases the Redis connection and stops background workers.
// It is safe to call Close multiple times.
func (c *Cache) Close(_ context.Context) error {
	if c.guard.Close() {
		return nil
	}
	c.detector.Close()
	return c.client.Close()
}
