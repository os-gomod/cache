package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/keyutil"
	"github.com/os-gomod/cache/v2/internal/lifecycle"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/internal/runtime"
	"github.com/os-gomod/cache/v2/internal/stats"
)

// Store is a Redis-backed cache implementation. It uses go-redis/v9 for
// connection pooling and command execution, with the unified runtime.Executor
// for operation execution and middleware hooks.
//
// Store is safe for concurrent use by multiple goroutines.
type Store struct {
	guard    lifecycle.Guard
	stats    *stats.Stats
	chain    *middleware.Chain
	executor *runtime.Executor
	client   redis.UniversalClient
	cfg      config
}

// New creates a new Redis cache Store with the given options. It establishes
// a connection to Redis and verifies connectivity with a PING command.
func New(opts ...Option) (*Store, error) {
	var cfg config
	cfg.apply(opts...)

	// Build the Redis client
	var rdb redis.UniversalClient

	if len(cfg.addresses) == 1 {
		// Single node
		rdb = redis.NewClient(&redis.Options{
			Addr:         cfg.addresses[0],
			Password:     cfg.password,
			DB:           cfg.db,
			PoolSize:     cfg.poolSize,
			DialTimeout:  cfg.dialTimeout,
			ReadTimeout:  cfg.readTimeout,
			WriteTimeout: cfg.writeTimeout,
		})
	} else {
		// Cluster / multi-node
		rdb = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:        cfg.addresses,
			Password:     cfg.password,
			PoolSize:     cfg.poolSize,
			DialTimeout:  cfg.dialTimeout,
			ReadTimeout:  cfg.readTimeout,
			WriteTimeout: cfg.writeTimeout,
		})
	}

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), cfg.dialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, errors.Factory.Connection("redis.new", err)
	}

	s := &Store{
		stats:  stats.NewStats(),
		client: rdb,
		cfg:    cfg,
	}

	// Build middleware chain
	s.chain = middleware.BuildChain(cfg.interceptors...)

	// Create executor
	s.executor = runtime.New(runtime.WithChain(s.chain))

	return s, nil
}

// Name returns the backend identifier: "redis".
func (*Store) Name() string { return "redis" }

// Close closes the Redis connection pool and marks the store as closed.
// It is safe to call Close multiple times.
func (s *Store) Close(_ context.Context) error {
	if s.guard.Close() {
		return nil
	}
	return s.client.Close()
}

// Closed reports whether the store has been closed.
func (s *Store) Closed() bool {
	return s.guard.IsClosed()
}

// Ping checks whether the Redis connection is alive. Returns an error if the
// store is closed or Redis is unreachable.
func (s *Store) Ping(ctx context.Context) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed("redis.ping")
	}
	if err := s.client.Ping(ctx).Err(); err != nil {
		return errors.Factory.Connection("redis.ping", err)
	}
	return nil
}

// Stats returns an immutable snapshot of the store's current statistics.
func (s *Store) Stats() contracts.StatsSnapshot {
	ss := s.stats.Snapshot()
	return contracts.StatsSnapshot{
		Hits:      ss.Hits,
		Misses:    ss.Misses,
		Sets:      ss.Sets,
		Deletes:   ss.Deletes,
		Evictions: ss.Evictions,
		Errors:    ss.Errors,
		Items:     ss.Items,
		StartTime: s.stats.StartTime(),
	}
}

// Client returns the underlying go-redis client for advanced operations
// not covered by the Cache interface (e.g., transactions, pub/sub).
func (s *Store) Client() redis.UniversalClient {
	return s.client
}

// buildKey prepends the configured key prefix to the raw key.
func (s *Store) buildKey(key string) string {
	return keyutil.BuildKey(s.cfg.keyPrefix, key)
}

// checkClosed returns an error if the store is closed.
func (s *Store) checkClosed(op string) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed(op)
	}
	return nil
}

// resolveTTL returns the effective TTL. If ttl <= 0, the default TTL is used.
func (s *Store) resolveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return s.cfg.defaultTTL
}

// Ensure Store implements contracts.Cache at compile time.
var (
	_ contracts.Reader        = (*Store)(nil)
	_ contracts.Writer        = (*Store)(nil)
	_ contracts.AtomicOps     = (*Store)(nil)
	_ contracts.Scanner       = (*Store)(nil)
	_ contracts.Lifecycle     = (*Store)(nil)
	_ contracts.StatsProvider = (*Store)(nil)
)
