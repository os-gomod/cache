package config

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
)

// RedisTLS holds TLS configuration for Redis connections.
type RedisTLS struct {
	Enabled            bool   `config:"enabled" default:"false" validate:"boolean"`
	CertFile           string `config:"cert_file" validate:"required_if=Enabled true"`
	KeyFile            string `config:"key_file" validate:"required_if=Enabled true"`
	CAFile             string `config:"ca_file"`
	InsecureSkipVerify bool   `config:"insecure_skip_verify" default:"false"`
	ServerName         string `config:"server_name"`
}

// Redis holds configuration for the Redis cache backend.
type Redis struct {
	base                                Base
	Addr                                string        `config:"addr" default:"localhost:6379" validate:"required"`
	TLS                                 *RedisTLS     `config:"tls"`
	Username                            string        `config:"username"`
	Password                            string        `config:"password"`
	DB                                  int           `config:"db" default:"0" validate:"min=0,max=15"`
	PoolSize                            int           `config:"pool_size" default:"10" validate:"min=1,max=1000"`
	PoolTimeout                         time.Duration `config:"pool_timeout" default:"4s" validate:"gte=0"`
	MinIdleConns                        int           `config:"min_idle_conns" default:"2" validate:"min=0"`
	MaxRetries                          int           `config:"max_retries" default:"3" validate:"min=0,max=10"`
	ConnMaxIdleTime                     time.Duration `config:"conn_max_idle_time" default:"5m" validate:"gte=0"`
	RetryBackoff                        time.Duration `config:"retry_backoff" default:"100ms" validate:"gte=0"`
	DialTimeout                         time.Duration `config:"dial_timeout" default:"5s" validate:"gte=0"`
	ReadTimeout                         time.Duration `config:"read_timeout" default:"3s" validate:"gte=0"`
	WriteTimeout                        time.Duration `config:"write_timeout" default:"3s" validate:"gte=0"`
	DefaultTTL                          time.Duration `config:"default_ttl" default:"1h" validate:"gte=0"`
	KeyPrefix                           string        `config:"key_prefix"`
	EnablePipeline                      bool          `config:"enable_pipeline" default:"true"`
	EnableMetrics                       bool          `config:"enable_metrics" default:"false"`
	EnableDistributedStampedeProtection bool          `config:"enable_stampede_protection" default:"false"`
	StampedeLockTTL                     time.Duration `config:"stampede_lock_ttl" default:"5s" validate:"gte=0"`
	StampedeWaitTimeout                 time.Duration `config:"stampede_wait_timeout" default:"2s" validate:"gte=0"`
	StampedeRetryInterval               time.Duration `config:"stampede_retry_interval" default:"25ms" validate:"gte=0"`
	Interceptors                        any
}

// SetDefaults fills zero-valued fields with sensible defaults.
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

// Validate checks the Redis configuration for correctness using struct tags.
func (c *Redis) Validate() error {
	if c.base.validate == nil {
		c.base = *NewBase("redis")
	}
	if err := c.base.validate.Struct(c); err != nil {
		return translateValidationErrors(err)
	}
	return nil
}

// BuildTLSConfig constructs a tls.Config from the RedisTLS settings.
// Returns nil if TLS is not enabled.
func (t *RedisTLS) BuildTLSConfig() (*tls.Config, error) {
	if !t.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		//nolint:gosec // G402: InsecureSkipVerify is intentionally user-configurable
		InsecureSkipVerify: t.InsecureSkipVerify,
		ServerName:         t.ServerName,
	}
	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, cacheerrors.Wrap("redis.tls load client cert", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if t.CAFile != "" {
		ca, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, cacheerrors.Wrap("redis.tls load ca", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, cacheerrors.New("redis.tls", "invalid ca pem", err)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// ValidateWithContext sets the operation prefix and validates the configuration.
func (c *Redis) ValidateWithContext(opPrefix string) error {
	c.base.opPrefix = opPrefix
	return c.Validate()
}

// Clone returns a shallow copy of the Redis configuration.
func (c *Redis) Clone() *Redis {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// IsClusterMode reports whether the address configuration indicates Redis Cluster.
func (c *Redis) IsClusterMode() bool {
	return c.Addr != "" && (c.Addr[0] == '[' || strings.Contains(c.Addr, ","))
}

// DefaultRedis returns a new Redis configuration with all defaults applied.
func DefaultRedis() *Redis {
	c := &Redis{}
	c.SetDefaults()
	return c
}

// DefaultRedisWithAddress returns a default Redis config with the given address.
func DefaultRedisWithAddress(addr string) *Redis {
	c := DefaultRedis()
	c.Addr = addr
	return c
}

// DefaultRedisCluster returns a Redis configuration pre-configured for cluster mode.
func DefaultRedisCluster(addresses ...string) *Redis {
	c := DefaultRedis()
	if len(addresses) > 0 {
		c.Addr = strings.Join(addresses, ",")
	}
	c.PoolSize = 20
	c.MinIdleConns = 5
	return c
}
