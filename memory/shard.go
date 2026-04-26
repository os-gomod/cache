package memory

import (
	"strings"
	"sync"
	"time"
)

// evicted holds the results of an eviction operation within a shard.
type evicted struct {
	keys  []string // keys that were evicted
	bytes int64    // total bytes freed by eviction
}

// shard is a single partition of the sharded memory cache, protected by its
// own RWMutex. Each shard independently tracks its items, total size, and
// item count.
type shard struct {
	mu    sync.RWMutex
	items map[string]*Entry
	size  int64 // total bytes of all values
	count int64 // total number of items
}

// newShard creates a new shard with an initial map capacity hint.
func newShard(initialCap int) *shard {
	if initialCap <= 0 {
		initialCap = 64
	}
	return &shard{
		items: make(map[string]*Entry, initialCap),
	}
}

// get retrieves an entry by key. The caller must hold at least a read lock.
func (s *shard) get(key string) (*Entry, bool) {
	e, ok := s.items[key]
	return e, ok
}

// set stores an entry under the given key. If maxSize > 0 and adding the entry
// would exceed maxSize, the oldest entries are evicted to make room.
// The caller must hold a write lock.
func (s *shard) set(key string, e *Entry, maxSize int64) evicted {
	// Update case: remove old entry first
	if old, exists := s.items[key]; exists {
		s.size -= old.Size
		s.count--
	}

	// Evict if necessary to make room
	var ev evicted
	if maxSize > 0 && s.size+e.Size > maxSize {
		ev = s.evictToMakeRoom(e.Size, maxSize)
	}

	s.items[key] = e
	s.size += e.Size
	s.count++
	return ev
}

// delete removes an entry by key and returns it. The caller must hold a write
// lock.
func (s *shard) delete(key string) (*Entry, bool) {
	e, ok := s.items[key]
	if !ok {
		return nil, false
	}
	delete(s.items, key)
	s.size -= e.Size
	s.count--
	return e, true
}

// keys returns all non-expired keys in the shard that match the given glob
// pattern. An empty pattern matches all keys. The caller must hold at least a
// read lock.
func (s *shard) keys(pattern string) []string {
	if len(s.items) == 0 {
		return nil
	}
	now := time.Now().UnixNano()
	result := make([]string, 0, len(s.items))
	for k, e := range s.items {
		if e.IsExpired(now) {
			continue
		}
		if pattern == "" || matchGlob(pattern, k) {
			result = append(result, k)
		}
	}
	return result
}

// clear removes all entries from the shard and resets its size and count.
// The caller must hold a write lock.
func (s *shard) clear() {
	s.items = make(map[string]*Entry, 64)
	s.size = 0
	s.count = 0
}

// totalSize returns the current item count and total byte size of the shard.
//
//nolint:nonamedreturns // named returns clarify the order for same-type return values
func (s *shard) totalSize() (count, size int64) {
	return s.count, s.size
}

// evictToMakeRoom evicts entries until the shard has enough room for the
// required bytes. Uses a simple LRU-like strategy (oldest LastAccess first).
// The caller must hold a write lock.
func (s *shard) evictToMakeRoom(required, maxSize int64) evicted {
	var ev evicted
	now := time.Now().UnixNano()

	for k, e := range s.items {
		if !e.IsExpired(now) {
			continue
		}
		delete(s.items, k)
		s.size -= e.Size
		s.count--
		ev.keys = append(ev.keys, k)
		ev.bytes += e.Size
	}

	// Second pass: evict oldest-accessed entries until we have room
	for s.size+required > maxSize && len(s.items) > 0 {
		var oldestKey string
		var oldestAccess int64 = 1<<63 - 1
		for k, e := range s.items {
			if e.LastAccess < oldestAccess {
				oldestKey = k
				oldestAccess = e.LastAccess
			}
		}
		if oldestKey == "" {
			break
		}
		if removed, ok := s.items[oldestKey]; ok {
			delete(s.items, oldestKey)
			s.size -= removed.Size
			s.count--
			ev.keys = append(ev.keys, oldestKey)
			ev.bytes += removed.Size
		}
	}
	return ev
}

// matchGlob performs simple glob matching. The pattern supports a single '*'
// wildcard that matches any sequence of characters.
func matchGlob(pattern, s string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	idx := strings.IndexByte(pattern, '*')
	prefix := pattern[:idx]
	suffix := pattern[idx+1:]
	if prefix != "" && !strings.HasPrefix(s, prefix) {
		return false
	}
	if suffix != "" && !strings.HasSuffix(s, suffix) {
		return false
	}
	return len(s) >= len(prefix)+len(suffix)
}
