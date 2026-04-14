package config

import "time"

// EvictionPolicy represents a cache eviction algorithm. Use one of the
// predefined constants (EvictLRU, EvictLFU, etc.) when configuring the
// memory backend.
type EvictionPolicy int

const (
	// EvictLRU evicts the least recently used entry.
	EvictLRU EvictionPolicy = iota
	// EvictLFU evicts the least frequently used entry.
	EvictLFU
	// EvictFIFO evicts entries in first-in-first-out order.
	EvictFIFO
	// EvictLIFO evicts entries in last-in-first-out order.
	EvictLIFO
	// EvictMRU evicts the most recently used entry.
	EvictMRU
	// EvictRR evicts a random entry.
	EvictRR
	// EvictTinyLFU uses the TinyLFU admission policy.
	EvictTinyLFU
)

// String returns the lowercase name of the eviction policy.
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

// IsValid returns true if the eviction policy is a recognized value.
func (p EvictionPolicy) IsValid() bool { return p >= EvictLRU && p <= EvictTinyLFU }

// Memory holds the configuration for the in-memory cache backend. Use
// DefaultMemory to create a fully-initialized instance, or apply options via
// the memory.With* functions.
type Memory struct {
	base Base
	// MaxEntries is the maximum number of cache entries.
	MaxEntries int `config:"max_entries" default:"10000" validate:"min=1"`
	// MaxMemoryMB is the maximum memory in megabytes.
	MaxMemoryMB int `config:"max_memory_mb" default:"100" validate:"min=0"`
	// MaxMemoryBytes is the maximum memory in bytes (overrides MaxMemoryMB if set).
	MaxMemoryBytes int64 `config:"max_memory_bytes" validate:"min=0"`
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL time.Duration `config:"default_ttl" default:"30m" validate:"gte=0"`
	// CleanupInterval is the interval for the background expiry janitor.
	CleanupInterval time.Duration `config:"cleanup_interval" default:"5m" validate:"gte=0"`
	// ShardCount is the number of internal shards (must be a power of two).
	ShardCount int `config:"shard_count" default:"32" validate:"min=1,power_of_two"`
	// EvictionPolicy selects the eviction algorithm.
	EvictionPolicy EvictionPolicy `config:"eviction_policy" default:"lru"`
	// OnEvictionPolicy is an optional callback invoked when an entry is evicted.
	OnEvictionPolicy func(key, reason string)
	// EnableMetrics enables stats collection.
	EnableMetrics bool `config:"enable_metrics" default:"false"`
	// Interceptors holds []observability.Interceptor for the observability chain.
	Interceptors any
}

// SetDefaults populates zero-valued fields with their default values.
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
	if !isPowerOfTwo(c.ShardCount) {
		c.ShardCount = nextPowerOfTwo(c.ShardCount)
	}
	if !c.EvictionPolicy.IsValid() {
		c.EvictionPolicy = EvictLRU
	}
}

// Validate checks the Memory config for consistency and returns the first
// error encountered.

func (c *Memory) Validate() error {
	if c.base.validate == nil {
		c.base = *NewBase("memory")
	}
	if err := c.base.validate.Struct(c); err != nil {
		return translateValidationErrors(err)
	}
	return nil
}

func (c *Memory) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a shallow copy of the Memory config. Function fields are
// copied by reference.
func (c *Memory) Clone() *Memory {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// DefaultMemory returns a Memory config with all defaults applied.
func DefaultMemory() *Memory {
	c := &Memory{}
	c.SetDefaults()
	return c
}

// isPowerOfTwo returns true if n is a positive power of two.
func isPowerOfTwo(n int) bool { return n > 0 && (n&(n-1)) == 0 }

// nextPowerOfTwo rounds n up to the next power of two.
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
