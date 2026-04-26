package config

import (
	"fmt"
	"time"
)

// Redis holds configuration for the Redis cache backend. It covers
// connection pooling, timeouts, retries, authentication, and default
// TTL settings.
type Redis struct {
	// Addr is the Redis server address in "host:port" format.
	Addr string `default:"localhost:6379" validate:"required"`

	// Username is used for Redis 6+ ACL authentication. Leave empty
	// for legacy (password-only) auth.
	Username string `default:""`

	// Password is the Redis authentication password. Leave empty for
	// unauthenticated connections.
	Password string `default:""`

	// DB selects the Redis database (0-15).
	DB int `default:"0" validate:"gte=0,lte=15"`

	// PoolSize is the maximum number of connections in the pool.
	PoolSize int `default:"10" validate:"gte=1,lte=1000"`

	// MinIdleConns is the minimum number of idle connections to keep
	// open. This helps avoid connection-setup latency for steady-state
	// workloads.
	MinIdleConns int `default:"2" validate:"gte=0"`

	// MaxRetries is the maximum number of retries for commands that
	// fail due to transient network errors.
	MaxRetries int `default:"3" validate:"gte=0,lte=10"`

	// DialTimeout is the timeout for establishing a new connection.
	DialTimeout time.Duration `default:"5s" validate:"gte=0"`

	// ReadTimeout is the timeout for reading a command response.
	ReadTimeout time.Duration `default:"3s" validate:"gte=0"`

	// WriteTimeout is the timeout for writing a command.
	WriteTimeout time.Duration `default:"3s" validate:"gte=0"`

	// DefaultTTL is applied to entries that don't specify an explicit TTL.
	DefaultTTL time.Duration `default:"1h" validate:"gte=0"`

	// KeyPrefix is prepended to all cache keys to provide namespace
	// isolation within a shared Redis instance.
	KeyPrefix string `default:""`
}

// SetDefaults populates zero-value fields with sensible production defaults.
func (c *Redis) SetDefaults() {
	setDefaultString(&c.Addr, "localhost:6379")
	setDefaultInt(&c.DB, 0)
	setDefaultInt(&c.PoolSize, 10)
	setDefaultInt(&c.MinIdleConns, 2)
	setDefaultInt(&c.MaxRetries, 3)
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.DefaultTTL == 0 {
		c.DefaultTTL = 1 * time.Hour
	}
}

// Validate checks all configuration constraints and returns a descriptive
// error if any constraint is violated.
func (c *Redis) Validate() error {
	base := NewBase()
	return base.ValidateStruct(c)
}

// Clone returns a deep copy of this configuration. The returned config
// is safe to modify without affecting the original.
func (c *Redis) Clone() *Redis {
	return &Redis{
		Addr:         c.Addr,
		Username:     c.Username,
		Password:     c.Password,
		DB:           c.DB,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
		MaxRetries:   c.MaxRetries,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
		DefaultTTL:   c.DefaultTTL,
		KeyPrefix:    c.KeyPrefix,
	}
}

// DefaultRedis returns a Redis configuration with all defaults applied.
func DefaultRedis() *Redis {
	c := &Redis{}
	c.SetDefaults()
	return c
}

// String returns a human-readable summary of the configuration.
// Password is masked for security.
func (c *Redis) String() string {
	masked := "***"
	if c.Password == "" {
		masked = ""
	}
	return fmt.Sprintf(
		"Redis{Addr:%q, DB:%d, PoolSize:%d, MinIdleConns:%d, MaxRetries:%d, DialTimeout:%s, ReadTimeout:%s, WriteTimeout:%s, DefaultTTL:%s, KeyPrefix:%q, Password:%q}",
		c.Addr,
		c.DB,
		c.PoolSize,
		c.MinIdleConns,
		c.MaxRetries,
		c.DialTimeout,
		c.ReadTimeout,
		c.WriteTimeout,
		c.DefaultTTL,
		c.KeyPrefix,
		masked,
	)
}
