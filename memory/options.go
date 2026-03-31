package memory

import (
	"time"

	"github.com/os-gomod/cache/config"
)

// Option configures a memory cache.
type Option func(*config.Memory)

// WithMaxEntries sets the maximum number of live entries.
func WithMaxEntries(count int) Option {
	return func(c *config.Memory) { c.MaxEntries = count }
}

// WithMaxMemoryMB sets the memory cap in MiB.
func WithMaxMemoryMB(mb int) Option {
	return func(c *config.Memory) {
		c.MaxMemoryMB = mb
		c.MaxMemoryBytes = int64(mb) * 1024 * 1024
	}
}

// WithMaxMemoryBytes sets the memory cap in bytes.
func WithMaxMemoryBytes(bytes int64) Option {
	const bytesPerMB = int64(1024 * 1024)

	return func(c *config.Memory) {
		c.MaxMemoryBytes = bytes
		if bytes <= 0 {
			c.MaxMemoryMB = 0
			return
		}
		c.MaxMemoryMB = int((bytes + bytesPerMB - 1) / bytesPerMB)
	}
}

// WithTTL sets the default TTL applied when callers pass 0.
func WithTTL(ttl time.Duration) Option {
	return func(c *config.Memory) { c.DefaultTTL = ttl }
}

// WithCleanupInterval sets the janitor sweep interval.
// Pass 0 to disable background cleanup.
func WithCleanupInterval(d time.Duration) Option {
	return func(c *config.Memory) { c.CleanupInterval = d }
}

// WithEvictionPolicy sets the eviction algorithm.
func WithEvictionPolicy(p config.EvictionPolicy) Option {
	return func(c *config.Memory) { c.EvictionPolicy = p }
}

// WithOnEvictionPolicy registers a callback invoked on every eviction.
func WithOnEvictionPolicy(fn func(key, reason string)) Option {
	return func(c *config.Memory) { c.OnEvictionPolicy = fn }
}

// WithShards sets the shard count (rounded up to the next power of two).
func WithShards(n int) Option {
	return func(c *config.Memory) { c.ShardCount = n }
}

// WithEnableMetrics enables per-operation statistics collection.
func WithEnableMetrics(enabled bool) Option {
	return func(c *config.Memory) { c.EnableMetrics = enabled }
}

// WithConfig replaces the entire configuration.  The config is cloned so the
// caller may freely modify their copy after this call.
func WithConfig(cfg *config.Memory) Option {
	if cfg == nil {
		return func(*config.Memory) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Memory) { *c = *cloned }
}

// ----------------------------------------------------------------------------
// MergeOptions applies opts onto a default config, then validates.
// ----------------------------------------------------------------------------

// MergeOptions builds a *config.Memory by applying opts onto DefaultMemory().
func MergeOptions(opts ...Option) (*config.Memory, error) {
	cfg := config.DefaultMemory()
	for _, opt := range opts {
		opt(cfg)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// --- convenience aliases ---

// WithLRU selects the LRU eviction policy.
func WithLRU() Option { return WithEvictionPolicy(config.EvictLRU) }

// WithLFU selects the LFU eviction policy.
func WithLFU() Option { return WithEvictionPolicy(config.EvictLFU) }

// WithFIFO selects the FIFO eviction policy.
func WithFIFO() Option { return WithEvictionPolicy(config.EvictFIFO) }

// WithLIFO selects the LIFO eviction policy.
func WithLIFO() Option { return WithEvictionPolicy(config.EvictLIFO) }

// WithMRU selects the MRU eviction policy.
func WithMRU() Option { return WithEvictionPolicy(config.EvictMRU) }

// WithRandom selects the random eviction policy.
func WithRandom() Option { return WithEvictionPolicy(config.EvictRR) }

// WithTinyLFU selects the W-TinyLFU eviction policy.
func WithTinyLFU() Option { return WithEvictionPolicy(config.EvictTinyLFU) }
