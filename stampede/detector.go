// Package stampede provides enterprise-grade stampede protection for cache
// backends. It combines probabilistic early refresh (XFetch algorithm) with
// singleflight deduplication to prevent cache stampedes under high concurrency.
package stampede

import (
	"context"
	"sync"

	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
)

// DefaultBeta is the default aggressiveness parameter for the XFetch algorithm.
// A value of 1.0 provides standard early-refresh behavior; lower values make
// early refresh less aggressive, reducing background refresh load at the cost of
// slightly higher stampede risk near the soft-expiry boundary.
const DefaultBeta = 1.0

// refreshSlot is a sentinel value stored in the inflight map to indicate that
// a background refresh is already in progress for a given key.
type refreshSlot struct{}

// Detector provides stampede protection for cache read paths. It wraps a
// singleflight group with XFetch-based probabilistic early refresh: when a
// cached entry approaches its soft-expiry boundary, one goroutine is allowed
// to refresh the value in the background while all other callers continue to
// receive the existing (stale-but-valid) cached value immediately.
//
// All background refresh goroutines are tracked via an internal WaitGroup to
// prevent goroutine leaks. Callers should ensure Close is invoked when the
// detector is no longer needed to wait for any in-flight background refreshes.
type Detector struct {
	sg        *singlefght.Group
	beta      float64
	metrics   *observability.Chain
	wg        sync.WaitGroup
	inflight  sync.Map // key → *refreshSlot (tracks in-flight background refreshes)
	closeOnce sync.Once
	closed    chan struct{}
}

// NewDetector creates a new stampede detector with the given XFetch beta
// parameter and observability chain. If beta is <= 0, DefaultBeta is used.
// If obs is nil, a no-op chain is used.
func NewDetector(beta float64, obs *observability.Chain) *Detector {
	if beta <= 0 {
		beta = DefaultBeta
	}
	if obs == nil {
		obs = observability.NopChain()
	}
	return &Detector{
		sg:      singlefght.NewGroup(),
		beta:    beta,
		metrics: obs,
		closed:  make(chan struct{}),
	}
}

// Do executes fn with stampede protection. The behavior depends on the
// current entry state:
//
//   - If current is nil (cache miss) or entry is nil, fn is executed through
//     the singleflight group so that concurrent callers for the same key are
//     deduplicated.
//   - If the entry exists and ShouldEarlyRefresh returns true, one goroutine
//     is allowed to refresh in the background (tracked via inflight map). All
//     callers immediately receive the stale value. If the background refresh
//     fails, the stale value is kept until hard TTL expiry.
//   - If the entry exists but should not be early-refreshed, the current
//     cached value is returned directly.
//
// The onRefresh callback is invoked with the new value when a background
// refresh succeeds. Callers typically use this to write the refreshed value
// back to the cache.
func (d *Detector) Do(
	ctx context.Context,
	key string,
	current []byte,
	entry *eviction.Entry,
	fn func(context.Context) ([]byte, error),
	onRefresh func([]byte),
) ([]byte, error) {
	// Cache miss: go through singleflight to deduplicate.
	if current == nil || entry == nil {
		return d.sg.Do(ctx, key, func() ([]byte, error) {
			return fn(ctx)
		})
	}

	// Entry exists but does not need early refresh yet.
	if !entry.ShouldEarlyRefresh(d.beta) {
		return current, nil
	}

	// Early refresh is warranted. Try to claim the refresh slot so
	// only one goroutine refreshes for this key.
	slot := &refreshSlot{}
	if _, loaded := d.inflight.LoadOrStore(key, slot); loaded {
		// Another goroutine is already refreshing this key.
		// Return the stale value immediately.
		return current, nil
	}

	// We claimed the slot. Launch background refresh.
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.inflight.Delete(key)

		// Record early refresh metric.
		d.metrics.After(ctx, observability.Op{
			Backend: "stampede",
			Name:    "early_refresh",
			Key:     key,
		}, observability.Result{})

		val, err := fn(ctx)
		if err != nil {
			// Background refresh failed; stale value remains
			// until hard TTL. This is expected and safe.
			return
		}
		if onRefresh != nil {
			onRefresh(val)
		}
	}()

	// Return the stale value immediately while refresh happens in background.
	return current, nil
}

// Close waits for any in-flight background refresh goroutines to complete
// and then marks the detector as closed. After Close returns, no new
// background refreshes will be started. Close is idempotent: calling it
// multiple times is safe.
func (d *Detector) Close() {
	d.closeOnce.Do(func() {
		close(d.closed)
	})
	d.wg.Wait()
}
