package layer

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

// Option is a functional option for configuring a layered cache.
type Option func(*config.Layered)

// WithL1Config sets the L1 (in-memory) cache configuration.
func WithL1Config(cfg *config.Memory) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { c.L1Config = cloned }
}

// WithL1MaxEntries sets the maximum number of entries in the L1 cache.
func WithL1MaxEntries(n int) Option {
	return func(c *config.Layered) { l1(c).MaxEntries = n }
}

// WithL1MaxMemoryMB sets the maximum memory usage in megabytes for the L1 cache.
func WithL1MaxMemoryMB(mb int) Option {
	return func(c *config.Layered) {
		l1(c).MaxMemoryMB = mb
		l1(c).MaxMemoryBytes = int64(mb) * 1024 * 1024
	}
}

// WithL1MaxMemoryBytes sets the maximum memory usage in bytes for the L1 cache.
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

// WithL1TTL sets the default time-to-live for L1 cache entries.
func WithL1TTL(ttl time.Duration) Option {
	return func(c *config.Layered) { l1(c).DefaultTTL = ttl }
}

// WithL1CleanupInterval sets the interval between L1 expired entry cleanup sweeps.
func WithL1CleanupInterval(d time.Duration) Option {
	return func(c *config.Layered) { l1(c).CleanupInterval = d }
}

// WithL1Shards sets the number of L1 concurrent shards.
func WithL1Shards(n int) Option {
	return func(c *config.Layered) { l1(c).ShardCount = n }
}

// WithL1EvictionPolicy sets the eviction algorithm for the L1 cache.
func WithL1EvictionPolicy(p config.EvictionPolicy) Option {
	return func(c *config.Layered) { l1(c).EvictionPolicy = p }
}

// WithL1LRU configures the L1 cache to use Least Recently Used eviction.
func WithL1LRU() Option {
	return WithL1EvictionPolicy(config.EvictLRU)
}

// WithL1LFU configures the L1 cache to use Least Frequently Used eviction.
func WithL1LFU() Option {
	return WithL1EvictionPolicy(config.EvictLFU)
}

// WithL1FIFO configures the L1 cache to use First In First Out eviction.
func WithL1FIFO() Option {
	return WithL1EvictionPolicy(config.EvictFIFO)
}

// WithL1LIFO configures the L1 cache to use Last In First Out eviction.
func WithL1LIFO() Option {
	return WithL1EvictionPolicy(config.EvictLIFO)
}

// WithL1MRU configures the L1 cache to use Most Recently Used eviction.
func WithL1MRU() Option {
	return WithL1EvictionPolicy(config.EvictMRU)
}

// WithL1Random configures the L1 cache to use random replacement eviction.
func WithL1Random() Option {
	return WithL1EvictionPolicy(config.EvictRR)
}

// WithL1TinyLFU configures the L1 cache to use TinyLFU eviction.
func WithL1TinyLFU() Option {
	return WithL1EvictionPolicy(config.EvictTinyLFU)
}

// WithL1OnEvictionPolicy sets a callback invoked when L1 entries are evicted.
func WithL1OnEvictionPolicy(fn func(key, reason string)) Option {
	return func(c *config.Layered) { l1(c).OnEvictionPolicy = fn }
}

// WithL1EnableMetrics enables or disables L1 metrics collection.
func WithL1EnableMetrics(enabled bool) Option {
	return func(c *config.Layered) { l1(c).EnableMetrics = enabled }
}

func l1(c *config.Layered) *config.Memory {
	if c.L1Config == nil {
		c.L1Config = &config.Memory{}
	}
	return c.L1Config
}

// WithL2Config sets the L2 (Redis) cache configuration.
func WithL2Config(cfg *config.Redis) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { c.L2Config = cloned }
}

// WithL2Address sets the Redis server address for the L2 cache.
func WithL2Address(addr string) Option {
	return func(c *config.Layered) { l2(c).Addr = addr }
}

// WithL2Username sets the Redis ACL username for the L2 cache.
func WithL2Username(u string) Option {
	return func(c *config.Layered) { l2(c).Username = u }
}

// WithL2Password sets the Redis password for the L2 cache.
func WithL2Password(p string) Option {
	return func(c *config.Layered) { l2(c).Password = p }
}

// WithL2DB sets the Redis database index for the L2 cache.
func WithL2DB(db int) Option {
	return func(c *config.Layered) { l2(c).DB = db }
}

// WithL2PoolSize sets the maximum number of L2 Redis socket connections.
func WithL2PoolSize(n int) Option {
	return func(c *config.Layered) { l2(c).PoolSize = n }
}

