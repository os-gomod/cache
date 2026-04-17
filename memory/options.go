package memory

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

// Option is a functional option for configuring an in-memory cache.
type Option func(*config.Memory)

// WithMaxEntries sets the maximum number of entries in the cache.
func WithMaxEntries(count int) Option {
	return func(c *config.Memory) { c.MaxEntries = count }
}

// WithMaxMemoryMB sets the maximum memory usage in megabytes.
func WithMaxMemoryMB(mb int) Option {
	return func(c *config.Memory) {
		c.MaxMemoryMB = mb
		c.MaxMemoryBytes = int64(mb) * 1024 * 1024
	}
}

// WithMaxMemoryBytes sets the maximum memory usage in bytes.
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

// WithTTL sets the default time-to-live for cache entries.
func WithTTL(ttl time.Duration) Option {
	return func(c *config.Memory) { c.DefaultTTL = ttl }
}

// WithCleanupInterval sets the interval between expired entry cleanup sweeps.
func WithCleanupInterval(d time.Duration) Option {
	return func(c *config.Memory) { c.CleanupInterval = d }
}

// WithEvictionPolicy sets the eviction algorithm used when the cache is full.
func WithEvictionPolicy(p config.EvictionPolicy) Option {
	return func(c *config.Memory) { c.EvictionPolicy = p }
}

// WithOnEvictionPolicy sets a callback invoked when entries are evicted.
func WithOnEvictionPolicy(fn func(key, reason string)) Option {
	return func(c *config.Memory) { c.OnEvictionPolicy = fn }
}

// WithShards sets the number of concurrent shards (must be a power of two).
func WithShards(n int) Option {
	return func(c *config.Memory) { c.ShardCount = n }
}

// WithEnableMetrics enables or disables internal metrics collection.
func WithEnableMetrics(enabled bool) Option {
	return func(c *config.Memory) { c.EnableMetrics = enabled }
}

// WithConfig applies a pre-built Memory configuration (cloned to prevent mutation).
func WithConfig(cfg *config.Memory) Option {
	if cfg == nil {
		return func(*config.Memory) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Memory) { *c = *cloned }
}

// MergeOptions applies all options to a default Memory config and validates it.
func MergeOptions(opts ...Option) (*config.Memory, error) {
	cfg := config.DefaultMemory()
	for _, opt := range opts {
		opt(cfg)
	}
	if err := config.Apply(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// WithLRU configures the cache to use Least Recently Used eviction.
func WithLRU() Option { return WithEvictionPolicy(config.EvictLRU) }

// WithLFU configures the cache to use Least Frequently Used eviction.
func WithLFU() Option { return WithEvictionPolicy(config.EvictLFU) }

// WithFIFO configures the cache to use First In First Out eviction.
func WithFIFO() Option { return WithEvictionPolicy(config.EvictFIFO) }

// WithLIFO configures the cache to use Last In First Out eviction.
func WithLIFO() Option { return WithEvictionPolicy(config.EvictLIFO) }

// WithMRU configures the cache to use Most Recently Used eviction.
func WithMRU() Option { return WithEvictionPolicy(config.EvictMRU) }

// WithRandom configures the cache to use random replacement eviction.
func WithRandom() Option { return WithEvictionPolicy(config.EvictRR) }

// WithTinyLFU configures the cache to use TinyLFU eviction.
func WithTinyLFU() Option { return WithEvictionPolicy(config.EvictTinyLFU) }

// WithInterceptors sets the observability interceptors for the memory cache.
func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Memory) { c.Interceptors = interceptors }
}
