package cache

import (
	"time"
)

// Option is a functional option for configuring cache backends.
// Options are applied in order, with later options overriding
// earlier ones for conflicting settings.
type Option func(*Config)

// Config holds the common configuration for cache backends. Individual
// backend implementations may extend this with additional options via
// their own Option types (e.g., memory.Option, redis.Option).
type Config struct {
	// DefaultTTL is the default time-to-live for cache entries when
	// no explicit TTL is provided. A zero value means entries do not
	// expire by default.
	DefaultTTL time.Duration

	// MaxEntries is the maximum number of entries the cache can hold.
	// When the limit is reached, entries are evicted according to the
	// configured eviction policy. A zero value means unlimited.
	MaxEntries int

	// EvictionPolicy specifies the algorithm used to evict entries
	// when MaxEntries is reached. Supported values: "lru", "lfu",
	// "fifo", "tinylfu". Defaults to "lru".
	EvictionPolicy string

	// Shards controls the number of internal shards used for
	// concurrency. More shards reduce lock contention but increase
	// memory overhead. Defaults to 64 for memory backends.
	Shards int

	// KeyPrefix is prepended to all keys stored in the backend.
	// Useful for multi-tenant isolation.
	KeyPrefix string

	// StatsEnabled controls whether the backend collects and reports
	// statistics (hit rate, miss rate, evictions, etc.). Defaults to true.
	StatsEnabled bool

	// Logger is the structured logger used for backend operations.
	// If nil, a no-op logger is used.
	Logger interface{} //nolint:godox // Logger interface to be defined in a future iteration
}

// WithDefaultTTL sets the default time-to-live for cache entries.
func WithDefaultTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.DefaultTTL = ttl
	}
}

// WithMaxEntries sets the maximum number of entries the cache can hold.
// When this limit is reached, entries are evicted according to the
// configured eviction policy.
func WithMaxEntries(maxVal int) Option {
	return func(c *Config) {
		c.MaxEntries = maxVal
	}
}

// WithEvictionPolicy sets the eviction algorithm. Supported values:
// "lru" (Least Recently Used), "lfu" (Least Frequently Used),
// "fifo" (First In First Out), "tinylfu" (TinyLFU).
// Defaults to "lru" if not specified.
func WithEvictionPolicy(policy string) Option {
	return func(c *Config) {
		c.EvictionPolicy = policy
	}
}

// WithShards sets the number of internal shards for the memory backend.
// More shards reduce lock contention under high concurrency. Defaults
// to 64. This option is ignored by non-memory backends.
func WithShards(n int) Option {
	return func(c *Config) {
		c.Shards = n
	}
}

// WithKeyPrefix sets a prefix that is prepended to all keys stored in
// the backend. This is useful for multi-tenant isolation or logical
// grouping within a shared cache.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// WithStatsEnabled controls whether the backend collects and reports
// statistics. Disabling stats can improve performance slightly for
// latency-sensitive workloads.
func WithStatsEnabled(enabled bool) Option {
	return func(c *Config) {
		c.StatsEnabled = enabled
	}
}

// HotKeyOption is a functional option for configuring a HotKeyDetector.
type HotKeyOption func(*HotKeyConfig)

// HotKeyConfig holds configuration for hot-key detection.
type HotKeyConfig struct {
	// Threshold is the number of accesses per window required for a
	// key to be considered "hot". Defaults to 100.
	Threshold int64

	// WindowDuration is the time window over which accesses are counted.
	// Defaults to 1 second.
	WindowDuration time.Duration

	// MaxKeys is the maximum number of keys tracked for hot-key
	// detection. Defaults to 10000.
	MaxKeys int

	// OnHotKey is called when a key exceeds the threshold. The callback
	// receives the key and the access count.
	OnHotKey func(key string, count int64)
}

// WithHotKeyThreshold sets the access count threshold for hot-key detection.
func WithHotKeyThreshold(threshold int64) HotKeyOption {
	return func(c *HotKeyConfig) {
		c.Threshold = threshold
	}
}

// WithHotKeyWindow sets the time window for hot-key detection.
func WithHotKeyWindow(d time.Duration) HotKeyOption {
	return func(c *HotKeyConfig) {
		c.WindowDuration = d
	}
}

// WithHotKeyMaxKeys sets the maximum number of keys tracked.
func WithHotKeyMaxKeys(maxVal int) HotKeyOption {
	return func(c *HotKeyConfig) {
		c.MaxKeys = maxVal
	}
}

// WithHotKeyCallback sets the callback invoked when a key becomes hot.
func WithHotKeyCallback(fn func(key string, count int64)) HotKeyOption {
	return func(c *HotKeyConfig) {
		c.OnHotKey = fn
	}
}

func defaultHotKeyConfig() *HotKeyConfig {
	return &HotKeyConfig{
		Threshold:      100,
		WindowDuration: time.Second,
		MaxKeys:        10000,
		OnHotKey:       nil,
	}
}

// WarmerOption is a functional option for configuring a Warmer.
type WarmerOption func(*WarmerConfig)

// WarmerConfig holds configuration for cache warming.
type WarmerConfig struct {
	// Concurrency controls how many keys are warmed in parallel.
	// Defaults to 10.
	Concurrency int

	// BatchSize controls how many keys are loaded per batch call.
	// Defaults to 100.
	BatchSize int

	// OnError is called when warming a key fails. If nil, errors
	// are silently ignored.
	OnError func(key string, err error)
}

// WithWarmerConcurrency sets the parallelism for cache warming.
func WithWarmerConcurrency(n int) WarmerOption {
	return func(c *WarmerConfig) {
		c.Concurrency = n
	}
}

// WithWarmerBatchSize sets the batch size for the loader function.
func WithWarmerBatchSize(n int) WarmerOption {
	return func(c *WarmerConfig) {
		c.BatchSize = n
	}
}

// WithWarmerErrorHandler sets the error handler for failed warm operations.
func WithWarmerErrorHandler(fn func(key string, err error)) WarmerOption {
	return func(c *WarmerConfig) {
		c.OnError = fn
	}
}

func defaultWarmerConfig() *WarmerConfig {
	return &WarmerConfig{
		Concurrency: 10,
		BatchSize:   100,
		OnError:     nil,
	}
}
