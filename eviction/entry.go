// Package eviction provides cache-entry lifecycle management and pluggable
// eviction policies (LRU, LFU, FIFO, LIFO, MRU, Random, TinyLFU).
package eviction

import (
	"sync"
	"sync/atomic"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/clock"
)

// entryPool recycles Entry objects to reduce GC pressure on the hot path.
var entryPool = sync.Pool{
	New: func() any { return new(Entry) },
}

// Entry is the internal high-performance cache item representation.
// All mutable fields that are read on the hot path (AccessAt, Hits, …) use
// atomic loads and stores so readers never need to hold a shard lock beyond
// the initial lookup.
type Entry struct {
	Key       string
	Value     []byte
	AccessAt  int64
	ExpiresAt int64
	CreatedAt int64
	Size      int64
	Hits      int64
	Frequency int64
	Recency   int64
	released  atomic.Bool
}

// AcquireEntry returns a pooled Entry initialized for use.
func AcquireEntry(key string, value []byte, ttl time.Duration) *Entry {
	e := entryPool.Get().(*Entry) //nolint:forcetypeassert // entryPool.New always returns *Entry
	e.initialize(key, value, ttl)
	return e
}

// NewEntry creates a non-pooled Entry (used for CAS replacements).
func NewEntry(key string, value []byte, ttl time.Duration) *Entry {
	e := &Entry{}
	e.initialize(key, value, ttl)
	return e
}

func (e *Entry) initialize(key string, value []byte, ttl time.Duration) {
	e.released.Store(false)
	now := clock.Now()
	e.Key = key
	e.Value = value
	e.Size = int64(len(key) + len(value))
	e.CreatedAt = now
	e.AccessAt = now
	e.ExpiresAt, _ = e.calcExpiry(now, ttl)
	atomic.StoreInt64(&e.Hits, 1)
	atomic.StoreInt64(&e.Frequency, 0)
	atomic.StoreInt64(&e.Recency, 0)
}

// ReleaseEntry returns e to the pool.  Must not be used after calling this.
func ReleaseEntry(e *Entry) {
	if e.released.Swap(true) {
		return
	}
	e.reset()
	entryPool.Put(e)
}

func (e *Entry) reset() {
	e.Key = ""
	e.Value = nil
	e.Size = 0
	e.CreatedAt = 0
	e.AccessAt = 0
	e.ExpiresAt = 0
	atomic.StoreInt64(&e.Hits, 0)
	atomic.StoreInt64(&e.Frequency, 0)
	atomic.StoreInt64(&e.Recency, 0)
}

// WithNewTTL returns a shallow copy with an updated expiry.
// The Value slice is shared under the immutable-value contract.
func (e *Entry) WithNewTTL(ttl time.Duration) *Entry {
	expiry, _ := e.calcExpiry(clock.Now(), ttl)
	return &Entry{
		Key:       e.Key,
		Value:     e.Value,
		Size:      e.Size,
		ExpiresAt: expiry,
		CreatedAt: e.GetCreatedAt(),
		AccessAt:  e.GetAccessAt(),
		Hits:      e.GetHits(),
		Frequency: e.GetFrequency(),
		Recency:   e.GetRecency(),
	}
}

func (e *Entry) calcExpiry(now int64, ttl time.Duration) (int64, error) {
	if ttl < 0 {
		return 0, _errors.InvalidConfig("entry.calcExpiry", "ttl must be non-negative")
	}
	if ttl == 0 {
		return 0, nil
	}
	return now + ttl.Nanoseconds(), nil
}

// IsExpired reports whether the entry has expired as of now.
func (e *Entry) IsExpired() bool { return e.isExpiredAt(clock.Now()) }

// IsExpiredAt reports whether the entry had expired at the provided timestamp.
func (e *Entry) IsExpiredAt(now int64) bool { return e.isExpiredAt(now) }

func (e *Entry) isExpiredAt(now int64) bool {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	return exp != 0 && now > exp
}

// Touch updates AccessAt and increments Hits.
func (e *Entry) Touch() {
	atomic.StoreInt64(&e.AccessAt, clock.Now())
	atomic.AddInt64(&e.Hits, 1)
}

// TTLRemaining returns the remaining TTL, or 0 for no-expiry entries.
func (e *Entry) TTLRemaining() time.Duration {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	if exp == 0 {
		return 0
	}
	return time.Duration(exp - clock.Now())
}

// --- atomic accessors ---

func (e *Entry) GetAccessAt() int64  { return atomic.LoadInt64(&e.AccessAt) }
func (e *Entry) GetCreatedAt() int64 { return atomic.LoadInt64(&e.CreatedAt) }
func (e *Entry) GetExpiresAt() int64 { return atomic.LoadInt64(&e.ExpiresAt) }
func (e *Entry) GetHits() int64      { return atomic.LoadInt64(&e.Hits) }
func (e *Entry) GetFrequency() int64 { return atomic.LoadInt64(&e.Frequency) }
func (e *Entry) GetRecency() int64   { return atomic.LoadInt64(&e.Recency) }

// HitsCount is an alias for GetHits kept for backward compatibility.
func (e *Entry) HitsCount() int64 { return e.GetHits() }

// FrequencyCount is an alias for GetFrequency kept for backward compatibility.
func (e *Entry) FrequencyCount() int64 { return e.GetFrequency() }

// --- mutators ---

func (e *Entry) SetFrequency(freq int64)  { atomic.StoreInt64(&e.Frequency, freq) }
func (e *Entry) SetRecency(recency int64) { atomic.StoreInt64(&e.Recency, recency) }
func (e *Entry) IncrFrequency() int64     { return atomic.AddInt64(&e.Frequency, 1) }
func (e *Entry) IncrHits() int64          { return atomic.AddInt64(&e.Hits, 1) }
