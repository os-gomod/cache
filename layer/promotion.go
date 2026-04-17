package layer

import (
	"context"
	"time"
)

func (c *Cache) promotionTTL(ctx context.Context, key string) (time.Duration, bool) {
	if c.cfg.L1TTLOverride > 0 {
		return c.cfg.L1TTLOverride, true
	}
	l1Default := c.cfg.L1Config.DefaultTTL
	remaining, err := c.l2.TTL(ctx, key)
	if err != nil {
		return l1Default, true
	}
	if remaining < 0 {
		return l1Default, true
	}
	if remaining == 0 {
		return 0, false
	}
	if l1Default > 0 && remaining > l1Default {
		return l1Default, true
	}
	return remaining, true
}

func (c *Cache) skipPromotion(key string) bool {
	_, skip := c.noPromote.LoadAndDelete(key)
	return skip
}
