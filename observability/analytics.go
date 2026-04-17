package observability

import (
	"math"
	"sort"
	"sync"
	"time"
)

type HitRateWindow struct {
	mu        sync.Mutex
	buckets   []hitBucket
	interval  time.Duration
	size      int
	current   int
	latencyMu sync.Mutex
	latencies []time.Duration
	maxLat    int
}
type hitBucket struct {
	hits   int64
	misses int64
}

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

func (w *HitRateWindow) RecordLatency(d time.Duration) {
	w.latencyMu.Lock()
	defer w.latencyMu.Unlock()
	w.latencies = append(w.latencies, d)
	if len(w.latencies) > w.maxLat {
		w.latencies = w.latencies[len(w.latencies)-w.maxLat:]
	}
}

func (w *HitRateWindow) Advance() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current++
	idx := w.current % w.size
	w.buckets[idx] = hitBucket{}
}

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

func (w *HitRateWindow) P50Latency() time.Duration {
	return w.percentile(50)
}

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
