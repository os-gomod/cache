// Package memory implements a high-performance, multi-shard, in-memory cache
// backend with configurable eviction, TTL-based expiration, and background
// cleanup. It uses the unified runtime.Executor for all operations and
// lifecycle.Guard for close state management.
package memory

import (
	"time"

	"github.com/os-gomod/cache/v2/internal/middleware"
)

// Option is a functional option for configuring the in-memory Store.
type Option func(*config)

// WithMaxEntries sets the maximum number of entries allowed in the cache.
// Default: 10000.
func WithMaxEntries(n int) Option {
	return func(c *config) { c.maxEntries = n }
}

// WithMaxMemoryBytes sets the maximum total memory (in bytes) that cached
// values may consume. Default: 100MB (104857600).
func WithMaxMemoryBytes(n int64) Option {
	return func(c *config) { c.maxMemoryBytes = n }
}

// WithShardCount sets the number of concurrent shards. Must be a power of two;
// will be rounded up automatically via hash.NormalizeShards. Default: 32.
func WithShardCount(n int) Option {
	return func(c *config) { c.shardCount = n }
}

// WithDefaultTTL sets the default time-to-live for entries that don't specify
// a TTL. A zero value means entries never expire. Default: 5 minutes.
func WithDefaultTTL(d time.Duration) Option {
	return func(c *config) { c.defaultTTL = d }
}

// WithCleanupInterval sets the interval between expired entry cleanup sweeps.
// A zero value disables the background janitor. Default: 1 minute.
func WithCleanupInterval(d time.Duration) Option {
	return func(c *config) { c.cleanupInterval = d }
}

// WithInterceptors sets the observability interceptors for the memory store.
func WithInterceptors(i ...middleware.Interceptor) Option {
	return func(c *config) { c.interceptors = i }
}

// config holds the validated configuration for the memory Store.
type config struct {
	maxEntries      int
	maxMemoryBytes  int64
	shardCount      int
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	interceptors    []middleware.Interceptor
}

// defaultConfig returns the default configuration for the memory Store.
func defaultConfig() config {
	return config{
		maxEntries:      10000,
		maxMemoryBytes:  100 * 1024 * 1024, // 100MB
		shardCount:      32,
		defaultTTL:      5 * time.Minute,
		cleanupInterval: 1 * time.Minute,
		interceptors:    nil,
	}
}

// apply applies all options to the default config and validates the result.
func (c *config) apply(opts ...Option) error {
	*c = defaultConfig()
	for _, opt := range opts {
		opt(c)
	}
	return c.validate()
}

// validate checks the configuration values for correctness.
func (c *config) validate() error {
	if c.maxEntries <= 0 {
		c.maxEntries = 10000
	}
	if c.maxMemoryBytes <= 0 {
		c.maxMemoryBytes = 100 * 1024 * 1024
	}
	if c.cleanupInterval < 0 {
		c.cleanupInterval = 0
	}
	return nil
}
