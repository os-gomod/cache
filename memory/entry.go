package memory

import "time"

// Entry represents a single cached item in the memory store. It tracks the
// value, expiration time, size, access metadata, and frequency counters used
// by eviction policies.
type Entry struct {
	// Key is the cache key (used for debugging and eviction).
	Key string

	// Value is the cached data.
	Value []byte

	// Expiry is the Unix nanosecond timestamp when the entry expires.
	// A value of 0 means the entry never expires.
	Expiry int64

	// Size is the byte length of Value.
	Size int64

	// LastAccess is the Unix nanosecond timestamp of the last access
	// (read or write).
	LastAccess int64

	// Frequency is the number of times this entry has been accessed.
	// Used by LFU-like eviction policies.
	Frequency int64
}

// NewEntry creates a new Entry with the given key, value, and TTL.
//
// Parameters:
//   - key: the cache key
//   - value: the cached value (must not be nil)
//   - ttl: time-to-live; a zero or negative value means the entry never expires
//   - now: current Unix nanosecond timestamp
func NewEntry(key string, value []byte, ttl time.Duration, now int64) *Entry {
	e := &Entry{
		Key:        key,
		Value:      value,
		Expiry:     0,
		Size:       int64(len(value)),
		LastAccess: now,
		Frequency:  1,
	}
	if ttl > 0 {
		e.Expiry = now + int64(ttl)
	}
	return e
}

// IsExpired reports whether the entry has expired relative to the given
// timestamp.
func (e *Entry) IsExpired(now int64) bool {
	return e.Expiry > 0 && now >= e.Expiry
}

// Touch updates the entry's LastAccess timestamp and increments Frequency.
func (e *Entry) Touch(now int64) {
	e.LastAccess = now
	e.Frequency++
}

// TTL returns the remaining time-to-live of the entry relative to the given
// timestamp. If the entry has no expiration (Expiry == 0), it returns a
// duration greater than 1 year. If the entry is expired, it returns a
// negative or zero duration.
func (e *Entry) TTL(now int64) time.Duration {
	if e.Expiry == 0 {
		return 365 * 24 * time.Hour
	}
	remaining := e.Expiry - now
	if remaining <= 0 {
		return 0
	}
	return time.Duration(remaining)
}
