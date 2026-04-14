package redis

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/observability"
)

type Option func(*config.Redis)

func WithAddress(addr string) Option { return func(c *config.Redis) { c.Addr = addr } }
func WithUsername(u string) Option   { return func(c *config.Redis) { c.Username = u } }
func WithPassword(p string) Option   { return func(c *config.Redis) { c.Password = p } }
func WithDB(db int) Option           { return func(c *config.Redis) { c.DB = db } }
func WithPoolSize(n int) Option      { return func(c *config.Redis) { c.PoolSize = n } }
func WithMinIdleConns(n int) Option {
	return func(c *config.Redis) { c.MinIdleConns = n }
}
func WithMaxRetries(n int) Option { return func(c *config.Redis) { c.MaxRetries = n } }
func WithRetryBackoff(d time.Duration) Option {
	return func(c *config.Redis) { c.RetryBackoff = d }
}

func WithTTL(ttl time.Duration) Option {
	return func(c *config.Redis) { c.DefaultTTL = ttl }
}

func WithKeyPrefix(prefix string) Option {
	return func(c *config.Redis) { c.KeyPrefix = prefix }
}

func WithEnableMetrics(e bool) Option {
	return func(c *config.Redis) { c.EnableMetrics = e }
}

func WithEnablePipeline(e bool) Option {
	return func(c *config.Redis) { c.EnablePipeline = e }
}
func WithDialTimeout(d time.Duration) Option { return func(c *config.Redis) { c.DialTimeout = d } }
func WithReadTimeout(d time.Duration) Option { return func(c *config.Redis) { c.ReadTimeout = d } }
func WithWriteTimeout(d time.Duration) Option {
	return func(c *config.Redis) { c.WriteTimeout = d }
}

func WithTimeouts(dial, read, write time.Duration) Option {
	return func(c *config.Redis) {
		c.DialTimeout = dial
		c.ReadTimeout = read
		c.WriteTimeout = write
	}
}

func WithDistributedStampedeProtection(e bool) Option {
	return func(c *config.Redis) { c.EnableDistributedStampedeProtection = e }
}

func WithStampedeLockTTL(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeLockTTL = d }
}

func WithStampedeWaitTimeout(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeWaitTimeout = d }
}

func WithStampedeRetryInterval(d time.Duration) Option {
	return func(c *config.Redis) { c.StampedeRetryInterval = d }
}

func WithConfig(cfg *config.Redis) Option {
	if cfg == nil {
		return func(*config.Redis) {}
	}
	cloned := cfg.Clone()
	return func(c *config.Redis) { *c = *cloned }
}

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

func WithInterceptors(interceptors ...observability.Interceptor) Option {
	return func(c *config.Redis) { c.Interceptors = interceptors }
}
