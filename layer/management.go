package layer

import (
	"context"

	cacheerrors "github.com/os-gomod/cache/errors"
)

// InvalidateL1 removes the given keys from the L1 cache, forcing subsequent reads from L2.
func (c *Cache) InvalidateL1(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("layered.invalidate_l1"); err != nil {
		return err
	}
	return c.l1.DeleteMulti(ctx, keys...)
}

// Refresh re-fetches the given keys from L2 and populates L1.
func (c *Cache) Refresh(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("layered.refresh"); err != nil {
		return err
	}
	values, err := c.l2.GetMulti(ctx, keys...)
	if err != nil {
		return cacheerrors.Wrap("layered.refresh", err)
	}
	for k, v := range values {
		if ttl, ok := c.promotionTTL(ctx, k); ok {
			_ = c.l1.Set(ctx, k, v, ttl)
		}
	}
	return nil
}
