// Package clock provides a shared monotonic nanosecond clock updated by a
// background goroutine.  Callers use Now() in hot paths to avoid repeated
// syscalls; the goroutine keeps the shared value fresh every millisecond.
package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	nowAtomic int64
	startOnce sync.Once
)

func init() { StartClock() }

func storeMax(ts int64) int64 {
	for {
		current := atomic.LoadInt64(&nowAtomic)
		if ts <= current {
			return current
		}
		if atomic.CompareAndSwapInt64(&nowAtomic, current, ts) {
			return ts
		}
	}
}

// Now returns the current Unix nanosecond timestamp.
// The value is refreshed every millisecond by the background ticker; it is
// always at most ~1 ms stale.
func Now() int64 {
	return storeMax(time.Now().UnixNano())
}

// StartClock starts the background ticker exactly once.  The init() call above
// ensures it runs before any package-level variable is read, but callers may
// invoke StartClock() explicitly in tests that need a fresh clock.
func StartClock() {
	startOnce.Do(func() {
		storeMax(time.Now().UnixNano())
		go func() {
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for t := range ticker.C {
				storeMax(t.UnixNano())
			}
		}()
	})
}
