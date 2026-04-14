package config

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

type RedisTLS struct {
	Enabled            bool   `config:"enabled" default:"false" validate:"boolean"`
	CertFile           string `config:"cert_file" validate:"required_if=Enabled true"`
	KeyFile            string `config:"key_file" validate:"required_if=Enabled true"`
	CAFile             string `config:"ca_file"`
	InsecureSkipVerify bool   `config:"insecure_skip_verify" default:"false"`
	ServerName         string `config:"server_name"`
}

// Redis holds the configuration for the Redis cache backend. Use DefaultRedis
// to create a fully-initialized instance, or apply options via the
// redis.With* functions.
type Redis struct {
	base Base
	// Addr is the Redis server address (comma-separated for clusters).
	Addr string `config:"addr" default:"localhost:6379" validate:"required"`
	// TLS holds TLS configuration for secure Redis connections.
	TLS *RedisTLS `config:"tls"`
	// Username is the Redis ACL username.
	Username string `config:"username"`
	// Password is the Redis ACL password.
	Password string `config:"password"`
	// DB is the Redis database number (0-15).
	DB int `config:"db" default:"0" validate:"min=0,max=15"`
	// PoolSize is the connection pool size.
	PoolSize int `config:"pool_size" default:"10" validate:"min=1,max=1000"`
	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int `config:"min_idle_conns" default:"2" validate:"min=0"`
	// MaxRetries is the maximum number of retries per command.
	MaxRetries int `config:"max_retries" default:"3" validate:"min=0,max=10"`
	// RetryBackoff is the base delay between retries.
	RetryBackoff time.Duration `config:"retry_backoff" default:"100ms" validate:"gte=0"`
	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration `config:"dial_timeout" default:"5s" validate:"gte=0"`
	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration `config:"read_timeout" default:"3s" validate:"gte=0"`
	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration `config:"write_timeout" default:"3s" validate:"gte=0"`
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL time.Duration `config:"default_ttl" default:"1h" validate:"gte=0"`
	// KeyPrefix is prepended to all cache keys.
	KeyPrefix string `config:"key_prefix"`
	// EnablePipeline enables Redis pipelining for batch operations.
	EnablePipeline bool `config:"enable_pipeline" default:"true"`
	// EnableMetrics enables stats collection.
	EnableMetrics bool `config:"enable_metrics" default:"false"`
	// EnableDistributedStampedeProtection enables distributed stampede locks.
	EnableDistributedStampedeProtection bool `config:"enable_stampede_protection" default:"false"`
	// StampedeLockTTL is the TTL for distributed stampede lock keys.
	StampedeLockTTL time.Duration `config:"stampede_lock_ttl" default:"5s" validate:"gte=0"`
	// StampedeWaitTimeout is how long to wait for a stampede lock.
	StampedeWaitTimeout time.Duration `config:"stampede_wait_timeout" default:"2s" validate:"gte=0"`
	// StampedeRetryInterval is the interval between stampede lock retries.
	StampedeRetryInterval time.Duration `config:"stampede_retry_interval" default:"25ms" validate:"gte=0"`
	// Interceptors holds []observability.Interceptor for the observability chain.
	Interceptors any
}

// SetDefaults populates zero-valued fields with their default values.

func (c *Redis) SetDefaults() {
	SetDefaultString(&c.Addr, "localhost:6379")
	if c.DB < 0 {
		c.DB = 0
	}
	if c.TLS == nil {
		c.TLS = &RedisTLS{}
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

// Validate checks the Redis config for consistency and returns the first
// error encountered.
func (c *Redis) Validate() error {
	if c.base.validate == nil {
		c.base = *NewBase("redis")
	}

	if err := c.base.validate.Struct(c); err != nil {
		return translateValidationErrors(err) // optional: make errors nicer
	}
	return nil
}

func (t *RedisTLS) BuildTLSConfig() (*tls.Config, error) {
	if !t.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify, //nolint:gosec // explicit opt-in for local/test deployments
		ServerName:         t.ServerName,
	}
	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, _errors.Wrap("redis.tls load client cert", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if t.CAFile != "" {
		ca, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, _errors.Wrap("redis.tls load ca", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, _errors.New("redis.tls", "invalid ca pem", err)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// ValidateWithContext validates with a custom operation prefix for error
// messages.
func (c *Redis) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a shallow copy of the Redis config.
func (c *Redis) Clone() *Redis {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// IsClusterMode returns true if the address indicates a Redis Cluster
// configuration (IPv6 bracket or comma-separated addresses).
func (c *Redis) IsClusterMode() bool {
	return c.Addr != "" && (c.Addr[0] == '[' || strings.Contains(c.Addr, ","))
}

// DefaultRedis returns a Redis config with all defaults applied.
func DefaultRedis() *Redis {
	c := &Redis{}
	c.SetDefaults()
	return c
}

// DefaultRedisWithAddress returns a Redis config with the given address and
// all other defaults applied.
func DefaultRedisWithAddress(addr string) *Redis {
	c := DefaultRedis()
	c.Addr = addr
	return c
}

// DefaultRedisCluster returns a Redis config configured for cluster mode with
// the given node addresses.
func DefaultRedisCluster(addresses ...string) *Redis {
	c := DefaultRedis()
	if len(addresses) > 0 {
		c.Addr = strings.Join(addresses, ",")
	}
	c.PoolSize = 20
	c.MinIdleConns = 5
	return c
}
