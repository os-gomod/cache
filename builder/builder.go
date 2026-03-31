package builder

import (
	"context"
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/layered"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
)

// Builder is the root entry point for constructing cache instances.
type Builder struct {
	ctx context.Context
}

// New returns a new Builder.  If ctx is nil, context.Background() is used.
func New(ctx context.Context) *Builder {
	return &Builder{ctx: cachectx.NormalizeContext(ctx)}
}

// Memory returns a MemoryBuilder using this Builder's context.
func (b *Builder) Memory() *MemoryBuilder { return NewMemoryBuilder(b.ctx) }

// Redis returns a RedisBuilder using this Builder's context.
func (b *Builder) Redis() *RedisBuilder { return NewRedisBuilder(b.ctx) }

// Layered returns a LayeredBuilder using this Builder's context.
func (b *Builder) Layered() *LayeredBuilder { return NewLayeredBuilder(b.ctx) }

// ----------------------------------------------------------------------------
// Memory convenience functions
// ----------------------------------------------------------------------------

// DefaultMemory creates a memory cache with default configuration.
func DefaultMemory(ctx context.Context) (*memory.Cache, error) {
	return New(ctx).Memory().Build()
}

// MemoryWithSize creates a memory cache capped at maxMB megabytes.
func MemoryWithSize(ctx context.Context, maxMB int) (*memory.Cache, error) {
	return New(ctx).Memory().MaxMemoryMB(maxMB).Build()
}

// MemoryWithTTL creates a memory cache with the given default TTL.
func MemoryWithTTL(ctx context.Context, ttl time.Duration) (*memory.Cache, error) {
	return New(ctx).Memory().TTL(ttl).Build()
}

// ----------------------------------------------------------------------------
// Redis convenience functions
// ----------------------------------------------------------------------------

// DefaultRedis creates a Redis cache with default configuration.
func DefaultRedis(ctx context.Context) (*redis.Cache, error) {
	return New(ctx).Redis().Build()
}

// RedisWithAddress creates a Redis cache pointed at addr.
func RedisWithAddress(ctx context.Context, addr string) (*redis.Cache, error) {
	return New(ctx).Redis().Addr(addr).Build()
}

// RedisWithConfig creates a Redis cache from an explicit config.
func RedisWithConfig(ctx context.Context, cfg *config.Redis) (*redis.Cache, error) {
	return New(ctx).Redis().WithConfig(cfg).Build()
}

// ----------------------------------------------------------------------------
// Layered convenience functions
// ----------------------------------------------------------------------------

// DefaultLayered creates a layered cache with default configuration.
func DefaultLayered(ctx context.Context) (*layered.Cache, error) {
	return New(ctx).Layered().Build()
}

// LayeredWithRedis creates a layered cache with L2 pointed at addr.
func LayeredWithRedis(ctx context.Context, addr string) (*layered.Cache, error) {
	return New(ctx).Layered().L2Addr(addr).Build()
}

// LayeredWithConfig creates a layered cache from an explicit config.
func LayeredWithConfig(ctx context.Context, cfg *config.Layered) (*layered.Cache, error) {
	return New(ctx).Layered().WithConfig(cfg).Build()
}
