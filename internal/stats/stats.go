// Package stats provides atomic counters for cache operations with
// cache-line padding to eliminate false sharing on hot paths.
package stats

import (
	"sync/atomic"
	"time"
)

// cacheLinePad prevents false sharing between adjacent atomic fields.
// 128 bytes covers both AMD64 and ARM64 cache line sizes.
type cacheLinePad [128 / 8]uint64

// Stats tracks cache operation counters using lock-free atomics.
// Cache-line padding is inserted between the most heavily contended
// field groups (hits/misses vs sets/deletes) to prevent false sharing.
type Stats struct {
	_            cacheLinePad
	hits         int64
	misses       int64
	_            cacheLinePad
	gets         int64
	sets         int64
	deletes      int64
	errors       int64
	evictions    int64
	_            cacheLinePad
	items        int64
	memory       int64
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

// Recorder is the unified interface for recording cache events.
// All backends should use this interface rather than calling
// Stats methods directly, enabling alternative implementations.
type Recorder interface {
	// Core operations.
	Hit()
	Miss()
	RecordGet()
	SetOp()
	DeleteOp()
	ErrorOp()
	EvictionOp()

	// Layered cache operations.
	L1Hit()
	L1Miss()
	L2Hit()
	L2Miss()
	L2Promotion()

	// Write-back operations.
	WriteBackEnqueue()
	WriteBackFlush()
	WriteBackDrop()
}

// Compile-time check that Stats implements Recorder.
var _ Recorder = (*Stats)(nil)

// NewStats returns a new Stats instance with the start time recorded.
func NewStats() *Stats {
	s := &Stats{}
	atomic.StoreInt64(&s.startNs, time.Now().UnixNano())
	return s
}

func inc(p *int64)            { atomic.AddInt64(p, 1) }
func add(p *int64, n int64)   { atomic.AddInt64(p, n) }
func load(p *int64) int64     { return atomic.LoadInt64(p) }
func store(p *int64, v int64) { atomic.StoreInt64(p, v) }

// Core operation recorders.

func (s *Stats) Hit()        { inc(&s.hits) }
func (s *Stats) Miss()       { inc(&s.misses) }
func (s *Stats) RecordGet()  { inc(&s.gets) }
func (s *Stats) SetOp()      { inc(&s.sets) }
func (s *Stats) DeleteOp()   { inc(&s.deletes) }
func (s *Stats) EvictionOp() { inc(&s.evictions) }
func (s *Stats) ErrorOp()    { inc(&s.errors) }

// Size trackers.

func (s *Stats) AddItems(n int64)  { add(&s.items, n) }
func (s *Stats) AddMemory(n int64) { add(&s.memory, n) }

// Layered cache recorders.

func (s *Stats) L1Hit()       { inc(&s.l1Hits); inc(&s.hits) }
func (s *Stats) L1Miss()      { inc(&s.l1Misses) }
func (s *Stats) L1Error()     { inc(&s.l1Errors); inc(&s.errors) }
func (s *Stats) L2Hit()       { inc(&s.l2Hits) }
func (s *Stats) L2Miss()      { inc(&s.l2Misses) }
func (s *Stats) L2Error()     { inc(&s.l2Errors); inc(&s.errors) }
func (s *Stats) L2Promotion() { inc(&s.l2Promotions) }

// Write-back recorders.

func (s *Stats) WriteBackEnqueue() { inc(&s.wbEnqueued) }
func (s *Stats) WriteBackFlush()   { inc(&s.wbFlushed) }
func (s *Stats) WriteBackDrop()    { inc(&s.wbDropped) }

// Accessors.

func (s *Stats) Hits() int64      { return load(&s.hits) }
func (s *Stats) Misses() int64    { return load(&s.misses) }
func (s *Stats) Gets() int64      { return load(&s.gets) }
func (s *Stats) Sets() int64      { return load(&s.sets) }
func (s *Stats) Deletes() int64   { return load(&s.deletes) }
func (s *Stats) Evictions() int64 { return load(&s.evictions) }
func (s *Stats) Errors() int64    { return load(&s.errors) }
func (s *Stats) Items() int64     { return load(&s.items) }
func (s *Stats) Memory() int64    { return load(&s.memory) }

func (s *Stats) L1Hits() int64       { return load(&s.l1Hits) }
func (s *Stats) L1Misses() int64     { return load(&s.l1Misses) }
func (s *Stats) L1Errors() int64     { return load(&s.l1Errors) }
func (s *Stats) L2Hits() int64       { return load(&s.l2Hits) }
func (s *Stats) L2Misses() int64     { return load(&s.l2Misses) }
func (s *Stats) L2Errors() int64     { return load(&s.l2Errors) }
func (s *Stats) L2Promotions() int64 { return load(&s.l2Promotions) }

func (s *Stats) WriteBackEnqueued() int64 { return load(&s.wbEnqueued) }
func (s *Stats) WriteBackFlushed() int64  { return load(&s.wbFlushed) }
func (s *Stats) WriteBackDropped() int64  { return load(&s.wbDropped) }

// Derived metrics.

func (s *Stats) HitRate() float64   { return hitRate(load(&s.hits), load(&s.misses)) }
func (s *Stats) L1HitRate() float64 { return hitRate(load(&s.l1Hits), load(&s.l1Misses)) }
func (s *Stats) L2HitRate() float64 { return hitRate(load(&s.l2Hits), load(&s.l2Misses)) }

func (s *Stats) OpsPerSecond() float64 {
	elapsed := time.Duration(time.Now().UnixNano() - load(&s.startNs))
	if elapsed <= 0 {
		return 0
	}
	ops := load(&s.gets) + load(&s.sets)
	return float64(ops) / elapsed.Seconds()
}

func (s *Stats) Uptime() time.Duration {
	return time.Duration(time.Now().UnixNano() - load(&s.startNs))
}

func hitRate(hits, misses int64) float64 {
	if total := hits + misses; total > 0 {
		return float64(hits) / float64(total) * 100
	}
	return 0
}

// Snapshot holds a point-in-time copy of all stats fields.
type Snapshot struct {
	Hits, Misses, Gets, Sets   int64
	Deletes, Evictions         int64
	Errors                     int64
	Items, Memory              int64
	HitRate, OpsPerSecond      float64
	Uptime                     time.Duration
	L1Hits, L1Misses, L1Errors int64
	L2Hits, L2Misses, L2Errors int64
	L2Promotions               int64
	L1HitRate, L2HitRate       float64
	WriteBackEnqueued          int64
	WriteBackFlushed           int64
	WriteBackDropped           int64
}

// TakeSnapshot returns a point-in-time copy of all stats.
func (s *Stats) TakeSnapshot() Snapshot {
	return Snapshot{
		Hits: s.Hits(), Misses: s.Misses(), Gets: s.Gets(), Sets: s.Sets(),
		Deletes: s.Deletes(), Evictions: s.Evictions(), Errors: s.Errors(),
		Items: s.Items(), Memory: s.Memory(),
		HitRate: s.HitRate(), OpsPerSecond: s.OpsPerSecond(), Uptime: s.Uptime(),
		L1Hits: s.L1Hits(), L1Misses: s.L1Misses(), L1Errors: s.L1Errors(),
		L2Hits: s.L2Hits(), L2Misses: s.L2Misses(), L2Errors: s.L2Errors(),
		L2Promotions: s.L2Promotions(), L1HitRate: s.L1HitRate(), L2HitRate: s.L2HitRate(),
		WriteBackEnqueued: s.WriteBackEnqueued(),
		WriteBackFlushed:  s.WriteBackFlushed(),
		WriteBackDropped:  s.WriteBackDropped(),
	}
}

// Snapshot is an alias for TakeSnapshot.
func (s *Stats) Snapshot() Snapshot { return s.TakeSnapshot() }

// Reset zeroes all counters and resets the start time.
func (s *Stats) Reset() {
	now := time.Now().UnixNano()
	for _, p := range []*int64{
		&s.hits, &s.misses, &s.gets, &s.sets, &s.deletes, &s.evictions, &s.errors,
		&s.items, &s.memory,
		&s.l1Hits, &s.l1Misses, &s.l1Errors, &s.l2Hits, &s.l2Misses, &s.l2Errors,
		&s.l2Promotions, &s.wbEnqueued, &s.wbFlushed, &s.wbDropped,
	} {
		store(p, 0)
	}
	store(&s.startNs, now)
}
