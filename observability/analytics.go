package observability

import (
	"math"
	"sort"
	"sync"
	"time"
)

// HitRateWindow tracks cache hit rates and latency percentiles over a
// sliding window of fixed-duration buckets. It uses a circular buffer
// so memory usage is bounded regardless of how long it runs.
type HitRateWindow struct {
	mu       sync.Mutex
	buckets  []hitBucket
	interval time.Duration
	size     int
	current  int

	latencyMu sync.Mutex
	latencies []time.Duration // raw latencies for percentile calc
	maxLat    int             // max latency samples to retain
}

type hitBucket struct {
	hits   int64
	misses int64
}

// NewHitRateWindow creates a sliding-window hit rate tracker.
// interval is the duration of each bucket (e.g., 1*time.Second).
// windowSize is the number of buckets in the sliding window (e.g., 60
// for a 1-minute window with 1-second intervals).
func NewHitRateWindow(interval time.Duration, windowSize int) *HitRateWindow {
	if interval <= 0 {
		interval = time.Second
	}
	if windowSize <= 0 {
		windowSize = 60
	}
	return &HitRateWindow{
		buckets:   make([]hitBucket, windowSize),
		interval:  interval,
		size:      windowSize,
		current:   0,
		latencies: make([]time.Duration, 0, 1024),
		maxLat:    10000,
	}
}

// Record records a hit or miss into the current time bucket.
func (w *HitRateWindow) Record(hit bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	bucket := &w.buckets[w.current%w.size]
	if hit {
		bucket.hits++
	} else {
		bucket.misses++
	}
}

// RecordLatency records a latency observation for percentile calculation.
func (w *HitRateWindow) RecordLatency(d time.Duration) {
	w.latencyMu.Lock()
	defer w.latencyMu.Unlock()

	w.latencies = append(w.latencies, d)
	if len(w.latencies) > w.maxLat {
		// Keep only the most recent samples.
		w.latencies = w.latencies[len(w.latencies)-w.maxLat:]
	}
}

// Advance moves the window forward by one interval. Call this periodically
// (e.g., from a ticker) to expire old buckets.
func (w *HitRateWindow) Advance() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.current++
	idx := w.current % w.size
	w.buckets[idx] = hitBucket{} // reset the new current bucket
}

// HitRate returns the aggregate hit rate over the last windowSize intervals.
// Returns 0.0 if there have been no operations.
func (w *HitRateWindow) HitRate() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	var totalHits, totalMisses int64
	for i := 0; i < w.size; i++ {
		totalHits += w.buckets[i].hits
		totalMisses += w.buckets[i].misses
	}
	total := totalHits + totalMisses
	if total == 0 {
		return 0.0
	}
	return float64(totalHits) / float64(total)
}

// P50Latency returns the median (p50) latency from the recorded samples.
// Returns 0 if no samples have been recorded.
func (w *HitRateWindow) P50Latency() time.Duration {
	return w.percentile(50)
}

// P99Latency returns the p99 latency from the recorded samples.
// Returns 0 if no samples have been recorded.
func (w *HitRateWindow) P99Latency() time.Duration {
	return w.percentile(99)
}

func (w *HitRateWindow) percentile(p int) time.Duration {
	w.latencyMu.Lock()
	samples := make([]time.Duration, len(w.latencies))
	copy(samples, w.latencies)
	w.latencyMu.Unlock()

	if len(samples) == 0 {
		return 0
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	idx := int(math.Ceil(float64(p)/100*float64(len(samples)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx]
}

// Reset clears all buckets and latency samples.
func (w *HitRateWindow) Reset() {
	w.mu.Lock()
	for i := range w.buckets {
		w.buckets[i] = hitBucket{}
	}
	w.current = 0
	w.mu.Unlock()

	w.latencyMu.Lock()
	w.latencies = w.latencies[:0]
	w.latencyMu.Unlock()
}
