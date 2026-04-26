package config

import (
	"fmt"
	"time"
)

// Layered holds configuration for the two-tier layered cache that
// combines a fast in-memory L1 cache with a persistent Redis L2 cache.
// It supports write-back caching, promotion on hit, and negative
// caching (caching "key not found" results).
type Layered struct {
	// L1Config is the configuration for the in-memory (L1) cache layer.
	// This field is required.
	L1Config *Memory `validate:"required"`

	// L2Config is the configuration for the Redis (L2) cache layer.
	// This field is required.
	L2Config *Redis `validate:"required"`

	// PromoteOnHit controls whether a cache hit in L2 causes the entry
	// to be promoted (copied) into L1. When true, repeated reads for
	// the same key become L1 hits after the first L2 hit.
	PromoteOnHit bool `default:"true"`

	// WriteBack enables asynchronous write-behind behaviour. When true,
	// writes to L2 are queued and performed by background workers,
	// reducing write latency at the cost of potential data loss on crash.
	WriteBack bool `default:"false"`

	// WriteBackQueueSize is the maximum number of pending write-back
	// operations. When the queue is full, writes fall back to synchronous
	// mode until the queue drains.
	WriteBackQueueSize int `default:"512" validate:"gte=1"`

	// WriteBackWorkers is the number of background goroutines that
	// drain the write-back queue.
	WriteBackWorkers int `default:"4" validate:"gte=1"`

	// NegativeTTL controls how long "key not found" results are cached.
	// This prevents thundering-herd scenarios when many requests
	// simultaneously miss for the same key. A zero value disables
	// negative caching.
	NegativeTTL time.Duration `default:"30s" validate:"gte=0"`
}

// SetDefaults populates zero-value fields with sensible production defaults.
// Sub-configs (L1, L2) are also initialized with their defaults if nil.
func (c *Layered) SetDefaults() {
	c.PromoteOnHit = true

	if c.L1Config == nil {
		c.L1Config = DefaultMemory()
	} else {
		c.L1Config.SetDefaults()
	}

	if c.L2Config == nil {
		c.L2Config = DefaultRedis()
	} else {
		c.L2Config.SetDefaults()
	}

	setDefaultInt(&c.WriteBackQueueSize, 512)
	setDefaultInt(&c.WriteBackWorkers, 4)
	if c.NegativeTTL == 0 {
		c.NegativeTTL = 30 * time.Second
	}
}

// Validate checks all configuration constraints including nested L1 and L2
// configs. Returns a descriptive error if any constraint is violated.
func (c *Layered) Validate() error {
	base := NewBase()

	if c.L1Config == nil {
		return cacheErrors("Layered.Validate", "L1 config is required", nil)
	}
	if c.L2Config == nil {
		return cacheErrors("Layered.Validate", "L2 config is required", nil)
	}

	// Validate only Layered-owned fields here so nested errors can be wrapped
	// with explicit L1/L2 context below.
	topLevel := struct {
		WriteBackQueueSize int           `validate:"gte=1"`
		WriteBackWorkers   int           `validate:"gte=1"`
		NegativeTTL        time.Duration `validate:"gte=0"`
	}{
		WriteBackQueueSize: c.WriteBackQueueSize,
		WriteBackWorkers:   c.WriteBackWorkers,
		NegativeTTL:        c.NegativeTTL,
	}
	if err := base.ValidateStruct(topLevel); err != nil {
		return cacheErrors("Layered.Validate", err.Error(), err)
	}

	// Validate nested configs.
	if err := c.L1Config.Validate(); err != nil {
		return cacheErrors("Layered.Validate", "L1 config validation failed", err)
	}
	if err := c.L2Config.Validate(); err != nil {
		return cacheErrors("Layered.Validate", "L2 config validation failed", err)
	}

	return nil
}

// Clone returns a deep copy of this configuration, including nested L1 and
// L2 configs. The returned config is safe to modify without affecting the
// original.
func (c *Layered) Clone() *Layered {
	clone := &Layered{
		PromoteOnHit:       c.PromoteOnHit,
		WriteBack:          c.WriteBack,
		WriteBackQueueSize: c.WriteBackQueueSize,
		WriteBackWorkers:   c.WriteBackWorkers,
		NegativeTTL:        c.NegativeTTL,
	}
	if c.L1Config != nil {
		clone.L1Config = c.L1Config.Clone()
	}
	if c.L2Config != nil {
		clone.L2Config = c.L2Config.Clone()
	}
	return clone
}

// DefaultLayered returns a Layered configuration with all defaults applied,
// including fully populated L1 (Memory) and L2 (Redis) sub-configs.
func DefaultLayered() *Layered {
	c := &Layered{}
	c.SetDefaults()
	return c
}

// String returns a human-readable summary of the configuration.
func (c *Layered) String() string {
	return fmt.Sprintf(
		"Layered{PromoteOnHit:%v, WriteBack:%v, WriteBackQueueSize:%d, WriteBackWorkers:%d, NegativeTTL:%s, L1:%s, L2:%s}",
		c.PromoteOnHit,
		c.WriteBack,
		c.WriteBackQueueSize,
		c.WriteBackWorkers,
		c.NegativeTTL,
		c.L1Config,
		c.L2Config,
	)
}
