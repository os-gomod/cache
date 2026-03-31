package builder

import (
	"context"
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/memory"
)

// MemoryBuilder builds a memory cache with a fluent, immutable API.
type MemoryBuilder struct {
	*GenericBuilder[config.Memory, *memory.Cache]
}

// NewMemoryBuilder creates a memory cache builder.
func NewMemoryBuilder(ctx context.Context) *MemoryBuilder {
	return &MemoryBuilder{
		GenericBuilder: newValidatedBuilder(
			ctx,
			config.DefaultMemory,
			memory.NewWithConfigContext,
			func(cfg *config.Memory) *config.Memory { return cfg.Clone() },
		),
	}
}

// WithConfig applies a full memory config.
func (b *MemoryBuilder) WithConfig(cfg *config.Memory) *MemoryBuilder {
	return b.wrapOption(memory.WithConfig(cfg))
}

// WithOption applies a memory.Option.
func (b *MemoryBuilder) WithOption(opt memory.Option) *MemoryBuilder {
	return b.wrapOption(opt)
}

func (b *MemoryBuilder) wrapOption(opt memory.Option) *MemoryBuilder {
	return &MemoryBuilder{
		GenericBuilder: b.GenericBuilder.WithOption(func(cfg *config.Memory) { opt(cfg) }),
	}
}

func (b *MemoryBuilder) MaxEntries(n int) *MemoryBuilder {
	return b.wrapOption(memory.WithMaxEntries(n))
}

func (b *MemoryBuilder) MaxMemoryMB(mb int) *MemoryBuilder {
	return b.wrapOption(memory.WithMaxMemoryMB(mb))
}

func (b *MemoryBuilder) MaxMemoryBytes(bytes int64) *MemoryBuilder {
	return b.wrapOption(memory.WithMaxMemoryBytes(bytes))
}

func (b *MemoryBuilder) TTL(ttl time.Duration) *MemoryBuilder {
	return b.wrapOption(memory.WithTTL(ttl))
}

func (b *MemoryBuilder) CleanupInterval(d time.Duration) *MemoryBuilder {
	return b.wrapOption(memory.WithCleanupInterval(d))
}

func (b *MemoryBuilder) EvictionPolicy(p config.EvictionPolicy) *MemoryBuilder {
	return b.wrapOption(memory.WithEvictionPolicy(p))
}

func (b *MemoryBuilder) OnEviction(fn func(key, reason string)) *MemoryBuilder {
	return b.wrapOption(memory.WithOnEvictionPolicy(fn))
}

func (b *MemoryBuilder) ShardCount(n int) *MemoryBuilder {
	return b.wrapOption(memory.WithShards(n))
}

func (b *MemoryBuilder) EnableMetrics(enabled bool) *MemoryBuilder {
	return b.wrapOption(memory.WithEnableMetrics(enabled))
}

// Eviction-policy shortcuts.
func (b *MemoryBuilder) LRU() *MemoryBuilder     { return b.EvictionPolicy(config.EvictLRU) }
func (b *MemoryBuilder) LFU() *MemoryBuilder     { return b.EvictionPolicy(config.EvictLFU) }
func (b *MemoryBuilder) FIFO() *MemoryBuilder    { return b.EvictionPolicy(config.EvictFIFO) }
func (b *MemoryBuilder) LIFO() *MemoryBuilder    { return b.EvictionPolicy(config.EvictLIFO) }
func (b *MemoryBuilder) MRU() *MemoryBuilder     { return b.EvictionPolicy(config.EvictMRU) }
func (b *MemoryBuilder) Random() *MemoryBuilder  { return b.EvictionPolicy(config.EvictRR) }
func (b *MemoryBuilder) TinyLFU() *MemoryBuilder { return b.EvictionPolicy(config.EvictTinyLFU) }

// MustBuild calls Build and panics on error.
func (b *MemoryBuilder) MustBuild() *memory.Cache { return b.GenericBuilder.MustBuild() }
