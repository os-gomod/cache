package config

import (
	"time"
)

// Layered holds the configuration for the two-tier (L1 memory + L2 Redis)
// cache backend. Use DefaultLayered to create a fully-initialized instance,
// or apply options via the layered.With* functions.
type Layered struct {
	base Base

	// L1Config is the in-memory (L1) tier configuration.
	L1Config *Memory `config:"l1"`
	// L2Config is the Redis (L2) tier configuration.
	L2Config *Redis `config:"l2"`
	// PromoteOnHit enables promotion of L2 hits into L1.
	PromoteOnHit bool `config:"promote_on_hit" default:"true" validate:"boolean"`
	// PromoteOnHitSet tracks whether PromoteOnHit was explicitly set.
	PromoteOnHitSet bool
	// WriteBack enables asynchronous write-through to L2.
	WriteBack bool `config:"write_back" default:"false"`
	// WriteBackQueueSize is the buffer size for the write-back queue.
	WriteBackQueueSize int `config:"write_back_queue_size" default:"512" validate:"min=1"`
	// WriteBackWorkers is the number of goroutines processing the write-back queue.
	WriteBackWorkers int `config:"write_back_workers" default:"4" validate:"min=1"`
	// NegativeTTL is the TTL for caching negative (miss) results.
	NegativeTTL time.Duration `config:"negative_ttl" default:"30s" validate:"gte=0"`
	// SyncEnabled enables cache invalidation via Redis Pub/Sub.
	SyncEnabled bool `config:"sync_enabled" default:"false"`
	// SyncChannel is the Redis Pub/Sub channel for invalidation messages.
	SyncChannel string `config:"sync_channel" default:"cache:invalidate" validate:"required_if=SyncEnabled true"`
	// SyncBufferSize is the buffer size for the sync subscription.
	SyncBufferSize int `config:"sync_buffer_size" default:"1000" validate:"min=1"`
	// L1TTLOverride overrides the L1 TTL independently of L2.
	L1TTLOverride time.Duration `config:"l1_ttl_override" validate:"gte=0"`
	// EnableStats enables stats collection on the layered backend.
	EnableStats bool `config:"enable_stats" default:"true"`
	//nolint:unused // reserved for future use
	disableL1Promotion bool
	// Interceptors holds []observability.Interceptor for the observability chain.
	Interceptors any
}

// SetDefaults populates zero-valued fields with their default values.
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

// Validate checks the Layered config for consistency and returns the first
// error encountered.
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

// ValidateWithContext validates with a custom operation prefix for error
// messages.
func (c *Layered) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a deep copy of the Layered config, including cloned L1 and
// L2 sub-configs.
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

// DefaultLayered returns a Layered config with all defaults applied.
func DefaultLayered() *Layered {
	c := &Layered{}
	c.SetDefaults()
	return c
}

// DefaultLayeredWithRedis returns a Layered config with the given Redis
// address and all other defaults applied.
func DefaultLayeredWithRedis(redisAddr string) *Layered {
	c := DefaultLayered()
	c.L2Config.Addr = redisAddr
	return c
}

// DefaultLayeredWithWriteBack returns a Layered config with write-back enabled
// and the given Redis address.
func DefaultLayeredWithWriteBack(redisAddr string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.WriteBack = true
	return c
}

// DefaultLayeredWithSync returns a Layered config with Redis Pub/Sub sync
// enabled on the given channel.
func DefaultLayeredWithSync(redisAddr, syncChannel string) *Layered {
	c := DefaultLayeredWithRedis(redisAddr)
	c.SyncEnabled = true
	c.SyncChannel = syncChannel
	return c
}

// DefaultLayeredWithTTL returns a Layered config with custom TTLs for L1, L2,
// and negative caching.
func DefaultLayeredWithTTL(l1TTL, l2TTL, negativeTTL time.Duration) *Layered {
	c := DefaultLayered()
	c.L1Config.DefaultTTL = l1TTL
	c.L2Config.DefaultTTL = l2TTL
	c.NegativeTTL = negativeTTL
	return c
}
