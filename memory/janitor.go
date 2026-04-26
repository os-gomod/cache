package memory

import (
	"time"
)

// janitor is a background goroutine that periodically scans all shards and
// removes expired entries. It runs at the configured cleanup interval and
// stops when the store is closed.
func (s *Store) janitor() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanupExpired()
		}
	}
}

// cleanupExpired scans all shards and removes expired entries.
func (s *Store) cleanupExpired() {
	now := time.Now().UnixNano()
	var totalEvicted int64

	for _, sh := range s.shards {
		sh.mu.Lock()
		for k, e := range sh.items {
			if e.IsExpired(now) {
				delete(sh.items, k)
				sh.size -= e.Size
				sh.count--
				totalEvicted++
			}
		}
		sh.mu.Unlock()
	}

	if totalEvicted > 0 {
		s.stats.EvictionOp()
		s.stats.AddItems(-totalEvicted)
	}
}
