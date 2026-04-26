package config

import (
	"fmt"
	"time"
)

// Memory holds configuration for the in-memory cache backend. It supports
// size-based and count-based eviction, configurable sharding for reduced
// lock contention, and automatic background cleanup of expired entries.
type Memory struct {
	// MaxEntries is the maximum number of entries the cache can hold.
	// When this limit is reached, entries are evicted according to the
	// configured EvictionPolicy. A value of 0 means unlimited.
	MaxEntries int `validate:"gte=0" default:"10000"`

	// MaxMemoryBytes is the maximum total memory (in bytes) the cache
	// may consume. When this limit is reached, entries are evicted.
	// A value of 0 means unlimited memory.
	MaxMemoryBytes int64 `validate:"gte=0" default:"104857600"`

	// ShardCount controls the number of internal lock shards. Higher
	// values reduce lock contention for high-concurrency workloads.
	// Must be >= 1.
	ShardCount int `validate:"gte=1" default:"32"`

	// DefaultTTL is the time-to-live applied to entries that don't
	// specify an explicit TTL. A zero value means entries never expire.
	DefaultTTL time.Duration `validate:"gte=0" default:"5m"`

	// CleanupInterval controls how often the background goroutine
	// scans for and removes expired entries. A zero value disables
	// background cleanup.
	CleanupInterval time.Duration `validate:"gte=0" default:"1m"`

	// EvictionPolicy determines which entries are evicted when the
	// cache is full. Supported values: lru, lfu, fifo, lifo, mru,
	// random, tinylfu.
	EvictionPolicy string `validate:"omitempty,oneof=lru lfu fifo lifo mru random tinylfu" default:"lru"`
}

// SetDefaults populates zero-value fields with sensible production defaults.
func (c *Memory) SetDefaults() {
	setDefaultInt(&c.MaxEntries, 10000)
	setDefaultInt64(&c.MaxMemoryBytes, 104857600) // 100 MiB
	setDefaultInt(&c.ShardCount, 32)
	if c.DefaultTTL == 0 {
		c.DefaultTTL = 5 * time.Minute
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = 1 * time.Minute
	}
	setDefaultString(&c.EvictionPolicy, "lru")
}

// Validate checks all configuration constraints and returns a descriptive
// error if any constraint is violated.
func (c *Memory) Validate() error {
	base := NewBase()
	return base.ValidateStruct(c)
}

// Clone returns a deep copy of this configuration. The returned config
// is safe to modify without affecting the original.
func (c *Memory) Clone() *Memory {
	return &Memory{
		MaxEntries:      c.MaxEntries,
		MaxMemoryBytes:  c.MaxMemoryBytes,
		ShardCount:      c.ShardCount,
		DefaultTTL:      c.DefaultTTL,
		CleanupInterval: c.CleanupInterval,
		EvictionPolicy:  c.EvictionPolicy,
	}
}

// DefaultMemory returns a Memory configuration with all defaults applied.
func DefaultMemory() *Memory {
	c := &Memory{}
	c.SetDefaults()
	return c
}

// String returns a human-readable summary of the configuration.
func (c *Memory) String() string {
	return fmt.Sprintf(
		"Memory{MaxEntries:%d, MaxMemoryBytes:%d, ShardCount:%d, DefaultTTL:%s, CleanupInterval:%s, EvictionPolicy:%q}",
		c.MaxEntries,
		c.MaxMemoryBytes,
		c.ShardCount,
		c.DefaultTTL,
		c.CleanupInterval,
		c.EvictionPolicy,
	)
}
