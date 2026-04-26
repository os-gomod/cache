// Package cache provides a unified, high-performance caching platform
// with pluggable backends, middleware, and enterprise-grade observability.
//
// # Quick Start
//
// Create a memory cache:
//
//	c, err := cache.NewMemory(WithMaxEntries(10000))
//	if err != nil { /* handle error */ }
//	defer c.Close(context.Background())
//
// Create a typed cache:
//
//	tc, err := cache.NewMemoryString()
//	if err != nil { /* handle error */ }
//	tc.Set(ctx, "greeting", "hello", time.Minute)
//
// # Backends
//
// The package ships with three backend implementations:
//   - Memory: in-process sharded cache with pluggable eviction
//   - Redis: distributed cache backed by Redis
//   - Layered: two-tier (L1 memory + L2 Redis) with promotion
//
// # Middleware
//
// Middleware wraps a backend to add cross-cutting concerns:
//
//	wrapped := cache.WithMiddleware(c,
//	    middleware.RetryMiddleware(cfg),
//	    middleware.CircuitBreakerMiddleware(cfg),
//	)
//
// # Enterprise Extensions
//
// Additional enterprise-grade features are available in this package:
//   - Warmer: pre-populate caches from a data source
//   - HotKeyDetector: detect and react to hot keys
//   - AdaptiveTTL: dynamically adjust TTL based on access patterns
//   - Compression: transparently compress/decompress values
package cache

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/layered"
	"github.com/os-gomod/cache/v2/memory"
	"github.com/os-gomod/cache/v2/redis"
)

// Backend is the primary cache interface exposed to consumers. It
// composes all capability interfaces into a single type, providing
// read, write, atomic, scan, lifecycle, and stats operations.
//
// All backend implementations returned by the constructors in this
// package satisfy this interface.
type Backend interface {
	contracts.Reader
	contracts.Writer
	contracts.AtomicOps
	contracts.Scanner
	contracts.Lifecycle
	contracts.StatsProvider
}

// NewMemory creates a new in-process memory cache with the given options.
// The memory backend uses sharding for high concurrent throughput and
// supports pluggable eviction policies (LRU, LFU, FIFO, TinyLFU).
//
// Example:
//
//	c, err := cache.NewMemory(WithMaxEntries(10000), WithDefaultTTL(10*time.Minute))
//	if err != nil { /* handle error */ }
//	defer c.Close(context.Background())
func NewMemory(opts ...memory.Option) (Backend, error) {
	s, err := memory.New(opts...)
	if err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return nil, err
	}
	return s, nil
}

// NewRedis creates a new Redis-backed distributed cache with the given
// options. Requires a running Redis instance.
//
// Example:
//
//	c, err := cache.NewRedis(redis.WithAddr("localhost:6379"), redis.WithPassword("secret"))
//	if err != nil { /* handle error */ }
//	defer c.Close(context.Background())
func NewRedis(opts ...redis.Option) (Backend, error) {
	s, err := redis.New(opts...)
	if err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return nil, err
	}
	return s, nil
}

// NewLayered creates a new two-tier layered cache with L1 (memory) and
// L2 (Redis) tiers. Reads check L1 first, falling through to L2 with
// automatic promotion. Writes propagate to both tiers.
//
// Example:
//
//	c, err := cache.NewLayered(
//	    layered.WithL1(memory.WithMaxEntries(1000)),
//	    layered.WithL2(redis.WithAddr("localhost:6379")),
//	)
//	if err != nil { /* handle error */ }
//	defer c.Close(context.Background())
func NewLayered(opts ...layered.Option) (Backend, error) {
	s, err := layered.New(opts...)
	if err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return nil, err
	}
	return s, nil
}

// WithMiddleware wraps a backend with the given middleware chain. Middleware
// is applied in the order provided (first middleware is the outermost wrapper).
// Each middleware can intercept, modify, or short-circuit cache operations.
//
// Example:
//
//	wrapped := cache.WithMiddleware(c,
//	    middleware.MetricsMiddleware(recorder),
//	    middleware.RetryMiddleware(retryCfg),
//	)
func WithMiddleware(b Backend, middlewares ...middleware.Middleware) Backend {
	chain := middleware.NewChain(middlewares...)
	return &middlewareBackend{backend: b, chain: chain}
}

// WithResilience applies a standard resilience stack to the backend:
// retry, circuit breaker, and rate limiter. Options can be used to
// configure each component. If no options are provided, sensible defaults
// are used for all three resilience components.
//
// Example:
//
//	wrapped := cache.WithResilience(c,
//	    WithRetry(3, 100*time.Millisecond),
//	    WithCircuitBreaker(5, 30*time.Second),
//	    WithRateLimit(1000, 100),
//	)
func WithResilience(b Backend, opts ...ResilienceOption) Backend {
	cfg := &resilienceConfig{
		maxAttempts:    3,
		initialDelay:   100 * time.Millisecond,
		cbThreshold:    5,
		cbTimeout:      30 * time.Second,
		rateLimitRate:  1000,
		rateLimitBurst: 100,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return WithMiddleware(b,
		middleware.RetryMiddleware(middleware.RetryConfig{
			MaxAttempts:  cfg.maxAttempts,
			InitialDelay: cfg.initialDelay,
		}),
		middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
			Threshold: cfg.cbThreshold,
			Timeout:   cfg.cbTimeout,
		}),
		middleware.RateLimiterMiddleware(middleware.RateLimiterConfig{
			Rate:  cfg.rateLimitRate,
			Burst: cfg.rateLimitBurst,
		}),
	)
}

