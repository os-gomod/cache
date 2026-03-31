package builder

import (
	"context"
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/redis"
)

// RedisBuilder builds a Redis cache with a fluent, immutable API.
type RedisBuilder struct {
	*GenericBuilder[config.Redis, *redis.Cache]
}

// NewRedisBuilder creates a Redis cache builder.
func NewRedisBuilder(ctx context.Context) *RedisBuilder {
	return &RedisBuilder{
		GenericBuilder: newValidatedBuilder(
			ctx,
			config.DefaultRedis,
			redis.NewWithConfigContext,
			func(cfg *config.Redis) *config.Redis { return cfg.Clone() },
		),
	}
}

// WithConfig applies a full Redis config.
func (b *RedisBuilder) WithConfig(cfg *config.Redis) *RedisBuilder {
	return b.wrapOption(redis.WithConfig(cfg))
}

// WithOption applies a redis.Option.
func (b *RedisBuilder) WithOption(opt redis.Option) *RedisBuilder {
	return b.wrapOption(opt)
}

func (b *RedisBuilder) wrapOption(opt redis.Option) *RedisBuilder {
	return &RedisBuilder{
		GenericBuilder: b.GenericBuilder.WithOption(func(cfg *config.Redis) { opt(cfg) }),
	}
}

func (b *RedisBuilder) Addr(
	addr string,
) *RedisBuilder {
	return b.wrapOption(redis.WithAddress(addr))
}

func (b *RedisBuilder) Username(
	u string,
) *RedisBuilder {
	return b.wrapOption(redis.WithUsername(u))
}

func (b *RedisBuilder) Password(
	p string,
) *RedisBuilder {
	return b.wrapOption(redis.WithPassword(p))
}
func (b *RedisBuilder) DB(db int) *RedisBuilder { return b.wrapOption(redis.WithDB(db)) }

func (b *RedisBuilder) PoolSize(
	n int,
) *RedisBuilder {
	return b.wrapOption(redis.WithPoolSize(n))
}

func (b *RedisBuilder) MinIdleConns(n int) *RedisBuilder {
	return b.wrapOption(redis.WithMinIdleConns(n))
}

func (b *RedisBuilder) MaxRetries(n int) *RedisBuilder {
	return b.wrapOption(redis.WithMaxRetries(n))
}

func (b *RedisBuilder) RetryBackoff(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithRetryBackoff(d))
}

func (b *RedisBuilder) DialTimeout(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithDialTimeout(d))
}

func (b *RedisBuilder) ReadTimeout(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithReadTimeout(d))
}

func (b *RedisBuilder) WriteTimeout(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithWriteTimeout(d))
}

func (b *RedisBuilder) Timeouts(dial, read, write time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithTimeouts(dial, read, write))
}

func (b *RedisBuilder) TTL(ttl time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithTTL(ttl))
}

func (b *RedisBuilder) KeyPrefix(prefix string) *RedisBuilder {
	return b.wrapOption(redis.WithKeyPrefix(prefix))
}

func (b *RedisBuilder) EnablePipeline(enabled bool) *RedisBuilder {
	return b.wrapOption(redis.WithEnablePipeline(enabled))
}

func (b *RedisBuilder) EnableMetrics(enabled bool) *RedisBuilder {
	return b.wrapOption(redis.WithEnableMetrics(enabled))
}

func (b *RedisBuilder) DistributedStampedeProtection(enabled bool) *RedisBuilder {
	return b.wrapOption(redis.WithDistributedStampedeProtection(enabled))
}

func (b *RedisBuilder) StampedeLockTTL(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithStampedeLockTTL(d))
}

func (b *RedisBuilder) StampedeWaitTimeout(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithStampedeWaitTimeout(d))
}

func (b *RedisBuilder) StampedeRetryInterval(d time.Duration) *RedisBuilder {
	return b.wrapOption(redis.WithStampedeRetryInterval(d))
}

// MustBuild calls Build and panics on error.
func (b *RedisBuilder) MustBuild() *redis.Cache { return b.GenericBuilder.MustBuild() }
