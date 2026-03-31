package config

import (
	"fmt"
	"strings"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// Redis holds configuration for the Redis-backed cache.
type Redis struct {
	base Base

	// Addr is the server address.  Comma-separated for cluster mode.
	// Default: "localhost:6379"
	Addr string

	// Username for ACL-based authentication (Redis 6+).
	Username string

	// Password for authentication.
	Password string

	// DB is the Redis database number (0-15 for standalone, ignored in cluster).
	// Default: 0
	DB int

	// PoolSize is the maximum number of socket connections.  Default: 10.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.  Default: 2.
	MinIdleConns int

	// MaxRetries is the maximum number of command retries.  Default: 3.
	MaxRetries int

	// RetryBackoff is the base delay between retries.  Default: 100 ms.
	RetryBackoff time.Duration

	// DialTimeout is the connection dial timeout.  Default: 5 s.
	DialTimeout time.Duration

	// ReadTimeout is the read-operation timeout.  Default: 3 s.
	ReadTimeout time.Duration

	// WriteTimeout is the write-operation timeout.  Default: 3 s.
	WriteTimeout time.Duration

	// DefaultTTL is the TTL applied when callers pass 0.  Default: 1 h.
	DefaultTTL time.Duration

	// KeyPrefix is prepended to all keys for namespacing.  Default: "".
	KeyPrefix string

	// EnablePipeline enables pipelining for batch operations.  Default: true.
	EnablePipeline bool

	// EnableMetrics enables per-operation metrics.  Default: false.
	EnableMetrics bool

	// EnableDistributedStampedeProtection guards GetOrSet with a Redis lock.
	// Default: false.
	EnableDistributedStampedeProtection bool

	// StampedeLockTTL is the TTL for the distributed GetOrSet lock.
	// Default: 5 s.
	StampedeLockTTL time.Duration

	// StampedeWaitTimeout controls how long waiters poll before computing
	// locally.  Default: 2 s.
	StampedeWaitTimeout time.Duration

	// StampedeRetryInterval controls polling cadence while waiting.
	// Default: 25 ms.
	StampedeRetryInterval time.Duration
}

// SetDefaults fills zero fields with production-safe defaults.
// Must be called before Validate.
func (c *Redis) SetDefaults() {
	SetDefaultString(&c.Addr, "localhost:6379")

	if c.DB < 0 {
		c.DB = 0
	}

	SetDefaultInt(&c.PoolSize, 10)
	SetDefaultInt(&c.MinIdleConns, 2)
	if c.MinIdleConns > c.PoolSize {
		c.MinIdleConns = c.PoolSize
	}

	SetDefaultInt(&c.MaxRetries, 3)
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}

	SetDefaultDuration(&c.RetryBackoff, 100*time.Millisecond)
	SetDefaultDuration(&c.DialTimeout, 5*time.Second)
	SetDefaultDuration(&c.ReadTimeout, 3*time.Second)
	SetDefaultDuration(&c.WriteTimeout, 3*time.Second)

	if c.DialTimeout <= 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 3 * time.Second
	}

	SetDefaultDuration(&c.DefaultTTL, time.Hour)
	if c.DefaultTTL < 0 {
		c.DefaultTTL = 0
	}

	SetDefaultBool(&c.EnablePipeline, true)

	SetDefaultDuration(&c.StampedeLockTTL, 5*time.Second)
	SetDefaultDuration(&c.StampedeWaitTimeout, 2*time.Second)
	SetDefaultDuration(&c.StampedeRetryInterval, 25*time.Millisecond)
}

// Validate checks the configuration for semantic correctness.
// Always call SetDefaults before Validate.
func (c *Redis) Validate() error {
	if err := c.validateCore(); err != nil {
		return err
	}
	if err := c.validateTiming(); err != nil {
		return err
	}
	return c.validateOptional()
}

func (c *Redis) validateCore() error {
	if err := c.base.validateRequired("address", c.Addr, "redis.validate"); err != nil {
		return err
	}
	if c.DB < 0 {
		return _errors.InvalidConfig("redis.validate",
			fmt.Sprintf("database number cannot be negative (got: %d)", c.DB))
	}
	if c.DB > 15 {
		return _errors.InvalidConfig("redis.validate",
			fmt.Sprintf("database number cannot exceed 15 (got: %d)", c.DB))
	}
	if err := c.base.validatePositiveInt("pool size", c.PoolSize, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validatePositiveInt("min idle connections", c.MinIdleConns, "redis.validate"); err != nil {
		return err
	}
	if c.MinIdleConns > c.PoolSize {
		return _errors.InvalidConfig("redis.validate",
			fmt.Sprintf("min idle connections (%d) cannot exceed pool size (%d)",
				c.MinIdleConns, c.PoolSize))
	}
	return c.base.validatePositiveInt("max retries", c.MaxRetries, "redis.validate")
}

func (c *Redis) validateTiming() error {
	if err := c.base.validateDuration("retry backoff", c.RetryBackoff, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDurationPositive("dial timeout", c.DialTimeout, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDurationPositive("read timeout", c.ReadTimeout, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDurationPositive("write timeout", c.WriteTimeout, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDuration("default TTL", c.DefaultTTL, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDuration("stampede lock ttl", c.StampedeLockTTL, "redis.validate"); err != nil {
		return err
	}
	if err := c.base.validateDuration("stampede wait timeout", c.StampedeWaitTimeout, "redis.validate"); err != nil {
		return err
	}
	return c.base.validateDuration(
		"stampede retry interval",
		c.StampedeRetryInterval,
		"redis.validate",
	)
}

func (c *Redis) validateOptional() error {
	if c.EnableDistributedStampedeProtection {
		if c.StampedeLockTTL <= 0 {
			return _errors.InvalidConfig("redis.validate", "stampede lock ttl must be positive")
		}
		if c.StampedeRetryInterval <= 0 {
			return _errors.InvalidConfig(
				"redis.validate",
				"stampede retry interval must be positive",
			)
		}
	}
	if len(c.KeyPrefix) > 256 {
		return _errors.InvalidConfig("redis.validate",
			fmt.Sprintf("key prefix is too long (%d bytes), maximum is 256", len(c.KeyPrefix)))
	}
	return nil
}

// ValidateWithContext validates with a custom operation prefix.
func (c *Redis) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a shallow copy.
func (c *Redis) Clone() *Redis {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// IsClusterMode reports whether Addr represents a cluster (comma-separated or
// bracket-quoted list).
func (c *Redis) IsClusterMode() bool {
	return c.Addr != "" && (c.Addr[0] == '[' || strings.Contains(c.Addr, ","))
}

// DefaultRedis returns a Redis config with sensible defaults.
func DefaultRedis() *Redis {
	c := &Redis{}
	c.SetDefaults()
	return c
}

// DefaultRedisWithAddress returns a Redis config for a single address.
func DefaultRedisWithAddress(addr string) *Redis {
	c := DefaultRedis()
	c.Addr = addr
	return c
}

// DefaultRedisCluster returns a Redis config optimized for cluster mode.
func DefaultRedisCluster(addresses ...string) *Redis {
	c := DefaultRedis()
	if len(addresses) > 0 {
		c.Addr = strings.Join(addresses, ",")
	}
	c.PoolSize = 20
	c.MinIdleConns = 5
	return c
}
