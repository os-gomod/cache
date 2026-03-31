// Package redis provides a Redis-backed cache.
package redis

import (
	"context"
	"sync/atomic"

	goredis "github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/config"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
)

// Cache is a Redis-backed cache.
//
// Concurrency model: all Redis commands are dispatched through go-redis which
// manages its own connection pool; no additional locking is required at this
// layer.  closed is an atomic bool so checkClosed never acquires a lock.
type Cache struct {
	cfg    config.Redis
	client goredis.UniversalClient
	closed atomic.Bool

	stats            *stats.Stats
	sg               *singlefght.Group
	casScript        *goredis.Script
	getSetScript     *goredis.Script
	unlockScript     *goredis.Script
	stampedeTokenSeq atomic.Uint64
}

// ----------------------------------------------------------------------------
// Constructors
// ----------------------------------------------------------------------------

// New creates a Redis cache with functional options.
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a Redis cache with functional options and a context.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

// NewWithConfig creates a Redis cache from an explicit config struct.
func NewWithConfig(cfg *config.Redis) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

// NewWithConfigContext is the canonical constructor used by all other New*
// functions.  It clones the config, applies defaults, validates, and
// constructs the cache.
func NewWithConfigContext(ctx context.Context, cfg *config.Redis) (*Cache, error) {
	if cfg == nil {
		cfg = config.DefaultRedis()
	}
	cfg = cfg.Clone()
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := buildClient(cfg)

	// Probe connectivity.
	if err := client.Ping(cachectx.NormalizeContext(ctx)).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Cache{
		cfg:          *cfg,
		client:       client,
		stats:        stats.NewStats(),
		sg:           singlefght.NewGroup(),
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
	// Comma-separated for cluster mode.
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

// buildKey prepends the configured key prefix.
func (c *Cache) buildKey(key string) string {
	if c.cfg.KeyPrefix == "" {
		return key
	}
	return c.cfg.KeyPrefix + key
}

// stripPrefix removes the key prefix from a key returned by KEYS/SCAN.
func (c *Cache) stripPrefix(key string) string {
	if c.cfg.KeyPrefix == "" || len(key) <= len(c.cfg.KeyPrefix) {
		return key
	}
	return key[len(c.cfg.KeyPrefix):]
}

// buildStampedeLockKey returns the lock key used by distributed stampede protection.
func (c *Cache) buildStampedeLockKey(key string) string {
	return c.buildKey(key) + ":__lock__"
}

// checkClosed returns a cache-closed error when the cache has been shut down.
func (c *Cache) checkClosed(op string) error {
	if c.closed.Load() {
		return _errors.Closed(op)
	}
	return nil
}

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.closed.Load() }

// Close closes the Redis connection pool.
func (c *Cache) Close(_ context.Context) error {
	if c.closed.Swap(true) {
		return nil
	}
	return c.client.Close()
}
