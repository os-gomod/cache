package cache

import (
	"context"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/errors"
)

// Warmer pre-populates a cache backend by loading data from an external
// source. It supports batch loading and configurable concurrency for
// efficient bulk warming of cache entries.
//
// Warmer is designed for startup-time cache population, scheduled refresh,
// and recovery scenarios where a cache needs to be repopulated after
// a failure or deployment.
//
// Example:
//
//	warmer := cache.NewWarmer(backend, func(keys []string) (map[string][]byte, error) {
//	    // Load from database
//	    rows, _ := db.Query("SELECT key, value FROM cache_data WHERE key IN ?", keys)
//	    // ... return map[string][]byte
//	})
//	err := warmer.WarmAll(ctx, func() ([]string, error) {
//	    return []string{"key1", "key2", "key3"}, nil
//	})
type Warmer struct {
	backend Backend
	loader  func(keys []string) (map[string][]byte, error)
	config  *WarmerConfig
}

// NewWarmer creates a new cache warmer that loads data into the given
// backend using the provided loader function. The loader function receives
// a batch of keys and should return a map of key-value pairs for those keys
// that exist in the source.
//
// The loader function is called with batches of keys (controlled by
// WithWarmerBatchSize) and multiple batches may be loaded in parallel
// (controlled by WithWarmerConcurrency).
func NewWarmer(b Backend, loader func(keys []string) (map[string][]byte, error), opts ...WarmerOption) *Warmer {
	cfg := defaultWarmerConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Warmer{
		backend: b,
		loader:  loader,
		config:  cfg,
	}
}

// Warm loads the specified keys into the cache. Keys are processed in
// batches (controlled by WithWarmerBatchSize) and batches are loaded
// concurrently (controlled by WithWarmerConcurrency).
//
// Returns the first error encountered, but continues loading remaining
// keys. Individual key errors are reported via the OnError callback
// if configured.
func (w *Warmer) Warm(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	batchSize := w.config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	concurrency := w.config.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	batches := splitBatches(keys, batchSize)
	return w.processBatches(ctx, batches, concurrency)
}

// splitBatches splits keys into batches of the given size.
func splitBatches(keys []string, batchSize int) [][]string {
	var batches [][]string
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batches = append(batches, keys[i:end])
	}
	return batches
}

// processBatches processes key batches with bounded concurrency.
func (w *Warmer) processBatches(ctx context.Context, batches [][]string, concurrency int) error {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var firstErr error
	var mu sync.Mutex

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := w.loader(batch)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = errors.Factory.WarmFailed(batch[0], err)
				}
				mu.Unlock()
				if w.config.OnError != nil {
					for _, k := range batch {
						w.config.OnError(k, err)
					}
				}
				return
			}

			for k, v := range data {
				if setErr := w.backend.Set(ctx, k, v, time.Duration(0)); setErr != nil {
					if w.config.OnError != nil {
						w.config.OnError(k, setErr)
					}
				}
			}
		}(batch)
	}

	wg.Wait()
	return firstErr
}

// WarmAll loads all keys provided by the source function into the cache.
// The source function should return the complete list of keys to warm.
// This is useful for full cache repopulation scenarios.
//
// The keys are then processed by Warm in batches with bounded concurrency.
func (w *Warmer) WarmAll(ctx context.Context, source func() ([]string, error)) error {
	keys, err := source()
	if err != nil {
		return errors.Factory.WarmSourceFailed(err)
	}
	return w.Warm(ctx, keys...)
}
