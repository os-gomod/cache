package config

import (
	"time"

	"github.com/os-gomod/cache/internal/hash"
)

// EvictionPolicy defines the algorithm used to select entries for eviction
// when the cache reaches capacity limits.
//
// Available policies:
//   - EvictLRU (Least Recently Used): Evicts entries not accessed recently.
//   - EvictLFU (Least Frequently Used): Evicts entries with lowest access count.
//   - EvictFIFO (First In First Out): Evicts oldest entries regardless of access.
//   - EvictLIFO (Last In First Out): Evicts newest entries first.
//   - EvictMRU (Most Recently Used): Evicts the most recently accessed.
//   - EvictRR (Random Replacement): Randomly selects victims.
//   - EvictTinyLFU: Approximate LFU with resetting mechanism.
//
// Default: EvictLRU.
type EvictionPolicy int

const (
	// EvictLRU evicts the least recently used entry.
	EvictLRU EvictionPolicy = iota
	// EvictLFU evicts the least frequently used entry.
	EvictLFU
	// EvictFIFO evicts the oldest entry (first in, first out).
	EvictFIFO
	// EvictLIFO evicts the newest entry (last in, first out).
	EvictLIFO
	// EvictMRU evicts the most recently used entry.
	EvictMRU
	// EvictRR evicts a random entry (random replacement).
	EvictRR
	// EvictTinyLFU uses a TinyLFU admission policy with LRU for evictions.
	EvictTinyLFU
)

func (p EvictionPolicy) String() string {
	switch p {
	case EvictLRU:
		return "lru"
	case EvictLFU:
		return "lfu"
	case EvictFIFO:
		return "fifo"
	case EvictLIFO:
		return "lifo"
	case EvictMRU:
		return "mru"
	case EvictRR:
		return "random"
	case EvictTinyLFU:
		return "tinylfu"
	default:
		return "unknown"
	}
}

// IsValid reports whether the eviction policy is a known value.
func (p EvictionPolicy) IsValid() bool { return p >= EvictLRU && p <= EvictTinyLFU }

// Memory holds the configuration for the in-memory cache backend.
type Memory struct {
	base             Base
	MaxEntries       int            `config:"max_entries" default:"10000" validate:"min=1"`
	MaxMemoryMB      int            `config:"max_memory_mb" default:"100" validate:"min=0"`
	MaxMemoryBytes   int64          `config:"max_memory_bytes" validate:"min=0"`
	DefaultTTL       time.Duration  `config:"default_ttl" default:"30m" validate:"gte=0"`
	CleanupInterval  time.Duration  `config:"cleanup_interval" default:"5m" validate:"gte=0"`
	ShardCount       int            `config:"shard_count" default:"32" validate:"min=1,power_of_two"`
	EvictionPolicy   EvictionPolicy `config:"eviction_policy" default:"lru"`
	OnEvictionPolicy func(key, reason string)
	EnableMetrics    bool `config:"enable_metrics" default:"false"`
	Interceptors     any
}

// SetDefaults fills in zero-valued fields with sensible defaults.
func (c *Memory) SetDefaults() {
	const bytesPerMB = int64(1024 * 1024)
	SetDefaultInt(&c.MaxEntries, 10000)
	if c.MaxMemoryMB == 0 && c.MaxMemoryBytes == 0 {
		c.MaxMemoryMB = 100
	}
	if c.MaxMemoryBytes == 0 && c.MaxMemoryMB > 0 {
		c.MaxMemoryBytes = int64(c.MaxMemoryMB) * bytesPerMB
	} else if c.MaxMemoryMB == 0 && c.MaxMemoryBytes > 0 {
		c.MaxMemoryMB = int((c.MaxMemoryBytes + bytesPerMB - 1) / bytesPerMB)
	}
	if c.MaxMemoryMB < 0 {
		c.MaxMemoryMB = 0
	}
	if c.MaxMemoryBytes < 0 {
		c.MaxMemoryBytes = 0
	}
	SetDefaultDuration(&c.DefaultTTL, 30*time.Minute)
	if c.DefaultTTL < 0 {
		c.DefaultTTL = 0
	}
	SetDefaultDuration(&c.CleanupInterval, 5*time.Minute)
	if c.CleanupInterval < 0 {
		c.CleanupInterval = 5 * time.Minute
	}
	SetDefaultInt(&c.ShardCount, 32)
	if c.ShardCount <= 0 {
		c.ShardCount = 32
	}
	if !hash.IsPowerOfTwo(c.ShardCount) {
		c.ShardCount = hash.NextPowerOfTwo(c.ShardCount)
	}
	if !c.EvictionPolicy.IsValid() {
		c.EvictionPolicy = EvictLRU
	}
}

// Validate checks the configuration values using struct tags.
func (c *Memory) Validate() error {
	if c.base.validate == nil {
		c.base = *NewBase("memory")
	}
	if err := c.base.validate.Struct(c); err != nil {
		return translateValidationErrors(err)
	}
	return nil
}

// ValidateWithContext sets the operation prefix and validates the configuration.
func (c *Memory) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a shallow copy of the configuration. Pointer fields are shared.
func (c *Memory) Clone() *Memory {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// DefaultMemory returns a new Memory configuration with all defaults applied.
func DefaultMemory() *Memory {
	c := &Memory{}
	c.SetDefaults()
	return c
}
