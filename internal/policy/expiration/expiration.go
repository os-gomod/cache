// Package expiration provides TTL management and expiration tracking for
// cache entries. It integrates with store backends to manage time-to-live
// semantics and provide expiration queries.
package expiration

import (
	"time"

	"github.com/os-gomod/cache/v2/memory"
)

// Engine manages TTL tracking and expiration queries for cache entries.
// It provides methods to check expiration status and calculate remaining
// TTL without being tied to a specific storage backend.
type Engine struct{}

// New creates a new expiration Engine.
func New() *Engine {
	return &Engine{}
}

// IsExpired reports whether the given entry has expired relative to the
// current time. An entry with Expiry == 0 never expires.
func (*Engine) IsExpired(entry *memory.Entry, now int64) bool {
	return entry.IsExpired(now)
}

// IsExpiredNow reports whether the given entry has expired relative to
// time.Now(). This is a convenience wrapper around IsExpired.
func (*Engine) IsExpiredNow(entry *memory.Entry) bool {
	return entry.IsExpired(time.Now().UnixNano())
}

// RemainingTTL returns the remaining time-to-live for the entry relative
// to the given timestamp. If the entry has no expiration (Expiry == 0),
// a duration greater than 1 year is returned. If expired, zero is returned.
func (*Engine) RemainingTTL(entry *memory.Entry, now int64) time.Duration {
	return entry.TTL(now)
}

// RemainingTTLNow returns the remaining time-to-live relative to time.Now().
func (*Engine) RemainingTTLNow(entry *memory.Entry) time.Duration {
	return entry.TTL(time.Now().UnixNano())
}

// CalculateExpiry computes the absolute Unix nano expiration timestamp
// from the given TTL and creation time.
func (*Engine) CalculateExpiry(ttl time.Duration, createdAt int64) int64 {
	if ttl <= 0 {
		return 0 // no expiration
	}
	return createdAt + int64(ttl)
}

// FormatTTL formats a duration as a human-readable string for logging
// and debugging purposes.
func (*Engine) FormatTTL(d time.Duration) string {
	switch {
	case d <= 0:
		return "expired"
	case d < time.Second:
		return "<1s"
	case d < time.Minute:
		return d.Round(time.Millisecond).String()
	case d < time.Hour:
		return d.Round(time.Second).String()
	default:
		return d.Round(time.Minute).String()
	}
}
