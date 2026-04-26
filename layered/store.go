package layered

import (
	"context"
	"sync"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/lifecycle"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/internal/runtime"
	"github.com/os-gomod/cache/v2/internal/stats"
)

// Store is a two-tier cache that combines a fast in-memory L1 cache with a
// slower distributed L2 cache. Reads check L1 first, fall through to L2 on
// miss, and optionally promote L2 hits to L1. Writes go to both layers.
//
// Store is safe for concurrent use by multiple goroutines.
type Store struct {
	guard    lifecycle.Guard
	stats    *stats.Stats
	chain    *middleware.Chain
	executor *runtime.Executor
	l1       contracts.Cache // fast (e.g., memory)
	l2       contracts.Cache // slow (e.g., redis)
	cfg      config
	wbCh     chan writeBackJob
	wg       sync.WaitGroup
}

// New creates a new layered Store with the given options. Both L1 and L2
// backends must be provided and must respond to Ping. If write-back is
// enabled, worker goroutines are started to process the write-back queue.
func New(opts ...Option) (*Store, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.l1 == nil {
		return nil, errors.Factory.InvalidConfig("layered.new", "L1 backend is required")
	}
	if cfg.l2 == nil {
		return nil, errors.Factory.InvalidConfig("layered.new", "L2 backend is required")
	}

	// Verify both backends are alive
	ctx := context.Background()
	if err := cfg.l1.Ping(ctx); err != nil {
		return nil, errors.Factory.Connection("layered.new", err)
	}
	if err := cfg.l2.Ping(ctx); err != nil {
		return nil, errors.Factory.Connection("layered.new", err)
	}

	s := &Store{
		stats: stats.NewStats(),
		l1:    cfg.l1,
		l2:    cfg.l2,
		cfg:   cfg,
	}

	// Build middleware chain
	s.chain = middleware.BuildChain(cfg.interceptors...)

	// Create executor
	s.executor = runtime.New(runtime.WithChain(s.chain))

	// Start write-back workers if enabled
	if cfg.writeBack && cfg.wbWorkers > 0 {
		s.wbCh = make(chan writeBackJob, cfg.wbQueueSize)
		for range cfg.wbWorkers {
			s.wg.Add(1)
			go s.writeBackWorker()
		}
	}

	return s, nil
}

// Name returns the backend identifier: "layered".
func (*Store) Name() string { return "layered" }

// Close closes both L1 and L2 backends and stops write-back workers.
// It is safe to call Close multiple times.
func (s *Store) Close(ctx context.Context) error {
	if s.guard.Close() {
		return nil
	}

	// Stop write-back workers
	if s.wbCh != nil {
		close(s.wbCh)
		s.wg.Wait()
	}

	// Close L1 first (fast), then L2 (slow)
	l1Err := s.l1.Close(ctx)
	l2Err := s.l2.Close(ctx)

	if l1Err != nil {
		return l1Err
	}
	return l2Err
}

// Closed reports whether the store has been closed.
func (s *Store) Closed() bool {
	return s.guard.IsClosed()
}

// Ping checks whether both L1 and L2 backends are alive.
func (s *Store) Ping(ctx context.Context) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed("layered.ping")
	}
	if err := s.l1.Ping(ctx); err != nil {
		return err
	}
	return s.l2.Ping(ctx)
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

// checkClosed returns an error if the store is closed.
func (s *Store) checkClosed(op string) error {
	if s.guard.IsClosed() {
		return errors.Factory.Closed(op)
	}
	return nil
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
