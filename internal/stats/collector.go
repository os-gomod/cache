package stats

import (
	"sync/atomic"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// cacheLinePad prevents false sharing between adjacent atomic fields by
// padding them to a full CPU cache line (128 bytes on most architectures).
type cacheLinePad [128 / 8]uint64

// Stats is a lock-free statistics collector for cache operations. All fields
// are individually aligned to cache lines to prevent false sharing on
// multi-core systems.
//
// Stats satisfies the runtime.StatsCollector interface and the
// contracts.StatsProvider interface. It also provides layered cache metrics
// (L1/L2) and write-back metrics for advanced store implementations.
type Stats struct {
	_            cacheLinePad
	hits         int64
	misses       int64
	_            cacheLinePad
	sets         int64
	deletes      int64
	errors       int64
	evictions    int64
	_            cacheLinePad
	items        int64
	memoryBytes  int64
	maxMemory    int64
	_            cacheLinePad
	l1Hits       int64
	l1Misses     int64
	l1Errors     int64
	_            cacheLinePad
	l2Hits       int64
	l2Misses     int64
	l2Errors     int64
	l2Promotions int64
	_            cacheLinePad
	wbEnqueued   int64
	wbFlushed    int64
	wbDropped    int64
	_            cacheLinePad
	startNs      int64
}

// NewStats creates a new Stats instance with the start time set to now.
func NewStats() *Stats {
	s := &Stats{}
	atomic.StoreInt64(&s.startNs, time.Now().UnixNano())
	return s
}

// --- Atomic helpers ---

func inc(p *int64)               { atomic.AddInt64(p, 1) }
func addInt(p *int64, n int64)   { atomic.AddInt64(p, n) }
func loadInt(p *int64) int64     { return atomic.LoadInt64(p) }
func storeInt(p *int64, v int64) { atomic.StoreInt64(p, v) }

// --- Core counter methods (StatsCollector interface) ---

// Hit records a cache hit.
func (s *Stats) Hit() { inc(&s.hits) }

// Miss records a cache miss.
func (s *Stats) Miss() { inc(&s.misses) }

// Set records a set operation.
func (s *Stats) Set() { inc(&s.sets) }

// Delete records a delete operation.
func (s *Stats) Delete() { inc(&s.deletes) }

// Error records an error.
func (s *Stats) Error() { inc(&s.errors) }

// Eviction records an eviction.
func (s *Stats) Eviction() { inc(&s.evictions) }

// --- Aliases for backward compatibility with existing store code ---

// SetOp records a set operation (alias for Set).
func (s *Stats) SetOp() { inc(&s.sets) }

// DeleteOp records a delete operation (alias for Delete).
func (s *Stats) DeleteOp() { inc(&s.deletes) }

// ErrorOp records an error (alias for Error).
func (s *Stats) ErrorOp() { inc(&s.errors) }

// EvictionOp records an eviction (alias for Eviction).
func (s *Stats) EvictionOp() { inc(&s.evictions) }

// RecordGet records a get operation (no-op counter; hits/misses are tracked separately).
func (*Stats) RecordGet() {}

// --- Memory and item tracking ---

// AddMemory adjusts the memory byte count by n (can be negative).
func (s *Stats) AddMemory(n int64) { addInt(&s.memoryBytes, n) }

// SubMemory decreases the memory byte count by n.
func (s *Stats) SubMemory(n int64) { addInt(&s.memoryBytes, -n) }

// AddItems adjusts the item count by n (can be negative).
func (s *Stats) AddItems(n int64) { addInt(&s.items, n) }

// SubItems decreases the item count by n.
func (s *Stats) SubItems(n int64) { addInt(&s.items, -n) }

// --- Reader methods ---

// Hits returns the total number of cache hits.
func (s *Stats) Hits() int64 { return loadInt(&s.hits) }

// Misses returns the total number of cache misses.
func (s *Stats) Misses() int64 { return loadInt(&s.misses) }

// Sets returns the total number of set operations.
func (s *Stats) Sets() int64 { return loadInt(&s.sets) }

// Deletes returns the total number of delete operations.
func (s *Stats) Deletes() int64 { return loadInt(&s.deletes) }

// Evictions returns the total number of evictions.
func (s *Stats) Evictions() int64 { return loadInt(&s.evictions) }

// Errors returns the total number of errors.
func (s *Stats) Errors() int64 { return loadInt(&s.errors) }

// Items returns the current number of items in the cache.
func (s *Stats) Items() int64 { return loadInt(&s.items) }

// Memory returns the current memory usage in bytes.
func (s *Stats) Memory() int64 { return loadInt(&s.memoryBytes) }

// --- Layered cache metrics ---

// L1Hit records a hit in the L1 (fast) cache layer.
func (s *Stats) L1Hit() { inc(&s.l1Hits); inc(&s.hits) }

// L1Miss records a miss in the L1 cache layer.
func (s *Stats) L1Miss() { inc(&s.l1Misses) }

// L1Error records an error from the L1 cache layer.
func (s *Stats) L1Error() { inc(&s.l1Errors); inc(&s.errors) }

// L2Hit records a hit in the L2 (slow) cache layer.
func (s *Stats) L2Hit() { inc(&s.l2Hits) }

// L2Miss records a miss in the L2 cache layer.
func (s *Stats) L2Miss() { inc(&s.l2Misses) }

// L2Error records an error from the L2 cache layer.
func (s *Stats) L2Error() { inc(&s.l2Errors); inc(&s.errors) }

// L2Promotion records a promotion from L2 to L1.
func (s *Stats) L2Promotion() { inc(&s.l2Promotions) }

// L1Hits returns the number of L1 cache hits.
func (s *Stats) L1Hits() int64 { return loadInt(&s.l1Hits) }

// L1Misses returns the number of L1 cache misses.
func (s *Stats) L1Misses() int64 { return loadInt(&s.l1Misses) }

// L1Errors returns the number of L1 cache errors.
func (s *Stats) L1Errors() int64 { return loadInt(&s.l1Errors) }

// L2Hits returns the number of L2 cache hits.
func (s *Stats) L2Hits() int64 { return loadInt(&s.l2Hits) }

// L2Misses returns the number of L2 cache misses.
func (s *Stats) L2Misses() int64 { return loadInt(&s.l2Misses) }

// L2Errors returns the number of L2 cache errors.
func (s *Stats) L2Errors() int64 { return loadInt(&s.l2Errors) }

// L2Promotions returns the number of L2-to-L1 promotions.
func (s *Stats) L2Promotions() int64 { return loadInt(&s.l2Promotions) }

// --- Write-back metrics ---

// WriteBackEnqueue records a write-back job being enqueued.
func (s *Stats) WriteBackEnqueue() { inc(&s.wbEnqueued) }

// WriteBackFlush records a write-back job being flushed to L2.
func (s *Stats) WriteBackFlush() { inc(&s.wbFlushed) }

// WriteBackDrop records a write-back job being dropped (queue full).
func (s *Stats) WriteBackDrop() { inc(&s.wbDropped) }

// WriteBackEnqueued returns the number of write-back jobs enqueued.
func (s *Stats) WriteBackEnqueued() int64 { return loadInt(&s.wbEnqueued) }

// WriteBackFlushed returns the number of write-back jobs flushed.
func (s *Stats) WriteBackFlushed() int64 { return loadInt(&s.wbFlushed) }

// WriteBackDropped returns the number of write-back jobs dropped.
func (s *Stats) WriteBackDropped() int64 { return loadInt(&s.wbDropped) }

// --- Computed metrics ---

// HitRate returns the cache hit rate as a float between 0 and 1.
func (s *Stats) HitRate() float64 {
	h := loadInt(&s.hits)
	m := loadInt(&s.misses)
	if total := h + m; total > 0 {
		return float64(h) / float64(total)
	}
	return 0
}

// L1HitRate returns the L1 hit rate as a float between 0 and 1.
func (s *Stats) L1HitRate() float64 {
	h := loadInt(&s.l1Hits)
	m := loadInt(&s.l1Misses)
	if total := h + m; total > 0 {
		return float64(h) / float64(total)
	}
	return 0
}

// L2HitRate returns the L2 hit rate as a float between 0 and 1.
func (s *Stats) L2HitRate() float64 {
	h := loadInt(&s.l2Hits)
	m := loadInt(&s.l2Misses)
	if total := h + m; total > 0 {
		return float64(h) / float64(total)
	}
	return 0
}

// OpsPerSecond returns the average operations per second since the stats were created.
func (s *Stats) OpsPerSecond() float64 {
	elapsed := time.Duration(time.Now().UnixNano() - loadInt(&s.startNs))
	if elapsed <= 0 {
		return 0
	}
	ops := loadInt(&s.hits) + loadInt(&s.misses) + loadInt(&s.sets) + loadInt(&s.deletes)
	return float64(ops) / elapsed.Seconds()
}

// StartTime returns the time at which the stats were created.
func (s *Stats) StartTime() time.Time {
	return time.Unix(0, loadInt(&s.startNs))
}

// Uptime returns the duration since the stats were created.
func (s *Stats) Uptime() time.Duration {
	return time.Duration(time.Now().UnixNano() - loadInt(&s.startNs))
}

// SetMaxMemory sets the maximum memory limit for the cache.
func (s *Stats) SetMaxMemory(bytes int64) {
	atomic.StoreInt64(&s.maxMemory, bytes)
}

// MaxMemory returns the configured maximum memory limit.
func (s *Stats) MaxMemory() int64 {
	return atomic.LoadInt64(&s.maxMemory)
}

// --- StatsProvider interface ---

// Stats returns a point-in-time snapshot of cache statistics as contracts.StatsSnapshot.
// This method satisfies the contracts.StatsProvider interface.
func (s *Stats) Stats() contracts.StatsSnapshot {
	return s.TakeSnapshot()
}

// TakeSnapshot captures an immutable point-in-time snapshot of all statistics.
func (s *Stats) TakeSnapshot() contracts.StatsSnapshot {
	return s.Snapshot()
}

// Snapshot captures an immutable point-in-time snapshot of all statistics.
// It returns a contracts.StatsSnapshot with all counter values atomically read.
func (s *Stats) Snapshot() contracts.StatsSnapshot {
	return contracts.StatsSnapshot{
		Hits:        s.Hits(),
		Misses:      s.Misses(),
		Sets:        s.Sets(),
		Deletes:     s.Deletes(),
		Evictions:   s.Evictions(),
		Errors:      s.Errors(),
		Items:       s.Items(),
		MemoryBytes: s.Memory(),
		MaxMemory:   s.MaxMemory(),
		StartTime:   s.StartTime(),
	}
}

// Reset resets all counters to zero and updates the start time.
func (s *Stats) Reset() {
	now := time.Now().UnixNano()
	for _, p := range []*int64{
		hitsPtr(s), missesPtr(s), setsPtr(s), deletesPtr(s),
		evictionsPtr(s), errorsPtr(s), itemsPtr(s), memoryBytesPtr(s),
		l1HitsPtr(s), l1MissesPtr(s), l1ErrorsPtr(s),
		l2HitsPtr(s), l2MissesPtr(s), l2ErrorsPtr(s), l2PromotionsPtr(s),
		wbEnqueuedPtr(s), wbFlushedPtr(s), wbDroppedPtr(s),
	} {
		storeInt(p, 0)
	}
	storeInt(&s.startNs, now)
}

// Pointer helpers for Reset to avoid &s.field literal addressing issues.
func hitsPtr(s *Stats) *int64         { return &s.hits }
func missesPtr(s *Stats) *int64       { return &s.misses }
func setsPtr(s *Stats) *int64         { return &s.sets }
func deletesPtr(s *Stats) *int64      { return &s.deletes }
func evictionsPtr(s *Stats) *int64    { return &s.evictions }
func errorsPtr(s *Stats) *int64       { return &s.errors }
func itemsPtr(s *Stats) *int64        { return &s.items }
func memoryBytesPtr(s *Stats) *int64  { return &s.memoryBytes }
func l1HitsPtr(s *Stats) *int64       { return &s.l1Hits }
func l1MissesPtr(s *Stats) *int64     { return &s.l1Misses }
func l1ErrorsPtr(s *Stats) *int64     { return &s.l1Errors }
func l2HitsPtr(s *Stats) *int64       { return &s.l2Hits }
func l2MissesPtr(s *Stats) *int64     { return &s.l2Misses }
func l2ErrorsPtr(s *Stats) *int64     { return &s.l2Errors }
func l2PromotionsPtr(s *Stats) *int64 { return &s.l2Promotions }
func wbEnqueuedPtr(s *Stats) *int64   { return &s.wbEnqueued }
func wbFlushedPtr(s *Stats) *int64    { return &s.wbFlushed }
func wbDroppedPtr(s *Stats) *int64    { return &s.wbDropped }

// Compile-time interface checks.
var (
	_ interface{ Hit() }      = (*Stats)(nil)
	_ interface{ Miss() }     = (*Stats)(nil)
	_ interface{ Set() }      = (*Stats)(nil)
	_ interface{ Delete() }   = (*Stats)(nil)
	_ interface{ Error() }    = (*Stats)(nil)
	_ interface{ Eviction() } = (*Stats)(nil)
	//nolint:interfacebloat // Stats is a comprehensive statistics collector; 14 methods is acceptable for this domain
	_ interface {
		SetOp()
		DeleteOp()
		ErrorOp()
		EvictionOp()
		L1Hit()
		L1Miss()
		L1Error()
		L2Hit()
		L2Miss()
		L2Error()
		L2Promotion()
		WriteBackEnqueue()
		WriteBackFlush()
		WriteBackDrop()
	} = (*Stats)(nil)
	_ interface{ AddMemory(bytes int64) } = (*Stats)(nil)
	_ interface{ SubMemory(bytes int64) } = (*Stats)(nil)
	_ interface{ AddItems(count int64) }  = (*Stats)(nil)
	_ interface{ SubItems(count int64) }  = (*Stats)(nil)
	_ interface {
		TakeSnapshot() contracts.StatsSnapshot
	} = (*Stats)(nil)
	_ interface {
		Snapshot() contracts.StatsSnapshot
	} = (*Stats)(nil)
	_ contracts.StatsProvider = (*Stats)(nil)
)
