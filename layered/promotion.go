package layered

import (
	"context"
	"time"
)

// promoteToL1 writes a value to the L1 cache. This is called when a value
// is found in L2 and should be promoted to L1 for faster future access.
// Promotion errors are silently ignored to avoid degrading the read path.
func (s *Store) promoteToL1(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if value == nil {
		// Negative cache entry - use a sentinel value
		value = []byte{0}
	}
	if err := s.l1.Set(ctx, key, value, ttl); err != nil {
		s.stats.L1Error()
	}
	s.stats.L2Promotion()
}

// resolveTTLFromL2 queries L2 for the remaining TTL of a key and returns it.
// If the TTL cannot be determined, a minimal TTL of 100ms is used to prevent
// L1 from caching stale entries longer than L2.
func (s *Store) resolveTTLFromL2(ctx context.Context, key string) time.Duration {
	ttl, err := s.l2.TTL(ctx, key)
	if err != nil || ttl <= 0 {
		// Use minimal TTL to prevent L1 staleness
		return 100 * time.Millisecond
	}
	return ttl
}
