package config

import (
	"time"
)

// Layered holds configuration for the layered (L1 memory + L2 Redis) cache backend.
type Layered struct {
	base               Base
	L1Config           *Memory `config:"l1"`
	L2Config           *Redis  `config:"l2"`
	PromoteOnHit       bool    `config:"promote_on_hit" default:"true" validate:"boolean"`
	PromoteOnHitSet    bool
	WriteBack          bool          `config:"write_back" default:"false"`
	WriteBackQueueSize int           `config:"write_back_queue_size" default:"512" validate:"min=1"`
	WriteBackWorkers   int           `config:"write_back_workers" default:"4" validate:"min=1"`
	NegativeTTL        time.Duration `config:"negative_ttl" default:"30s" validate:"gte=0"`
	SyncEnabled        bool          `config:"sync_enabled" default:"false"`
	SyncChannel        string        `config:"sync_channel" default:"cache:invalidate" validate:"required_if=SyncEnabled true"`
	SyncBufferSize     int           `config:"sync_buffer_size" default:"1000" validate:"min=1"`
	L1TTLOverride      time.Duration `config:"l1_ttl_override" validate:"gte=0"`
	EnableStats        bool          `config:"enable_stats" default:"true"`
	Interceptors       any
}

// SetDefaults fills zero-valued fields with sensible defaults.
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

// Validate checks the layered configuration for correctness.
func (c *Layered) Validate() error {
	if c.base.validate == nil {
		c.base = *NewBase("layered")
	}
	if c.L1Config != nil {
		if err := c.L1Config.Validate(); err != nil {
			return err
		}
	}
	if c.L2Config != nil {
		if err := c.L2Config.Validate(); err != nil {
			return err
		}
	}
	if err := c.base.validate.Struct(c); err != nil {
		return translateValidationErrors(err)
	}
	return nil
}

// ValidateWithContext sets the operation prefix and validates the configuration.
func (c *Layered) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a deep copy of the Layered configuration.
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

// DefaultLayered returns a new Layered configuration with all defaults applied.
func DefaultLayered() *Layered {
	c := &Layered{}
	c.SetDefaults()
	return c
}

// DefaultLayeredWithRedis returns a default layered config with the given Redis address.
func DefaultLayeredWithRedis(redisAddr string) *Layered {
	c := DefaultLayered()
	c.L2Config.Addr = redisAddr
	return c
}

// DefaultLayeredWithWriteBack returns a layered config with write-back enabled.
func DefaultLayeredWithWriteBack(redisAddr string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.WriteBack = true
	return c
}

// DefaultLayeredWithSync returns a layered config with cross-instance sync enabled.
func DefaultLayeredWithSync(redisAddr, syncChannel string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.SyncEnabled = true
	c.SyncChannel = syncChannel
	return c
}

// DefaultLayeredWithTTL returns a layered config with the specified TTL values.
func DefaultLayeredWithTTL(l1TTL, l2TTL, negativeTTL time.Duration) *Layered {
	c := DefaultLayered()
	c.L1Config.DefaultTTL = l1TTL
	c.L2Config.DefaultTTL = l2TTL
	c.NegativeTTL = negativeTTL
	return c
}
