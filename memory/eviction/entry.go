package eviction

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

var entryPool = sync.Pool{
	New: func() any { return new(Entry) },
}

// Entry represents a single cache entry with both hard and soft expiry times.
// The hard expiry (ExpiresAt) is the absolute deadline after which the entry
// is considered stale and must be evicted. The soft expiry (SoftExpiresAt) is
// the point at which the entry becomes a candidate for proactive background
// refresh via the XFetch algorithm, while still serving the cached value to
// callers. This dual-TTL model enables stampede-resistant early refresh.
type Entry struct {
	Key       string
	Value     []byte
	AccessAt  int64
	ExpiresAt int64
	// SoftExpiresAt is the time before hard expiry when background refresh
	// should be considered. If zero, it defaults to 85% of the hard TTL.
	SoftExpiresAt int64
	CreatedAt     int64
	Size          int64
	Hits          int64
	Frequency     int64
	Recency       int64
	// RefreshCount tracks the number of times this entry has been
	// proactively refreshed before hard expiry.
	RefreshCount int64
	released     atomic.Bool
}

// AcquireEntry retrieves an Entry from the pool and initializes it.
// If softTTL is 0, the soft expiry defaults to 85% of the hard TTL.
func AcquireEntry(key string, value []byte, ttl, softTTL time.Duration) *Entry {
	e := entryPool.Get().(*Entry)
	e.initialize(key, value, ttl, softTTL)
	return e
}

// NewEntry creates a fresh Entry without using the pool.
// If softTTL is 0, the soft expiry defaults to 85% of the hard TTL.
func NewEntry(key string, value []byte, ttl, softTTL time.Duration) *Entry {
	e := &Entry{}
	e.initialize(key, value, ttl, softTTL)
	return e
}

func (e *Entry) initialize(key string, value []byte, ttl, softTTL time.Duration) {
	e.released.Store(false)
	now := time.Now().UnixNano()
	e.Key = key
	e.Value = value
	e.Size = int64(len(key) + len(value))
	e.CreatedAt = now
	e.AccessAt = now
	e.ExpiresAt, _ = e.calcExpiry(now, ttl)

	// Calculate soft expiry.
	switch {
	case softTTL > 0 && softTTL < ttl:
		e.SoftExpiresAt = now + softTTL.Nanoseconds()
	case ttl > 0:
		// Default: 85% of hard TTL.
		e.SoftExpiresAt = now + int64(float64(ttl.Nanoseconds())*0.85)
	default:
		// No TTL means no soft expiry.
		e.SoftExpiresAt = 0
	}

	atomic.StoreInt64(&e.Hits, 1)
	atomic.StoreInt64(&e.Frequency, 0)
	atomic.StoreInt64(&e.Recency, 0)
	atomic.StoreInt64(&e.RefreshCount, 0)
}

// ReleaseEntry returns an Entry to the pool. It is safe to call multiple times.
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
	e.SoftExpiresAt = 0
	atomic.StoreInt64(&e.Hits, 0)
	atomic.StoreInt64(&e.Frequency, 0)
	atomic.StoreInt64(&e.Recency, 0)
	atomic.StoreInt64(&e.RefreshCount, 0)
}

