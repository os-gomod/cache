// Package memory implements a high-performance, multi-shard, in-memory cache
// with pluggable eviction policies and TTL-based expiration.
package memory

import (
	"context"
	"sync"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/chain"
	"github.com/os-gomod/cache/internal/hash"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/stampede"
)

// Cache is a multi-shard in-memory cache with pluggable eviction policies.
// It provides O(1) access via consistent hashing across shards and supports
// TTL-based expiration with background janitor cleanup.
//
// The cache is safe for concurrent use by multiple goroutines.
type Cache struct {
	cfg                  config.Memory
	guard                lifecycle.Guard
	stats                *stats.Stats
	chain                *observability.Chain
	explicitInterceptors []observability.Interceptor
	shards               []*shard
	sg                   *singlefght.Group
	detector             *stampede.Detector
	wg                   sync.WaitGroup
	stopCh               chan struct{}
	maxSize              int64
	nShards              int
	shardMask            uint32
}

// shard is a single shard of the sharded cache, protected by its own mutex.
type shard struct {
	mu    sync.RWMutex
	items map[string]*eviction.Entry
	evict eviction.Evictor
	size  int64
	count int64
}

// New creates a new in-memory cache with the given options.
// It is equivalent to NewWithContext(context.Background(), opts...).
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a new in-memory cache with the given context and options.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

// NewWithConfig creates a new cache from a config with a background context.
func NewWithConfig(cfg *config.Memory) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

// NewWithConfigContext creates a new cache from a config with the given context.
// If cfg is nil, defaults are applied. The config is cloned to prevent mutation.
func NewWithConfigContext(ctx context.Context, cfg *config.Memory) (*Cache, error) {
	if cfg == nil {
		cfg = config.DefaultMemory()
	}
	cfg = cfg.Clone()
	if err := config.Apply(cfg); err != nil {
		return nil, err
	}
	return newFromConfig(cachectx.NormalizeContext(ctx), cfg)
}

func newFromConfig(_ context.Context, cfg *config.Memory) (*Cache, error) {
	nShards := hash.NormalizeShards(cfg.ShardCount)
	perShardMem := cfg.MaxMemoryBytes / int64(nShards)
	perShardEntries := int64(cfg.MaxEntries) / int64(nShards)
	shards := make([]*shard, nShards)
	for i := range shards {
		shards[i] = &shard{
			items: make(map[string]*eviction.Entry, max(64, int(perShardEntries))),
			evict: eviction.New(cfg.EvictionPolicy, perShardMem),
		}
	}
	c := &Cache{
		cfg:       *cfg,
		guard:     lifecycle.Guard{},
		shards:    shards,
		stats:     stats.NewStats(),
		chain:     chain.BuildChain(cfg.Interceptors),
		sg:        singlefght.NewGroup(),
		detector:  stampede.NewDetector(stampede.DefaultBeta, chain.BuildChain(cfg.Interceptors)),
		maxSize:   cfg.MaxMemoryBytes,
		stopCh:    make(chan struct{}),
		nShards:   nShards,
		shardMask: hash.BitmaskForPowerOfTwo(nShards),
	}
	if cfg.CleanupInterval > 0 {
		c.wg.Add(1)
		go c.janitor()
	}
	return c, nil
}

func (c *Cache) checkClosed(op string) error {
	return c.guard.CheckClosed(op)
}

// SetInterceptors replaces the observability interceptors for this cache.
func (c *Cache) SetInterceptors(interceptors ...observability.Interceptor) {
	c.explicitInterceptors = interceptors
	c.chain = chain.SetInterceptors(c.chain, interceptors)
}

// Ping checks whether the cache is alive. It returns an error if the cache is closed.
func (c *Cache) Ping(_ context.Context) error { return c.guard.CheckClosed("memory.ping") }

// Name returns the backend identifier for this cache ("memory").
func (c *Cache) Name() string { return "memory" }

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.guard.IsClosed() }

// Close stops the background janitor and releases resources.
// It is safe to call Close multiple times; subsequent calls are no-ops.
func (c *Cache) Close(_ context.Context) error {
	if c.guard.Close() {
		return nil
	}
	close(c.stopCh)
	c.wg.Wait()
	c.detector.Close()
	return nil
}

func (c *Cache) shardFor(key string) *shard {
	idx := hash.FNV1a32(key) & c.shardMask
	return c.shards[idx]
}
