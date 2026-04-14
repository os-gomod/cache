package layer

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

type Option func(*config.Layered)

func WithL1Config(cfg *config.Memory) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { c.L1Config = cloned }
}

func WithL1MaxEntries(n int) Option {
	return func(c *config.Layered) { l1(c).MaxEntries = n }
}

func WithL1MaxMemoryMB(mb int) Option {
	return func(c *config.Layered) {
		l1(c).MaxMemoryMB = mb
		l1(c).MaxMemoryBytes = int64(mb) * 1024 * 1024
	}
}

func WithL1MaxMemoryBytes(bytes int64) Option {
	const bytesPerMB = int64(1024 * 1024)
	return func(c *config.Layered) {
		l1Cfg := l1(c)
		l1Cfg.MaxMemoryBytes = bytes
		if bytes <= 0 {
			l1Cfg.MaxMemoryMB = 0
			return
		}
		l1Cfg.MaxMemoryMB = int((bytes + bytesPerMB - 1) / bytesPerMB)
	}
}

func WithL1TTL(ttl time.Duration) Option {
	return func(c *config.Layered) { l1(c).DefaultTTL = ttl }
}

func WithL1CleanupInterval(d time.Duration) Option {
	return func(c *config.Layered) { l1(c).CleanupInterval = d }
}

func WithL1Shards(n int) Option {
	return func(c *config.Layered) { l1(c).ShardCount = n }
}

func WithL1EvictionPolicy(p config.EvictionPolicy) Option {
	return func(c *config.Layered) { l1(c).EvictionPolicy = p }
}

func WithL1LRU() Option {
	return WithL1EvictionPolicy(config.EvictLRU)
}

func WithL1LFU() Option {
	return WithL1EvictionPolicy(config.EvictLFU)
}

func WithL1FIFO() Option {
	return WithL1EvictionPolicy(config.EvictFIFO)
}

func WithL1LIFO() Option {
	return WithL1EvictionPolicy(config.EvictLIFO)
}

func WithL1MRU() Option {
	return WithL1EvictionPolicy(config.EvictMRU)
}

func WithL1Random() Option {
	return WithL1EvictionPolicy(config.EvictRR)
}

func WithL1TinyLFU() Option {
	return WithL1EvictionPolicy(config.EvictTinyLFU)
}

func WithL1OnEvictionPolicy(fn func(key, reason string)) Option {
	return func(c *config.Layered) { l1(c).OnEvictionPolicy = fn }
}

func WithL1EnableMetrics(enabled bool) Option {
	return func(c *config.Layered) { l1(c).EnableMetrics = enabled }
}

func l1(c *config.Layered) *config.Memory {
	if c.L1Config == nil {
		c.L1Config = &config.Memory{}
	}
	return c.L1Config
}

func WithL2Config(cfg *config.Redis) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { c.L2Config = cloned }
}

func WithL2Address(addr string) Option {
	return func(c *config.Layered) { l2(c).Addr = addr }
}

func WithL2Username(u string) Option {
	return func(c *config.Layered) { l2(c).Username = u }
}

func WithL2Password(p string) Option {
	return func(c *config.Layered) { l2(c).Password = p }
}

func WithL2DB(db int) Option {
	return func(c *config.Layered) { l2(c).DB = db }
}

func WithL2PoolSize(n int) Option {
	return func(c *config.Layered) { l2(c).PoolSize = n }
}

func WithL2MinIdleConns(n int) Option {
	return func(c *config.Layered) { l2(c).MinIdleConns = n }
}

func WithL2MaxRetries(n int) Option {
	return func(c *config.Layered) { l2(c).MaxRetries = n }
}

func WithL2RetryBackoff(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).RetryBackoff = d }
}

func WithL2DialTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).DialTimeout = d }
}

func WithL2ReadTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).ReadTimeout = d }
}

func WithL2WriteTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).WriteTimeout = d }
}

func WithL2Timeouts(dial, read, write time.Duration) Option {
	return func(c *config.Layered) {
		l2(c).DialTimeout = dial
		l2(c).ReadTimeout = read
		l2(c).WriteTimeout = write
	}
}

func WithL2TTL(ttl time.Duration) Option {
	return func(c *config.Layered) { l2(c).DefaultTTL = ttl }
}

func WithL2KeyPrefix(prefix string) Option {
	return func(c *config.Layered) { l2(c).KeyPrefix = prefix }
}

func WithL2EnablePipeline(enabled bool) Option {
	return func(c *config.Layered) { l2(c).EnablePipeline = enabled }
}

func WithL2EnableMetrics(enabled bool) Option {
	return func(c *config.Layered) { l2(c).EnableMetrics = enabled }
}

func l2(c *config.Layered) *config.Redis {
	if c.L2Config == nil {
		c.L2Config = &config.Redis{}
	}
	return c.L2Config
}

func WithPromoteOnHit(enabled bool) Option {
	return func(c *config.Layered) {
		c.PromoteOnHit = enabled
		c.PromoteOnHitSet = true
	}
}

func WithWriteBack(enabled bool) Option {
	return func(c *config.Layered) { c.WriteBack = enabled }
}

func WithNegativeTTL(ttl time.Duration) Option {
	return func(c *config.Layered) { c.NegativeTTL = ttl }
}

func WithSyncEnabled(enabled bool) Option {
	return func(c *config.Layered) { c.SyncEnabled = enabled }
}

func WithSyncChannel(ch string) Option {
	return func(c *config.Layered) { c.SyncChannel = ch }
}

func WithSyncBufferSize(n int) Option {
	return func(c *config.Layered) { c.SyncBufferSize = n }
}

func WithSync(channel string, bufferSize int) Option {
	return func(c *config.Layered) {
		c.SyncEnabled = true
		c.SyncChannel = channel
		c.SyncBufferSize = bufferSize
	}
}

func WithWriteBackConfig(queueSize, workers int) Option {
	return func(c *config.Layered) {
		c.WriteBack = true
		c.WriteBackQueueSize = queueSize
		c.WriteBackWorkers = workers
	}
}

func WithConfig(cfg *config.Layered) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { *c = *cloned }
}

func MergeOptions(opts ...Option) (*config.Layered, error) {
	cfg := config.DefaultLayered()
	for _, opt := range opts {
		opt(cfg)
	}
	if err := config.Apply(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Layered) { c.Interceptors = interceptors }
}