// WithNewTTL returns a copy of the entry with a new hard TTL and recalculated
// soft expiry (85% of the new TTL).
func (e *Entry) WithNewTTL(ttl time.Duration) *Entry {
	expiry, _ := e.calcExpiry(time.Now().UnixNano(), ttl)
	ne := &Entry{
		Key:           e.Key,
		Value:         e.Value,
		Size:          e.Size,
		ExpiresAt:     expiry,
		SoftExpiresAt: e.SoftExpiresAt,
		CreatedAt:     e.GetCreatedAt(),
		AccessAt:      e.GetAccessAt(),
		Hits:          e.GetHits(),
		Frequency:     e.GetFrequency(),
		Recency:       e.GetRecency(),
		RefreshCount:  e.GetRefreshCount(),
	}
	// Recalculate soft expiry for the new TTL.
	if ttl > 0 {
		ne.SoftExpiresAt = time.Now().UnixNano() + int64(float64(ttl.Nanoseconds())*0.85)
	} else {
		ne.SoftExpiresAt = 0
	}
	return ne
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

// IsExpired reports whether the entry has passed its hard expiry.
func (e *Entry) IsExpired() bool { return e.isExpiredAt(time.Now().UnixNano()) }

// IsExpiredAt reports whether the entry has passed its hard expiry at the given time.
func (e *Entry) IsExpiredAt(now int64) bool { return e.isExpiredAt(now) }

func (e *Entry) isExpiredAt(now int64) bool {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	return exp != 0 && now > exp
}

// Touch updates the access time and increments the hit counter.
func (e *Entry) Touch() {
	atomic.StoreInt64(&e.AccessAt, time.Now().UnixNano())
	atomic.AddInt64(&e.Hits, 1)
}

// TTLRemaining returns the duration until hard expiry.
func (e *Entry) TTLRemaining() time.Duration {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	if exp == 0 {
		return 0
	}
	return time.Duration(exp - time.Now().UnixNano())
}

// ShouldEarlyRefresh implements the XFetch algorithm (inspired by Varnish and
// Nginx cache). It returns true with probability proportional to how close we
// are to the soft expiry boundary. This enables probabilistic early refresh
// that spreads refresh attempts over time, preventing cache stampedes.
//
// The beta parameter controls aggressiveness:
//   - beta ∈ (0, 1]: higher values cause more aggressive early refresh.
//   - A typical value is 0.5–1.0.
//
// When the entry has no soft expiry (SoftExpiresAt == 0) or no hard expiry
// (ExpiresAt == 0), this always returns false.
func (e *Entry) ShouldEarlyRefresh(beta float64) bool {
	softExp := atomic.LoadInt64(&e.SoftExpiresAt)
	hardExp := atomic.LoadInt64(&e.ExpiresAt)
	if softExp == 0 || hardExp == 0 {
		return false
	}
	now := time.Now().UnixNano()
	// Already past soft expiry — always refresh.
	if now >= softExp {
		return true
	}
	delta := float64(hardExp - softExp)
	t := float64(softExp - now)
	// XFetch: the probability of triggering early refresh increases as we
	// approach soft expiry. The random logarithm ensures that refreshes are
	// spread out rather than all hitting at once.
	return t-delta*beta*math.Log(rand.Float64()) <= 0
}

// GetAccessAt returns the last access time (nanoseconds since epoch).
func (e *Entry) GetAccessAt() int64 { return atomic.LoadInt64(&e.AccessAt) }

// GetCreatedAt returns the creation time (nanoseconds since epoch).
func (e *Entry) GetCreatedAt() int64 { return atomic.LoadInt64(&e.CreatedAt) }

// GetExpiresAt returns the hard expiry time (nanoseconds since epoch).
func (e *Entry) GetExpiresAt() int64 { return atomic.LoadInt64(&e.ExpiresAt) }

// GetSoftExpiresAt returns the soft expiry time (nanoseconds since epoch).
func (e *Entry) GetSoftExpiresAt() int64 { return atomic.LoadInt64(&e.SoftExpiresAt) }

// GetHits returns the hit count.
func (e *Entry) GetHits() int64 { return atomic.LoadInt64(&e.Hits) }

// GetFrequency returns the eviction frequency score.
func (e *Entry) GetFrequency() int64 { return atomic.LoadInt64(&e.Frequency) }

// GetRecency returns the eviction recency score.
func (e *Entry) GetRecency() int64 { return atomic.LoadInt64(&e.Recency) }

// GetRefreshCount returns the number of proactive refreshes performed.
func (e *Entry) GetRefreshCount() int64 { return atomic.LoadInt64(&e.RefreshCount) }

// IncrRefreshCount atomically increments the refresh counter and returns the new value.
func (e *Entry) IncrRefreshCount() int64 { return atomic.AddInt64(&e.RefreshCount, 1) }

func (e *Entry) HitsCount() int64      { return e.GetHits() }
func (e *Entry) FrequencyCount() int64 { return e.GetFrequency() }

func (e *Entry) SetFrequency(freq int64)  { atomic.StoreInt64(&e.Frequency, freq) }
func (e *Entry) SetRecency(recency int64) { atomic.StoreInt64(&e.Recency, recency) }
func (e *Entry) IncrFrequency() int64     { return atomic.AddInt64(&e.Frequency, 1) }
func (e *Entry) IncrHits() int64          { return atomic.AddInt64(&e.Hits, 1) }
