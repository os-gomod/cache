package manager

import (
	"context"

	"github.com/os-gomod/cache/layer"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
)

// NewMemoryManager creates a CacheManager with an in-memory backend and default resilience policy.
func NewMemoryManager(opts ...memory.Option) (*CacheManager, error) {
	mc, err := memory.New(opts...)
	if err != nil {
		return nil, err
	}
	return New(
		WithDefaultBackend(mc),
		WithPolicy(resilience.DefaultPolicy()),
	)
}

// NewRedisManager creates a CacheManager with a Redis backend and default resilience policy.
func NewRedisManager(opts ...redis.Option) (*CacheManager, error) {
	rc, err := redis.New(opts...)
	if err != nil {
		return nil, err
	}
	return New(
		WithDefaultBackend(rc),
		WithPolicy(resilience.DefaultPolicy()),
	)
}

// NewLayeredManager creates a CacheManager with a layered (L1+L2) backend and default resilience policy.
func NewLayeredManager(
	l1Opts []memory.Option,
	l2Opts []redis.Option,
	layeredOpts []layer.Option,
) (*CacheManager, error) {
	l1, err := memory.New(l1Opts...)
	if err != nil {
		return nil, err
	}
	l2, err := redis.New(l2Opts...)
	if err != nil {
		_ = l1.Close(context.Background())
		return nil, err
	}
	lc, err := layer.NewWithBackends(context.Background(), l1, l2, layeredOpts...)
	if err != nil {
		_ = l1.Close(context.Background())
		_ = l2.Close(context.Background())
		return nil, err
	}
	return New(
		WithDefaultBackend(lc),
		WithPolicy(resilience.DefaultPolicy()),
	)
}
