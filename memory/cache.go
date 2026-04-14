// Package memory provides an in-memory cache implementation with support for advanced operations like GetMulti,
// SetMulti, DeleteMulti, GetOrSet with singleflight, CompareAndSwap, Increment/Decrement, and various eviction
// policies. It also includes observability hooks for monitoring cache performance and usage patterns.
package memory

import (
	"context"
	"sync"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/stampede"
)

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

type shard struct {
	mu    sync.RWMutex
	items map[string]*eviction.Entry
	evict eviction.Evictor
	size  int64
	count int64
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

func NewWithConfig(cfg *config.Memory) (*Cache, error) {
	return NewWithConfigContext(context.Background(), cfg)
}

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

	// Build the observability chain from config — interceptors only.
	var interceptors []observability.Interceptor
	if ic, ok := cfg.Interceptors.([]observability.Interceptor); ok {
		interceptors = append(interceptors, ic...)
	}

	var chain *observability.Chain
	if len(interceptors) > 0 {
		chain = observability.NewChain(interceptors...)
	} else {
		chain = observability.NopChain()
	}

	c := &Cache{
		cfg:       *cfg,
		guard:     lifecycle.Guard{},
		shards:    shards,
		stats:     stats.NewStats(),
		chain:     chain,
		sg:        singlefght.NewGroup(),
		detector:  stampede.NewDetector(stampede.DefaultBeta, chain),
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

func (c *Cache) Ping(_ context.Context) error { return c.guard.CheckClosed("memory.ping") }
func (c *Cache) Name() string                 { return "memory" }
func (c *Cache) Stats() stats.Snapshot        { return c.stats.TakeSnapshot() }
func (c *Cache) Closed() bool                 { return c.guard.IsClosed() }
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
	idx := fnv1a32(key) & c.shardMask
	return c.shards[idx]
}

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
