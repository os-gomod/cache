package cache

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// HotKeyDetector identifies frequently accessed cache keys that may
// indicate hot-spot patterns, thundering herd risks, or cache-stampede
// scenarios. Once detected, consumers can take corrective action such
// as extending TTL, using request coalescing, or proactively refreshing.
//
// Detection uses a sliding-window counter per key. When a key's access
// count within the window exceeds the configured threshold, it is
// flagged as "hot" and the optional callback is invoked.
//
// HotKeyDetector is safe for concurrent use.
//
// Example:
//
//	detector := cache.NewHotKeyDetector(
//	    cache.WithHotKeyThreshold(100),
//	    cache.WithHotKeyCallback(func(key string, count int64) {
//	        log.Printf("hot key detected: %s (count: %d)", key, count)
//	    }),
//	)
//
//	// In your cache middleware or wrapper:
//	detector.Record(key)
//	if detector.IsHot(key) {
//	    // Enable request coalescing or extend TTL
//	}
type HotKeyDetector struct {
	counters map[string]*counter
	mu       sync.RWMutex
	config   *HotKeyConfig
	topN     []KeyCount // cached top-N results
	topNMu   sync.RWMutex
}

// counter tracks access counts for a single key using atomic operations.
type counter struct {
	count  int64
	window int64 // Unix nanoseconds of window start
}

// KeyCount represents a key and its access count.
type KeyCount struct {
	Key   string
	Count int64
}

// NewHotKeyDetector creates a new hot-key detector with the given options.
// The detector starts with default settings if no options are provided:
//   - Threshold: 100 accesses per window
//   - Window: 1 second
//   - MaxKeys: 10000 tracked keys
func NewHotKeyDetector(opts ...HotKeyOption) *HotKeyDetector {
	cfg := defaultHotKeyConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &HotKeyDetector{
		counters: make(map[string]*counter),
		config:   cfg,
	}
}

// Record increments the access count for the given key. If the key's
// count exceeds the threshold, the OnHotKey callback is invoked (if
// configured). Record is safe for concurrent use and has minimal
// overhead (single atomic operation in the fast path).
func (d *HotKeyDetector) Record(key string) {
	now := time.Now().UnixNano()

	// Fast path: try read lock first.
	d.mu.RLock()
	c, ok := d.counters[key]
	d.mu.RUnlock()

	if !ok {
		// Slow path: acquire write lock to create counter.
		d.mu.Lock()
		c, ok = d.counters[key]
		if !ok {
			// Enforce MaxKeys limit.
			if d.config.MaxKeys > 0 && len(d.counters) >= d.config.MaxKeys {
				// Evict the key with the lowest count.
				d.evictOldest()
			}
			c = &counter{
				window: now,
			}
			d.counters[key] = c
		}
		d.mu.Unlock()
	}

	// Check if window has expired and reset if needed.
	windowDuration := d.config.WindowDuration
	if windowDuration <= 0 {
		windowDuration = time.Second
	}
	windowNanos := windowDuration.Nanoseconds()

	oldWindow := atomic.LoadInt64(&c.window)
	if now-oldWindow > windowNanos {
		// Reset counter for new window. Use CAS to avoid contention.
		if atomic.CompareAndSwapInt64(&c.window, oldWindow, now) {
			atomic.StoreInt64(&c.count, 1)
		} else {
			atomic.AddInt64(&c.count, 1)
		}
	} else {
		count := atomic.AddInt64(&c.count, 1)

		// Check threshold.
		if d.config.Threshold > 0 && count == d.config.Threshold+1 {
			if d.config.OnHotKey != nil {
				d.config.OnHotKey(key, count)
			}
		}
	}
}

// IsHot returns true if the given key's access count exceeds the
// configured threshold within the current window.
func (d *HotKeyDetector) IsHot(key string) bool {
	d.mu.RLock()
	c, ok := d.counters[key]
	d.mu.RUnlock()

	if !ok {
		return false
	}

	count := atomic.LoadInt64(&c.count)
	return count >= d.config.Threshold
}

// Count returns the current access count for the given key, or 0 if
// the key is not being tracked.
func (d *HotKeyDetector) Count(key string) int64 {
	d.mu.RLock()
	c, ok := d.counters[key]
	d.mu.RUnlock()

	if !ok {
		return 0
	}
	return atomic.LoadInt64(&c.count)
}

// TopKeys returns the top N keys by access count, sorted in descending
// order. Only keys with non-zero counts in the current window are included.
func (d *HotKeyDetector) TopKeys(n int) []KeyCount {
	// Check cached results first.
	d.topNMu.RLock()
	if len(d.topN) >= n {
		cached := make([]KeyCount, n)
		copy(cached, d.topN[:n])
		d.topNMu.RUnlock()
		return cached
	}
	d.topNMu.RUnlock()

	d.mu.RLock()
	keys := make([]string, 0, len(d.counters))
	for k := range d.counters {
		keys = append(keys, k)
	}
	d.mu.RUnlock()

	results := make([]KeyCount, 0, len(keys))
	for _, k := range keys {
		count := d.Count(k)
		if count > 0 {
			results = append(results, KeyCount{Key: k, Count: count})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	if n > 0 && len(results) > n {
		results = results[:n]
	}

	// Cache the results.
	d.topNMu.Lock()
	d.topN = results
	d.topNMu.Unlock()

	return results
}

// Reset clears all tracked keys and their counters.
func (d *HotKeyDetector) Reset() {
	d.mu.Lock()
	d.counters = make(map[string]*counter)
	d.mu.Unlock()

	d.topNMu.Lock()
	d.topN = nil
	d.topNMu.Unlock()
}

// Size returns the number of keys currently being tracked.
func (d *HotKeyDetector) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.counters)
}

// evictOldest removes the counter with the oldest window start time.
// Caller must hold d.mu write lock.
func (d *HotKeyDetector) evictOldest() {
	var oldestKey string
	var oldestWindow int64
	for k, c := range d.counters {
		w := atomic.LoadInt64(&c.window)
		if oldestKey == "" || w < oldestWindow {
			oldestKey = k
			oldestWindow = w
		}
	}
	if oldestKey != "" {
		delete(d.counters, oldestKey)
	}
}
