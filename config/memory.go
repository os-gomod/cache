package config

import (
	"fmt"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// EvictionPolicy defines cache eviction algorithms.
type EvictionPolicy int

const (
	// EvictLRU evicts the least recently used entries (default).
	EvictLRU EvictionPolicy = iota
	// EvictLFU evicts the least frequently used entries.
	EvictLFU
	// EvictFIFO evicts the oldest entries by insertion order.
	EvictFIFO
	// EvictLIFO evicts the newest entries by insertion order.
	EvictLIFO
	// EvictMRU evicts the most recently used entries.
	EvictMRU
	// EvictRR evicts a random entry.
	EvictRR
	// EvictTinyLFU uses W-TinyLFU approximation for near-optimal hit rates.
	EvictTinyLFU
)

// String returns the policy name.
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

// IsValid reports whether p is a known eviction policy.
func (p EvictionPolicy) IsValid() bool { return p >= EvictLRU && p <= EvictTinyLFU }

// ----------------------------------------------------------------------------
// Memory configuration
// ----------------------------------------------------------------------------

// Memory holds configuration for the in-process sharded cache.
type Memory struct {
	base Base

	// MaxEntries is the maximum number of live entries.  Default: 10 000.
	MaxEntries int

	// MaxMemoryMB is the memory cap in MiB.  Default: 100.
	MaxMemoryMB int

	// MaxMemoryBytes is the memory cap in bytes; derived from MaxMemoryMB
	// when zero.  Set either field; SetDefaults reconciles both.
	MaxMemoryBytes int64

	// DefaultTTL is the TTL used when callers pass 0.  Default: 30 min.
	DefaultTTL time.Duration

	// CleanupInterval controls how often the janitor sweeps expired entries.
	// 0 disables background cleanup (entries expire lazily on access).
	// Default: 5 min.
	CleanupInterval time.Duration

	// ShardCount is the number of independent shards; must be a power of two.
	// Default: 32.
	ShardCount int

	// EvictionPolicy selects the eviction algorithm.  Default: EvictLRU.
	EvictionPolicy EvictionPolicy

	// OnEvictionPolicy is called whenever an entry is evicted.
	// The reason parameter is "capacity" or "expired".
	OnEvictionPolicy func(key, reason string)

	// EnableMetrics enables collection of per-operation statistics.
	// Default: false.
	EnableMetrics bool
}

// SetDefaults fills zero fields with production-safe defaults.
// Must be called before Validate.
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
		c.CleanupInterval = 0
	}

	SetDefaultInt(&c.ShardCount, 32)
	if c.ShardCount <= 0 {
		c.ShardCount = 32
	}
	if !isPowerOfTwo(c.ShardCount) {
		c.ShardCount = nextPowerOfTwo(c.ShardCount)
	}

	if !c.EvictionPolicy.IsValid() {
		c.EvictionPolicy = EvictLRU
	}
}

// Validate checks the configuration for semantic correctness.
// Always call SetDefaults before Validate.
func (c *Memory) Validate() error {
	if err := c.base.validateNonNegative("max entries", c.MaxEntries, "memory.validate"); err != nil {
		return err
	}
	if err := c.base.validateNonNegative("max memory MB", c.MaxMemoryMB, "memory.validate"); err != nil {
		return err
	}
	if c.MaxMemoryBytes < 0 {
		return _errors.InvalidConfig("memory.validate",
			fmt.Sprintf("max memory bytes cannot be negative (got: %d)", c.MaxMemoryBytes))
	}
	if c.MaxEntries == 0 && c.MaxMemoryMB == 0 && c.MaxMemoryBytes == 0 {
		return _errors.InvalidConfig("memory.validate",
			"at least one of max entries or max memory must be set")
	}
	if err := c.base.validateDuration("default TTL", c.DefaultTTL, "memory.validate"); err != nil {
		return err
	}
	if err := c.base.validateDuration("cleanup interval", c.CleanupInterval, "memory.validate"); err != nil {
		return err
	}
	if err := c.base.validateNonNegative("shard count", c.ShardCount, "memory.validate"); err != nil {
		return err
	}
	if c.ShardCount > 0 && !isPowerOfTwo(c.ShardCount) {
		return _errors.InvalidConfig("memory.validate",
			fmt.Sprintf("shard count must be a power of two (got: %d)", c.ShardCount))
	}
	if !c.EvictionPolicy.IsValid() {
		return _errors.InvalidConfig("memory.validate",
			fmt.Sprintf("invalid eviction policy: %d", c.EvictionPolicy))
	}
	return nil
}

// ValidateWithContext validates with a custom operation prefix.
func (c *Memory) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a deep copy.  The OnEvictionPolicy function is shared
// (functions are not deeply copyable); callers that need independent
// callbacks should set the field after cloning.
func (c *Memory) Clone() *Memory {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// DefaultMemory returns a Memory config with sensible defaults applied.
func DefaultMemory() *Memory {
	c := &Memory{}
	c.SetDefaults()
	return c
}

// isPowerOfTwo reports whether n is a power of two.
func isPowerOfTwo(n int) bool { return n > 0 && (n&(n-1)) == 0 }

// nextPowerOfTwo returns the smallest power of two ≥ n.
func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
