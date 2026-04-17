// Package stampede provides stampede (thundering herd) protection for cache
// backends using early refresh and singleflight deduplication.
package stampede

import (
	"context"
	"sync"

	"github.com/os-gomod/cache/internal/singlefght"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
)

// DefaultBeta is the default early-refresh threshold factor.
// A value of 1.0 triggers refresh when an entry's age exceeds half its TTL.
const DefaultBeta = 1.0

type refreshSlot struct{}

// Detector prevents cache stampede by performing early refresh of entries
// approaching expiration. At most one refresh runs per key at a time.
type Detector struct {
	sg        *singlefght.Group
	beta      float64
	metrics   *observability.Chain
	wg        sync.WaitGroup
	inflight  sync.Map
	closeOnce sync.Once
	closed    chan struct{}
}

// NewDetector creates a new Detector with the given early-refresh threshold
// and observability chain. Beta values of 0 or less are normalized to DefaultBeta.
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

// Do returns the current value for key if it is not due for early refresh.
// If the entry should be refreshed, it triggers an asynchronous refresh
// via fn and immediately returns the current (stale) value.
func (d *Detector) Do(
	ctx context.Context,
	key string,
	current []byte,
	entry *eviction.Entry,
	fn func(context.Context) ([]byte, error),
	onRefresh func([]byte),
) ([]byte, error) {
	if current == nil || entry == nil {
		return d.sg.Do(ctx, key, func() ([]byte, error) {
			return fn(ctx)
		})
	}
	if !entry.ShouldEarlyRefresh(d.beta) {
		return current, nil
	}
	slot := &refreshSlot{}
	if _, loaded := d.inflight.LoadOrStore(key, slot); loaded {
		return current, nil
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.inflight.Delete(key)
		d.metrics.After(ctx, observability.Op{
			Backend: "stampede",
			Name:    "early_refresh",
			Key:     key,
		}, observability.Result{})
		val, err := fn(ctx)
		if err != nil {
			return
		}
		if onRefresh != nil {
			onRefresh(val)
		}
	}()
	return current, nil
}

// Close waits for all in-flight refresh operations to complete.
func (d *Detector) Close() {
	d.closeOnce.Do(func() {
		close(d.closed)
	})
	d.wg.Wait()
}
