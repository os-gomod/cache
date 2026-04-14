package memory

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

type Option func(*config.Memory)

func WithMaxEntries(count int) Option {
	return func(c *config.Memory) { c.MaxEntries = count }
}

func WithMaxMemoryMB(mb int) Option {
	return func(c *config.Memory) {
		c.MaxMemoryMB = mb
		c.MaxMemoryBytes = int64(mb) * 1024 * 1024
	}
}

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

func WithTTL(ttl time.Duration) Option {
	return func(c *config.Memory) { c.DefaultTTL = ttl }
}

func WithCleanupInterval(d time.Duration) Option {
	return func(c *config.Memory) { c.CleanupInterval = d }
}

func WithEvictionPolicy(p config.EvictionPolicy) Option {
	return func(c *config.Memory) { c.EvictionPolicy = p }
}

func WithOnEvictionPolicy(fn func(key, reason string)) Option {
	return func(c *config.Memory) { c.OnEvictionPolicy = fn }
}

func WithShards(n int) Option {
	return func(c *config.Memory) { c.ShardCount = n }
}

func WithEnableMetrics(enabled bool) Option {
	return func(c *config.Memory) { c.EnableMetrics = enabled }
}

func WithConfig(cfg *config.Memory) Option {
	if cfg == nil {
		return func(*config.Memory) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Memory) { *c = *cloned }
}

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

func WithLRU() Option     { return WithEvictionPolicy(config.EvictLRU) }
func WithLFU() Option     { return WithEvictionPolicy(config.EvictLFU) }
func WithFIFO() Option    { return WithEvictionPolicy(config.EvictFIFO) }
func WithLIFO() Option    { return WithEvictionPolicy(config.EvictLIFO) }
func WithMRU() Option     { return WithEvictionPolicy(config.EvictMRU) }
func WithRandom() Option  { return WithEvictionPolicy(config.EvictRR) }
func WithTinyLFU() Option { return WithEvictionPolicy(config.EvictTinyLFU) }

// WithInterceptors configures the cache to use the given interceptors in
// the observability chain. Interceptors are called in order on Before and
// in reverse order on After.
func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Memory) { c.Interceptors = interceptors }
}
