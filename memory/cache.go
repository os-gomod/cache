// Package memory provides a high-performance, sharded in-process cache.
package memory

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/os-gomod/cache/config"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/eviction"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
)

// Cache is a sharded in-process cache.
//
// Concurrency model:
//   - Each shard owns a sync.RWMutex that guards its items map, eviction
//     bookkeeping, and the per-shard size/count fields.
//   - The janitor goroutine sweeps expired entries and exits when stopCh
//     is closed by Close().
//   - closed is an atomic bool so checkClosed never acquires a lock.
type Cache struct {
	cfg    config.Memory
	stats  *stats.Stats
	shards []*shard
	sg     *singlefght.Group
	wg     sync.WaitGroup
	closed atomic.Bool
	stopCh chan struct{}

	// maxSize is the total memory budget in bytes derived from cfg.MaxMemoryBytes.
	maxSize int64
	// nShards caches len(shards) to avoid repeated slice-header accesses.
	nShards int
	// shardMask selects a shard because nShards is always a power of two.
	shardMask uint32
}

// shard is one independently-locked segment of the cache.
type shard struct {
	mu    sync.RWMutex
	items map[string]*eviction.Entry
	evict eviction.Evictor
	size  int64
	count int64
}

// ----------------------------------------------------------------------------
// Constructors
// ----------------------------------------------------------------------------

// New creates a memory cache with functional options.
func New(opts ...Option) (*Cache, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext creates a memory cache with functional options and a context.
func NewWithContext(ctx context.Context, opts ...Option) (*Cache, error) {
	cfg, err := MergeOptions(opts...)
	if err != nil {
		return nil, err
	}
	return NewWithConfigContext(ctx, cfg)
}

// NewWithConfig creates a memory cache from an explicit config struct.
func NewWithConfig(cfg *config.Memory) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

// NewWithConfigContext is the canonical constructor; all other New* functions
// delegate here.  It clones the config, applies defaults, validates, and
// constructs the cache.
func NewWithConfigContext(ctx context.Context, cfg *config.Memory) (*Cache, error) {
	if cfg == nil {
		cfg = config.DefaultMemory()
	}
	cfg = cfg.Clone()
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return newFromConfig(cachectx.NormalizeContext(ctx), cfg)
}

func newFromConfig(_ context.Context, cfg *config.Memory) (*Cache, error) {
	nShards := normalizeShards(cfg.ShardCount)
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
		shards:    shards,
		stats:     stats.NewStats(),
		sg:        singlefght.NewGroup(),
		maxSize:   cfg.MaxMemoryBytes,
		stopCh:    make(chan struct{}),
		nShards:   nShards,
		shardMask: bitmaskForPowerOfTwo(nShards),
	}

	if cfg.CleanupInterval > 0 {
		c.wg.Add(1)
		go c.janitor()
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

// Stats returns a point-in-time snapshot of cache statistics.
func (c *Cache) Stats() stats.Snapshot { return c.stats.TakeSnapshot() }

// Closed reports whether the cache has been closed.
func (c *Cache) Closed() bool { return c.closed.Load() }

// Close shuts down the cache and releases all resources.
func (c *Cache) Close(_ context.Context) error {
	if c.closed.Swap(true) {
		return nil // already closed
	}
	close(c.stopCh)
	c.wg.Wait()
	return nil
}

// ----------------------------------------------------------------------------
// Shard routing (O(1), branch-free bitmask)
// ----------------------------------------------------------------------------

// shardFor returns the shard for key using a bitmask — nShards is always a
// power of two so (nShards-1) is valid mask.
func (c *Cache) shardFor(key string) *shard {
	idx := fnv1a32(key) & c.shardMask
	return c.shards[idx]
}

// fnv1a32 returns the FNV-1a 32-bit hash of s.
// Inlined to avoid the overhead of hash.Hash32 allocation on the hot path.
func fnv1a32(s string) uint32 {
	const (
		offset32 uint32 = 2166136261
		prime32  uint32 = 16777619
	)
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

// normalizeShards rounds n up to the nearest power of two, clamped to [1, 4096].
func normalizeShards(n int) int {
	const maxShards = 4096
	if n <= 0 {
		return 64
	}
	if n > maxShards {
		return maxShards
	}
	if n&(n-1) == 0 {
		return n
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

func bitmaskForPowerOfTwo(n int) uint32 {
	var mask uint32
	for bit := 1; bit < n; bit <<= 1 {
		mask = (mask << 1) | 1
	}
	return mask
}
