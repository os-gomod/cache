package builder

import (
	"context"
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/layered"
)

// LayeredBuilder builds a layered (L1 memory + L2 Redis) cache with a fluent,
// immutable API.
type LayeredBuilder struct {
	*GenericBuilder[config.Layered, *layered.Cache]
}

// NewLayeredBuilder creates a layered cache builder.
func NewLayeredBuilder(ctx context.Context) *LayeredBuilder {
	return &LayeredBuilder{
		GenericBuilder: newValidatedBuilder(
			ctx,
			config.DefaultLayered,
			layered.NewWithConfigContext,
			func(cfg *config.Layered) *config.Layered { return cfg.Clone() },
		),
	}
}

// WithConfig applies a full layered config.
func (b *LayeredBuilder) WithConfig(cfg *config.Layered) *LayeredBuilder {
	return b.wrapOption(layered.WithConfig(cfg))
}

// WithOption applies a layered.Option.
func (b *LayeredBuilder) WithOption(opt layered.Option) *LayeredBuilder {
	return b.wrapOption(opt)
}

func (b *LayeredBuilder) wrapOption(opt layered.Option) *LayeredBuilder {
	return &LayeredBuilder{
		GenericBuilder: b.GenericBuilder.WithOption(func(cfg *config.Layered) { opt(cfg) }),
	}
}

// --- L1 (memory) options ---

func (b *LayeredBuilder) WithL1Config(cfg *config.Memory) *LayeredBuilder {
	return b.wrapOption(layered.WithL1Config(cfg))
}

func (b *LayeredBuilder) L1MaxEntries(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithL1MaxEntries(n))
}

func (b *LayeredBuilder) L1MaxMemoryMB(mb int) *LayeredBuilder {
	return b.wrapOption(layered.WithL1MaxMemoryMB(mb))
}

func (b *LayeredBuilder) L1MaxMemoryBytes(bytes int64) *LayeredBuilder {
	return b.wrapOption(layered.WithL1MaxMemoryBytes(bytes))
}

func (b *LayeredBuilder) L1TTL(ttl time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL1TTL(ttl))
}

func (b *LayeredBuilder) L1CleanupInterval(d time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL1CleanupInterval(d))
}

func (b *LayeredBuilder) L1Shards(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithL1Shards(n))
}

func (b *LayeredBuilder) L1EvictionPolicy(p config.EvictionPolicy) *LayeredBuilder {
	return b.wrapOption(layered.WithL1EvictionPolicy(p))
}

func (b *LayeredBuilder) L1OnEviction(fn func(key, reason string)) *LayeredBuilder {
	return b.wrapOption(layered.WithL1OnEvictionPolicy(fn))
}

func (b *LayeredBuilder) L1EnableMetrics(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithL1EnableMetrics(enabled))
}

// L1 eviction-policy shortcuts.
func (b *LayeredBuilder) L1LRU() *LayeredBuilder    { return b.L1EvictionPolicy(config.EvictLRU) }
func (b *LayeredBuilder) L1LFU() *LayeredBuilder    { return b.L1EvictionPolicy(config.EvictLFU) }
func (b *LayeredBuilder) L1FIFO() *LayeredBuilder   { return b.L1EvictionPolicy(config.EvictFIFO) }
func (b *LayeredBuilder) L1LIFO() *LayeredBuilder   { return b.L1EvictionPolicy(config.EvictLIFO) }
func (b *LayeredBuilder) L1MRU() *LayeredBuilder    { return b.L1EvictionPolicy(config.EvictMRU) }
func (b *LayeredBuilder) L1Random() *LayeredBuilder { return b.L1EvictionPolicy(config.EvictRR) }

func (b *LayeredBuilder) L1TinyLFU() *LayeredBuilder { return b.L1EvictionPolicy(config.EvictTinyLFU) }

// --- L2 (Redis) options ---

func (b *LayeredBuilder) WithL2Config(cfg *config.Redis) *LayeredBuilder {
	return b.wrapOption(layered.WithL2Config(cfg))
}

func (b *LayeredBuilder) L2Addr(addr string) *LayeredBuilder {
	return b.wrapOption(layered.WithL2Address(addr))
}

func (b *LayeredBuilder) L2Username(u string) *LayeredBuilder {
	return b.wrapOption(layered.WithL2Username(u))
}

func (b *LayeredBuilder) L2Password(p string) *LayeredBuilder {
	return b.wrapOption(layered.WithL2Password(p))
}
func (b *LayeredBuilder) L2DB(db int) *LayeredBuilder { return b.wrapOption(layered.WithL2DB(db)) }
func (b *LayeredBuilder) L2PoolSize(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithL2PoolSize(n))
}

func (b *LayeredBuilder) L2MinIdleConns(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithL2MinIdleConns(n))
}

func (b *LayeredBuilder) L2MaxRetries(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithL2MaxRetries(n))
}

func (b *LayeredBuilder) L2RetryBackoff(d time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2RetryBackoff(d))
}

func (b *LayeredBuilder) L2DialTimeout(d time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2DialTimeout(d))
}

func (b *LayeredBuilder) L2ReadTimeout(d time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2ReadTimeout(d))
}

func (b *LayeredBuilder) L2WriteTimeout(d time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2WriteTimeout(d))
}

func (b *LayeredBuilder) L2Timeouts(dial, read, write time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2Timeouts(dial, read, write))
}

func (b *LayeredBuilder) L2TTL(ttl time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithL2TTL(ttl))
}

func (b *LayeredBuilder) L2KeyPrefix(prefix string) *LayeredBuilder {
	return b.wrapOption(layered.WithL2KeyPrefix(prefix))
}

func (b *LayeredBuilder) L2EnablePipeline(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithL2EnablePipeline(enabled))
}

func (b *LayeredBuilder) L2EnableMetrics(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithL2EnableMetrics(enabled))
}

// --- layered-level options ---

func (b *LayeredBuilder) PromoteOnHit(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithPromoteOnHit(enabled))
}

func (b *LayeredBuilder) WriteBack(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithWriteBack(enabled))
}

func (b *LayeredBuilder) NegativeTTL(ttl time.Duration) *LayeredBuilder {
	return b.wrapOption(layered.WithNegativeTTL(ttl))
}

func (b *LayeredBuilder) SyncEnabled(enabled bool) *LayeredBuilder {
	return b.wrapOption(layered.WithSyncEnabled(enabled))
}

func (b *LayeredBuilder) SyncChannel(ch string) *LayeredBuilder {
	return b.wrapOption(layered.WithSyncChannel(ch))
}

func (b *LayeredBuilder) SyncBufferSize(n int) *LayeredBuilder {
	return b.wrapOption(layered.WithSyncBufferSize(n))
}

// MustBuild calls Build and panics on error.
func (b *LayeredBuilder) MustBuild() *layered.Cache { return b.GenericBuilder.MustBuild() }
