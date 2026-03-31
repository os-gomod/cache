package memory

import (
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/eviction"
	"github.com/os-gomod/cache/internal/clock"
)

// ----------------------------------------------------------------------------
// Capacity enforcement
// ----------------------------------------------------------------------------

func (c *Cache) enforceCapacity(s *shard, protectedKey string) {
	for {
		overItems := c.cfg.MaxEntries > 0 && c.stats.Items() > int64(c.cfg.MaxEntries)
		overSize := c.maxSize > 0 && c.stats.Memory() > c.maxSize
		if !overItems && !overSize {
			break
		}
		victimShard, victimKey := c.selectVictim(s, protectedKey)
		if victimShard == nil || victimKey == "" {
			break
		}

		if victimShard != s {
			victimShard.mu.Lock()
		}
		e, ok := victimShard.items[victimKey]
		if !ok {
			if victimShard != s {
				victimShard.mu.Unlock()
			}
			continue
		}
		delete(victimShard.items, victimKey)
		victimShard.evict.OnDelete(victimKey)
		victimShard.size -= e.Size
		victimShard.count--
		if victimShard != s {
			victimShard.mu.Unlock()
		}

		c.stats.EvictionOp()
		c.stats.AddItems(-1)
		c.stats.AddMemory(-e.Size)

		if c.cfg.OnEvictionPolicy != nil {
			c.cfg.OnEvictionPolicy(victimKey, "capacity")
		}
	}
}

func (c *Cache) selectVictim(lockedShard *shard, protectedKey string) (*shard, string) {
	now := clock.Now()
	var victimShard *shard
	var victimKey string
	var victimEntry *eviction.Entry

	for _, s := range c.shards {
		if s != lockedShard {
			s.mu.RLock()
		}
		for key, e := range s.items {
			if key == protectedKey {
				continue
			}
			if e.IsExpiredAt(now) {
				if s != lockedShard {
					s.mu.RUnlock()
				}
				return s, key
			}
			if victimEntry == nil || c.shouldReplaceVictim(e, victimEntry) {
				victimShard = s
				victimKey = key
				victimEntry = e
			}
		}
		if s != lockedShard {
			s.mu.RUnlock()
		}
	}
	return victimShard, victimKey
}

func (c *Cache) shouldReplaceVictim(candidate, current *eviction.Entry) bool {
	switch c.cfg.EvictionPolicy {
	case config.EvictLFU, config.EvictTinyLFU:
		if candidate.GetHits() != current.GetHits() {
			return candidate.GetHits() < current.GetHits()
		}
		return candidate.GetCreatedAt() < current.GetCreatedAt()
	case config.EvictLIFO:
		return candidate.GetCreatedAt() > current.GetCreatedAt()
	case config.EvictMRU:
		return candidate.GetAccessAt() > current.GetAccessAt()
	case config.EvictFIFO:
		return candidate.GetCreatedAt() < current.GetCreatedAt()
	default: // LRU / Random
		return candidate.GetAccessAt() < current.GetAccessAt()
	}
}

// deleteFromShard removes key from s under s.mu (caller must hold s.mu.Lock).
func (c *Cache) deleteFromShard(s *shard, key string) {
	e, ok := s.items[key]
	if !ok {
		return
	}
	delete(s.items, key)
	s.evict.OnDelete(key)
	s.size -= e.Size
	s.count--
	c.stats.DeleteOp()
	c.stats.AddItems(-1)
	c.stats.AddMemory(-e.Size)
}

// ----------------------------------------------------------------------------
// Janitor (background expiration sweep)
// ----------------------------------------------------------------------------

func (c *Cache) janitor() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sweepExpired()
		}
	}
}

func (c *Cache) sweepExpired() {
	now := clock.Now()
	for _, s := range c.shards {
		s.mu.Lock()
		for key, e := range s.items {
			if !e.IsExpiredAt(now) {
				continue
			}
			delete(s.items, key)
			s.evict.OnDelete(key)
			s.size -= e.Size
			s.count--
			c.stats.EvictionOp()
			c.stats.AddItems(-1)
			c.stats.AddMemory(-e.Size)
			if c.cfg.OnEvictionPolicy != nil {
				c.cfg.OnEvictionPolicy(key, "expired")
			}
		}
		s.mu.Unlock()
	}
}