// WithL2MinIdleConns sets the minimum number of idle L2 Redis connections.
func WithL2MinIdleConns(n int) Option {
	return func(c *config.Layered) { l2(c).MinIdleConns = n }
}

// WithL2MaxRetries sets the maximum number of L2 Redis command retries.
func WithL2MaxRetries(n int) Option {
	return func(c *config.Layered) { l2(c).MaxRetries = n }
}

// WithL2RetryBackoff sets the duration between L2 Redis retry attempts.
func WithL2RetryBackoff(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).RetryBackoff = d }
}

// WithL2DialTimeout sets the timeout for establishing new L2 Redis connections.
func WithL2DialTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).DialTimeout = d }
}

// WithL2ReadTimeout sets the timeout for reading from the L2 Redis server.
func WithL2ReadTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).ReadTimeout = d }
}

// WithL2WriteTimeout sets the timeout for writing to the L2 Redis server.
func WithL2WriteTimeout(d time.Duration) Option {
	return func(c *config.Layered) { l2(c).WriteTimeout = d }
}

// WithL2Timeouts sets dial, read, and write timeouts for the L2 Redis at once.
func WithL2Timeouts(dial, read, write time.Duration) Option {
	return func(c *config.Layered) {
		l2(c).DialTimeout = dial
		l2(c).ReadTimeout = read
		l2(c).WriteTimeout = write
	}
}

// WithL2TTL sets the default time-to-live for L2 Redis entries.
func WithL2TTL(ttl time.Duration) Option {
	return func(c *config.Layered) { l2(c).DefaultTTL = ttl }
}

// WithL2KeyPrefix sets a prefix prepended to all L2 Redis keys.
func WithL2KeyPrefix(prefix string) Option {
	return func(c *config.Layered) { l2(c).KeyPrefix = prefix }
}

// WithL2EnablePipeline enables or disables L2 Redis pipelining for batch operations.
func WithL2EnablePipeline(enabled bool) Option {
	return func(c *config.Layered) { l2(c).EnablePipeline = enabled }
}

// WithL2EnableMetrics enables or disables L2 metrics collection.
func WithL2EnableMetrics(enabled bool) Option {
	return func(c *config.Layered) { l2(c).EnableMetrics = enabled }
}

func l2(c *config.Layered) *config.Redis {
	if c.L2Config == nil {
		c.L2Config = &config.Redis{}
	}
	return c.L2Config
}

// WithPromoteOnHit enables or disables L1 promotion on L2 cache hits.
func WithPromoteOnHit(enabled bool) Option {
	return func(c *config.Layered) {
		c.PromoteOnHit = enabled
		c.PromoteOnHitSet = true
	}
}

// WithWriteBack enables or disables asynchronous L2 write-back.
func WithWriteBack(enabled bool) Option {
	return func(c *config.Layered) { c.WriteBack = enabled }
}

// WithNegativeTTL sets the time-to-live for negative cache entries (cache misses stored in L1).
func WithNegativeTTL(ttl time.Duration) Option {
	return func(c *config.Layered) { c.NegativeTTL = ttl }
}

// WithSyncEnabled enables or disables cross-instance cache invalidation via Redis Pub/Sub.
func WithSyncEnabled(enabled bool) Option {
	return func(c *config.Layered) { c.SyncEnabled = enabled }
}

// WithSyncChannel sets the Redis Pub/Sub channel for cross-instance invalidation.
func WithSyncChannel(ch string) Option {
	return func(c *config.Layered) { c.SyncChannel = ch }
}

// WithSyncBufferSize sets the buffer size for the sync channel subscriber.
func WithSyncBufferSize(n int) Option {
	return func(c *config.Layered) { c.SyncBufferSize = n }
}

// WithSync configures cross-instance sync with a channel name and buffer size.
func WithSync(channel string, bufferSize int) Option {
	return func(c *config.Layered) {
		c.SyncEnabled = true
		c.SyncChannel = channel
		c.SyncBufferSize = bufferSize
	}
}

// WithWriteBackConfig configures write-back mode with the given queue size and worker count.
func WithWriteBackConfig(queueSize, workers int) Option {
	return func(c *config.Layered) {
		c.WriteBack = true
		c.WriteBackQueueSize = queueSize
		c.WriteBackWorkers = workers
	}
}

// WithConfig applies a pre-built Layered configuration (cloned to prevent mutation).
func WithConfig(cfg *config.Layered) Option {
	if cfg == nil {
		return func(*config.Layered) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Layered) { *c = *cloned }
}

// MergeOptions applies all options to a default Layered config and validates it.
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

// WithInterceptors sets the observability interceptors for the layered cache.
func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Layered) { c.Interceptors = interceptors }
}