// resilienceConfig holds the configuration for the resilience middleware stack.
type resilienceConfig struct {
	maxAttempts    int
	initialDelay   time.Duration
	cbThreshold    int
	cbTimeout      time.Duration
	rateLimitRate  float64
	rateLimitBurst int
}

// ResilienceOption is a functional option for configuring resilience middleware.
type ResilienceOption func(*resilienceConfig)

// WithRetry configures the retry middleware with the given maximum number
// of attempts and initial backoff delay. The delay doubles after each attempt
// (exponential backoff with jitter).
func WithRetry(maxAttempts int, initialDelay time.Duration) ResilienceOption {
	return func(c *resilienceConfig) {
		c.maxAttempts = maxAttempts
		c.initialDelay = initialDelay
	}
}

// WithCircuitBreaker configures the circuit breaker middleware with the given
// failure threshold and reset timeout. The circuit opens after threshold
// consecutive failures and closes after timeout.
func WithCircuitBreaker(threshold int, timeout time.Duration) ResilienceOption {
	return func(c *resilienceConfig) {
		c.cbThreshold = threshold
		c.cbTimeout = timeout
	}
}

// WithRateLimit configures the rate limiter middleware with the given rate
// (operations per second) and burst size. Requests exceeding the rate are
// rejected with a rate-limit error.
func WithRateLimit(rate float64, burst int) ResilienceOption {
	return func(c *resilienceConfig) {
		c.rateLimitRate = rate
		c.rateLimitBurst = burst
	}
}

// middlewareBackend wraps a Backend with middleware chain hooks.
type middlewareBackend struct {
	backend Backend
	chain   *middleware.Chain
}

func (w *middlewareBackend) Get(ctx context.Context, key string) ([]byte, error) {
	op := contracts.Operation{Name: "get", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	val, err := w.backend.Get(ctx, key)
	result := contracts.Result{Err: err, Value: val}
	if val != nil {
		result.ByteSize = len(val)
	}
	w.chain.After(ctx, op, result)
	return val, err
}

func (w *middlewareBackend) Set(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) error {
	op := contracts.Operation{Name: "set", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	err := w.backend.Set(ctx, key, value, ttl)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return err
}

func (w *middlewareBackend) Delete(ctx context.Context, key string) error {
	op := contracts.Operation{Name: "delete", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	err := w.backend.Delete(ctx, key)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return err
}

func (w *middlewareBackend) GetMulti(
	ctx context.Context,
	keys ...string,
) (map[string][]byte, error) {
	op := contracts.Operation{Name: "getmulti", KeyCount: len(keys), Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.GetMulti(ctx, keys...)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

func (w *middlewareBackend) SetMulti(
	ctx context.Context,
	items map[string][]byte,
	ttl time.Duration,
) error {
	op := contracts.Operation{Name: "setmulti", KeyCount: len(items), Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	err := w.backend.SetMulti(ctx, items, ttl)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return err
}

func (w *middlewareBackend) DeleteMulti(ctx context.Context, keys ...string) error {
	op := contracts.Operation{Name: "deletemulti", KeyCount: len(keys), Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	err := w.backend.DeleteMulti(ctx, keys...)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return err
}

//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) Exists(ctx context.Context, key string) (bool, error) {
	op := contracts.Operation{Name: "exists", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.Exists(ctx, key)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	op := contracts.Operation{Name: "ttl", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.TTL(ctx, key)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

func (w *middlewareBackend) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	op := contracts.Operation{Name: "cas", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.CompareAndSwap(ctx, key, oldVal, newVal, ttl)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

//
//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	op := contracts.Operation{Name: "setnx", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.SetNX(ctx, key, value, ttl)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	op := contracts.Operation{Name: "incr", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.Increment(ctx, key, delta)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	op := contracts.Operation{Name: "decr", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.Decrement(ctx, key, delta)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

//
//nolint:dupl // structural similarity is intentional
func (w *middlewareBackend) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	op := contracts.Operation{Name: "getset", Key: key, Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.GetSet(ctx, key, value, ttl)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

func (w *middlewareBackend) Keys(ctx context.Context, pattern string) ([]string, error) {
	op := contracts.Operation{Name: "keys", Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.Keys(ctx, pattern)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

func (w *middlewareBackend) Clear(ctx context.Context) error {
	op := contracts.Operation{Name: "clear", Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	err := w.backend.Clear(ctx)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return err
}

func (w *middlewareBackend) Size(ctx context.Context) (int64, error) {
	op := contracts.Operation{Name: "size", Backend: w.backend.Name()}
	ctx = w.chain.Before(ctx, op)
	result, err := w.backend.Size(ctx)
	w.chain.After(ctx, op, contracts.Result{Err: err})
	return result, err
}

// Lifecycle and Stats methods delegate directly.
func (w *middlewareBackend) Ping(ctx context.Context) error  { return w.backend.Ping(ctx) }
func (w *middlewareBackend) Close(ctx context.Context) error { return w.backend.Close(ctx) }
func (w *middlewareBackend) Closed() bool                    { return w.backend.Closed() }
func (w *middlewareBackend) Name() string                    { return w.backend.Name() }
func (w *middlewareBackend) Stats() contracts.StatsSnapshot  { return w.backend.Stats() }

// Ensure context is used so the import is not stripped.
var _ context.Context = context.Background()
