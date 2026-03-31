package layered

import (
	"context"
	"time"
)

// promotionTTL returns the TTL to use when promoting an L2 hit into L1.
// Returns (ttl, true) when promotion should proceed, (0, false) to skip it.
func (c *Cache) promotionTTL(ctx context.Context, key string) (time.Duration, bool) {
	// L1TTLOverride takes absolute precedence when set.
	if c.cfg.L1TTLOverride > 0 {
		return c.cfg.L1TTLOverride, true
	}

	l1Default := c.cfg.L1Config.DefaultTTL

	remaining, err := c.l2.TTL(ctx, key)
	if err != nil {
		// Could not determine L2 TTL; promote using L1 default.
		return l1Default, true
	}

	// Redis returns -1 (as a duration) for persistent keys.
	if remaining < 0 {
		return l1Default, true
	}

	// Skip promotion when the L2 entry is already at or past expiry.
	if remaining == 0 {
		return 0, false
	}

	// Cap by L1 default TTL; use whichever is shorter.
	if l1Default > 0 && remaining > l1Default {
		return l1Default, true
	}
	return remaining, true
}

// skipPromotion reports whether promotion should be suppressed for key and
// clears the suppress flag so subsequent hits are promoted normally.
func (c *Cache) skipPromotion(key string) bool {
	_, skip := c.noPromote.LoadAndDelete(key)
	return skip
}
