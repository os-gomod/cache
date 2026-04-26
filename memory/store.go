package memory

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/hash"
	"github.com/os-gomod/cache/v2/internal/keyutil"
	"github.com/os-gomod/cache/v2/internal/lifecycle"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/internal/runtime"
	"github.com/os-gomod/cache/v2/internal/stats"
)

// Store is a multi-shard, in-memory cache backend. It provides O(1) access
// via FNV-1a consistent hashing across shards, TTL-based expiration with
// background janitor cleanup, and memory-aware eviction.
//
// Store is safe for concurrent use by multiple goroutines.
type Store struct {
	guard     lifecycle.Guard
	stats     *stats.Stats
	chain     *middleware.Chain
	executor  *runtime.Executor
	shards    []*shard
	shardMask uint32
	cfg       config
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// New creates a new in-memory cache Store with the given options.
// Returns an error if the configuration is invalid.
func New(opts ...Option) (*Store, error) {
	var cfg config
	if err := cfg.apply(opts...); err != nil {
		return nil, errors.Factory.InvalidConfig("memory.new", err.Error())
	}

	// Normalize shard count to a power of two
	nShards := hash.NormalizeShards(cfg.shardCount)
	perShardEntries := cfg.maxEntries / nShards
	if perShardEntries <= 0 {
		perShardEntries = 64
	}

	// Create shards
	shards := make([]*shard, nShards)
	for i := range shards {
		shards[i] = newShard(perShardEntries)
	}

	s := &Store{
		stats:     stats.NewStats(),
		shards:    shards,
		shardMask: hash.BitmaskForPowerOfTwo(nShards),
		cfg:       cfg,
		stopCh:    make(chan struct{}),
	}

	// Build middleware chain
	s.chain = middleware.BuildChain(cfg.interceptors...)

	// Create executor
	s.executor = runtime.New(runtime.WithChain(s.chain))

	// Start background janitor if cleanup interval is set
	if cfg.cleanupInterval > 0 {
		s.wg.Add(1)
		go s.janitor()
	}

	return s, nil
}

// Name returns the backend identifier: "memory".
func (*Store) Name() string { return "memory" }

// Close stops the background janitor goroutine and marks the store as closed.
// It is safe to call Close multiple times; subsequent calls are no-ops.
func (s *Store) Close(_ context.Context) error {
	if s.guard.Close() {
		// Already closed
		return nil
	}
	close(s.stopCh)
	s.wg.Wait()
	return nil
}

// Closed reports whether the store has been closed.
func (s *Store) Closed() bool {
	return s.guard.IsClosed()
}

// Ping checks whether the memory store is alive. Returns an error if the
// store has been closed.
func (s *Store) Ping(_ context.Context) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed("memory.ping")
	}
	return nil
}

// Stats returns an immutable snapshot of the store's current statistics.
func (s *Store) Stats() contracts.StatsSnapshot {
	ss := s.stats.Snapshot()
	return contracts.StatsSnapshot{
		Hits:        ss.Hits,
		Misses:      ss.Misses,
		Sets:        ss.Sets,
		Deletes:     ss.Deletes,
		Evictions:   ss.Evictions,
		Errors:      ss.Errors,
		Items:       ss.Items,
		MemoryBytes: ss.MemoryBytes,
		MaxMemory:   s.cfg.maxMemoryBytes,
		StartTime:   s.stats.StartTime(),
	}
}

// shardFor returns the shard responsible for the given key.
func (s *Store) shardFor(key string) *shard {
	idx := hash.FNV1a32(key) & s.shardMask
	return s.shards[idx]
}

// checkClosed returns an error if the store is closed, using the given
// operation name for error context.
func (s *Store) checkClosed(op string) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed(op)
	}
	return nil
}

// validateKey checks the key and returns an error if invalid.
func (*Store) validateKey(op, key string) error {
	//nolint:wrapcheck // ValidateKey already returns a descriptive error
	return keyutil.ValidateKey(op, key)
}

// resolveTTL returns the effective TTL for an entry. If ttl is zero or
// negative, the default TTL from the config is used. If the default is
// also zero, the entry has no expiration.
func (s *Store) resolveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return s.cfg.defaultTTL
}
