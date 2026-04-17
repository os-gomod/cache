package redis

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

// Option is a functional option for configuring a Redis cache.
type Option func(*config.Redis)

// WithAddress sets the Redis server address (host:port or comma-separated for cluster).
func WithAddress(addr string) Option { return func(c *config.Redis) { c.Addr = addr } }

// WithUsername sets the Redis ACL username.
func WithUsername(u string) Option { return func(c *config.Redis) { c.Username = u } }

// WithPassword sets the Redis password.
func WithPassword(p string) Option { return func(c *config.Redis) { c.Password = p } }

// WithDB sets the Redis database index.
func WithDB(db int) Option { return func(c *config.Redis) { c.DB = db } }

// WithPoolSize sets the maximum number of socket connections.
func WithPoolSize(n int) Option { return func(c *config.Redis) { c.PoolSize = n } }

// WithMinIdleConns sets the minimum number of idle connections.
func WithMinIdleConns(n int) Option {
	return func(c *config.Redis) { c.MinIdleConns = n }
}

// WithMaxRetries sets the maximum number of command retries.
func WithMaxRetries(n int) Option { return func(c *config.Redis) { c.MaxRetries = n } }

// WithRetryBackoff sets the duration between retry attempts.
func WithRetryBackoff(d time.Duration) Option {
	return func(c *config.Redis) { c.RetryBackoff = d }
}

// WithTTL sets the default time-to-live for Redis entries.
func WithTTL(ttl time.Duration) Option {
	return func(c *config.Redis) { c.DefaultTTL = ttl }
}

// WithKeyPrefix sets a prefix prepended to all Redis keys.
func WithKeyPrefix(prefix string) Option {
	return func(c *config.Redis) { c.KeyPrefix = prefix }
}

// WithEnableMetrics enables or disables internal metrics collection.
func WithEnableMetrics(e bool) Option {
	return func(c *config.Redis) { c.EnableMetrics = e }
}

// WithEnablePipeline enables or disables Redis pipelining for batch operations.
func WithEnablePipeline(e bool) Option {
	return func(c *config.Redis) { c.EnablePipeline = e }
}

// WithDialTimeout sets the timeout for establishing new connections.
func WithDialTimeout(d time.Duration) Option { return func(c *config.Redis) { c.DialTimeout = d } }

// WithReadTimeout sets the timeout for reading from Redis.
func WithReadTimeout(d time.Duration) Option { return func(c *config.Redis) { c.ReadTimeout = d } }

// WithWriteTimeout sets the timeout for writing to Redis.
func WithWriteTimeout(d time.Duration) Option {
	return func(c *config.Redis) { c.WriteTimeout = d }
}

// WithTimeouts sets dial, read, and write timeouts at once.
func WithTimeouts(dial, read, write time.Duration) Option {
	return func(c *config.Redis) {
		c.DialTimeout = dial
		c.ReadTimeout = read
		c.WriteTimeout = write
	}
}

// WithDistributedStampedeProtection enables or disables distributed stampede protection.
func WithDistributedStampedeProtection(e bool) Option {
	return func(c *config.Redis) { c.EnableDistributedStampedeProtection = e }
}

// WithStampedeLockTTL sets the TTL for stampede protection locks.
func WithStampedeLockTTL(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeLockTTL = d }
}

// WithStampedeWaitTimeout sets the maximum wait time for a stampede lock.
func WithStampedeWaitTimeout(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeWaitTimeout = d }
}

// WithStampedeRetryInterval sets the polling interval while waiting for a stampede lock.
func WithStampedeRetryInterval(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeRetryInterval = d }
}

// WithConfig applies a pre-built Redis configuration (cloned to prevent mutation).
func WithConfig(cfg *config.Redis) Option {
	if cfg == nil {
		return func(*config.Redis) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Redis) { *c = *cloned }
}

// MergeOptions applies all options to a default Redis config and validates it.
func MergeOptions(opts ...Option) (*config.Redis, error) {
	cfg := config.DefaultRedis()
	for _, opt := range opts {
		opt(cfg)
	}
	if err := config.Apply(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// WithInterceptors sets the observability interceptors for the Redis cache.
func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Redis) { c.Interceptors = interceptors }
}
