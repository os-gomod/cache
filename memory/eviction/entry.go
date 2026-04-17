package eviction

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
)

var entryPool = sync.Pool{
	New: func() any { return new(Entry) },
}

type Entry struct {
	Key           string
	Value         []byte
	AccessAt      int64
	ExpiresAt     int64
	SoftExpiresAt int64
	CreatedAt     int64
	Size          int64
	Hits          int64
	Frequency     int64
	Recency       int64
	RefreshCount  int64
	released      atomic.Bool
}

func AcquireEntry(key string, value []byte, ttl, softTTL time.Duration) *Entry {
	e := entryPool.Get().(*Entry)
	e.initialize(key, value, ttl, softTTL)
	return e
}

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
	switch {
	case softTTL > 0 && softTTL < ttl:
		e.SoftExpiresAt = now + softTTL.Nanoseconds()
	case ttl > 0:
		e.SoftExpiresAt = now + int64(float64(ttl.Nanoseconds())*0.85)
	default:
		e.SoftExpiresAt = 0
	}
	atomic.StoreInt64(&e.Hits, 1)
	atomic.StoreInt64(&e.Frequency, 0)
	atomic.StoreInt64(&e.Recency, 0)
	atomic.StoreInt64(&e.RefreshCount, 0)
}

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
	if ttl > 0 {
		ne.SoftExpiresAt = time.Now().UnixNano() + int64(float64(ttl.Nanoseconds())*0.85)
	} else {
		ne.SoftExpiresAt = 0
	}
	return ne
}

func (e *Entry) calcExpiry(now int64, ttl time.Duration) (int64, error) {
	if ttl < 0 {
		return 0, cacheerrors.InvalidConfig("entry.calcExpiry", "ttl must be non-negative")
	}
	if ttl == 0 {
		return 0, nil
	}
	return now + ttl.Nanoseconds(), nil
}
func (e *Entry) IsExpired() bool            { return e.isExpiredAt(time.Now().UnixNano()) }
func (e *Entry) IsExpiredAt(now int64) bool { return e.isExpiredAt(now) }
func (e *Entry) isExpiredAt(now int64) bool {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	return exp != 0 && now > exp
}

func (e *Entry) Touch() {
	atomic.StoreInt64(&e.AccessAt, time.Now().UnixNano())
	atomic.AddInt64(&e.Hits, 1)
}

func (e *Entry) TTLRemaining() time.Duration {
	exp := atomic.LoadInt64(&e.ExpiresAt)
	if exp == 0 {
		return 0
	}
	return time.Duration(exp - time.Now().UnixNano())
}

func (e *Entry) ShouldEarlyRefresh(beta float64) bool {
	softExp := atomic.LoadInt64(&e.SoftExpiresAt)
	hardExp := atomic.LoadInt64(&e.ExpiresAt)
	if softExp == 0 || hardExp == 0 {
		return false
	}
	now := time.Now().UnixNano()
	if now >= softExp {
		return true
	}
	delta := float64(hardExp - softExp)
	t := float64(softExp - now)
	return t-delta*beta*math.Log(rand.Float64()) <= 0
}
func (e *Entry) GetAccessAt() int64       { return atomic.LoadInt64(&e.AccessAt) }
func (e *Entry) GetCreatedAt() int64      { return atomic.LoadInt64(&e.CreatedAt) }
func (e *Entry) GetExpiresAt() int64      { return atomic.LoadInt64(&e.ExpiresAt) }
func (e *Entry) GetSoftExpiresAt() int64  { return atomic.LoadInt64(&e.SoftExpiresAt) }
func (e *Entry) GetHits() int64           { return atomic.LoadInt64(&e.Hits) }
func (e *Entry) GetFrequency() int64      { return atomic.LoadInt64(&e.Frequency) }
func (e *Entry) GetRecency() int64        { return atomic.LoadInt64(&e.Recency) }
func (e *Entry) GetRefreshCount() int64   { return atomic.LoadInt64(&e.RefreshCount) }
func (e *Entry) IncrRefreshCount() int64  { return atomic.AddInt64(&e.RefreshCount, 1) }
func (e *Entry) HitsCount() int64         { return e.GetHits() }
func (e *Entry) FrequencyCount() int64    { return e.GetFrequency() }
func (e *Entry) SetFrequency(freq int64)  { atomic.StoreInt64(&e.Frequency, freq) }
func (e *Entry) SetRecency(recency int64) { atomic.StoreInt64(&e.Recency, recency) }
func (e *Entry) IncrFrequency() int64     { return atomic.AddInt64(&e.Frequency, 1) }
func (e *Entry) IncrHits() int64          { return atomic.AddInt64(&e.Hits, 1) }
