package config

import (
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// Layered holds configuration for the two-level (L1 memory + L2 Redis) cache.
type Layered struct {
	base Base

	// L1Config configures the in-memory layer.  Default: DefaultMemory().
	L1Config *Memory

	// L2Config configures the Redis layer.  Default: DefaultRedis().
	L2Config *Redis

	// PromoteOnHit promotes L2 hits to L1.  Default: true.
	PromoteOnHit bool

	// PromoteOnHitSet is true when PromoteOnHit was set explicitly by the caller
	// so that SetDefaults respects the choice even when the value is false.
	PromoteOnHitSet bool

	// WriteBack enables asynchronous writes to L2.  Default: false.
	WriteBack bool

	// WriteBackQueueSize is the async queue depth.  Default: 512.
	WriteBackQueueSize int

	// WriteBackWorkers is the goroutine count draining the queue.  Default: 4.
	WriteBackWorkers int

	// NegativeTTL is the TTL for negative-cache sentinels.  Default: 30 s.
	NegativeTTL time.Duration

	// SyncEnabled enables L1 invalidation via Redis Pub/Sub.  Default: false.
	SyncEnabled bool

	// SyncChannel is the Pub/Sub channel for invalidation.
	// Default: "cache:invalidate".
	SyncChannel string

	// SyncBufferSize is the buffer depth of the invalidation channel.
	// Default: 1 000.
	SyncBufferSize int

	// L1TTLOverride, when > 0, overrides the L2 TTL used when promoting to L1.
	// Default: 0 (inherit L2 TTL capped by L1.DefaultTTL).
	L1TTLOverride time.Duration

	// EnableStats enables layered-specific statistics.  Default: true.
	EnableStats bool

	// disableL1Promotion is an internal maintenance flag; not part of the
	// public API.
	disableL1Promotion bool //nolint:unused // reserved for future maintenance mode
}

// SetDefaults fills zero fields with production-safe defaults.
// Must be called before Validate.
func (c *Layered) SetDefaults() {
	if c.L1Config == nil {
		c.L1Config = &Memory{}
	}
	c.L1Config.SetDefaults()

	if c.L2Config == nil {
		c.L2Config = &Redis{}
	}
	c.L2Config.SetDefaults()

	if !c.PromoteOnHitSet {
		SetDefaultBool(&c.PromoteOnHit, true)
	}

	SetDefaultDuration(&c.NegativeTTL, 30*time.Second)
	if c.NegativeTTL < 0 {
		c.NegativeTTL = 0
	}

	SetDefaultString(&c.SyncChannel, "cache:invalidate")
	SetDefaultInt(&c.SyncBufferSize, 1000)
	if c.SyncBufferSize < 0 {
		c.SyncBufferSize = 0
	}

	SetDefaultInt(&c.WriteBackQueueSize, 512)
	if c.WriteBackQueueSize <= 0 {
		c.WriteBackQueueSize = 512
	}

	SetDefaultInt(&c.WriteBackWorkers, 4)
	if c.WriteBackWorkers <= 0 {
		c.WriteBackWorkers = 4
	}

	if c.L1TTLOverride < 0 {
		c.L1TTLOverride = 0
	}

	SetDefaultBool(&c.EnableStats, true)
}

// Validate checks the configuration for semantic correctness.
// Always call SetDefaults before Validate.
func (c *Layered) Validate() error {
	if c.L1Config != nil {
		if err := c.L1Config.Validate(); err != nil {
			return _errors.New("layered.validate", "", err)
		}
	}
	if c.L2Config != nil {
		if err := c.L2Config.Validate(); err != nil {
			return _errors.New("layered.validate", "", err)
		}
	}
	if err := c.base.validateDuration("negative TTL", c.NegativeTTL, "layered.validate"); err != nil {
		return err
	}
	if c.SyncEnabled {
		if err := c.base.validateRequired("sync channel", c.SyncChannel, "layered.validate"); err != nil {
			return err
		}
		if err := c.base.validatePositiveInt("sync buffer size", c.SyncBufferSize, "layered.validate"); err != nil {
			return err
		}
	}
	if c.WriteBack {
		if err := c.base.validatePositiveInt(
			"write-back queue size",
			c.WriteBackQueueSize,
			"layered.validate",
		); err != nil {
			return err
		}
		if err := c.base.validatePositiveInt("write-back workers", c.WriteBackWorkers, "layered.validate"); err != nil {
			return err
		}
	}
	return c.base.validateDuration("L1 TTL override", c.L1TTLOverride, "layered.validate")
}

// ValidateWithContext validates with a custom operation prefix.
func (c *Layered) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a deep copy.
func (c *Layered) Clone() *Layered {
	if c == nil {
		return nil
	}
	clone := *c
	if c.L1Config != nil {
		clone.L1Config = c.L1Config.Clone()
	}
	if c.L2Config != nil {
		clone.L2Config = c.L2Config.Clone()
	}
	return &clone
}

// DefaultLayered returns a Layered config with sensible defaults.
func DefaultLayered() *Layered {
	c := &Layered{}
	c.SetDefaults()
	return c
}

// DefaultLayeredWithRedis returns a Layered config with a custom Redis address.
func DefaultLayeredWithRedis(redisAddr string) *Layered {
	c := DefaultLayered()
	c.L2Config.Addr = redisAddr
	return c
}

// DefaultLayeredWithWriteBack returns a Layered config with write-back enabled.
func DefaultLayeredWithWriteBack(redisAddr string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.WriteBack = true
	return c
}

// DefaultLayeredWithSync returns a Layered config with Pub/Sub sync enabled.
func DefaultLayeredWithSync(redisAddr, syncChannel string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.SyncEnabled = true
	c.SyncChannel = syncChannel
	return c
}

// DefaultLayeredWithTTL returns a Layered config with explicit TTLs.
func DefaultLayeredWithTTL(l1TTL, l2TTL, negativeTTL time.Duration) *Layered {
	c := DefaultLayered()
	c.L1Config.DefaultTTL = l1TTL
	c.L2Config.DefaultTTL = l2TTL
	c.NegativeTTL = negativeTTL
	return c
}
