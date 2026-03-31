package layered

import (
	"context"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/internal/obs"
)

// Get retrieves a value, trying L1 first then L2.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	_, span := obs.Start(ctx, "layered.get")
	defer span.End()

	if err := c.checkClosed("layered.get"); err != nil {
		span.SetError(err)
		return nil, err
	}
	if key == "" {
		return nil, _errors.EmptyKey("layered.get")
	}
	select {
	case <-ctx.Done():
		return nil, _errors.CancelledError("layered.get")
	default:
	}
	if cachectx.ShouldBypassCache(ctx) {
		c.stats.Miss()
		obs.RecordMiss(ctx, "layered", "get", 0)
		return nil, _errors.NotFound("layered.get", key)
	}

	c.stats.GetOp()

	// L1 probe
	if val, err := c.l1.Get(ctx, key); err == nil {
		if cachectx.IsNegativeValue(val) {
			c.stats.L1Hit()
			obs.RecordHit(ctx, "layered.l1", "get", 0)
			return nil, _errors.NotFound("layered.get", key)
		}
		c.stats.L1Hit()
		obs.RecordHit(ctx, "layered.l1", "get", 0)
		return val, nil
	}
	c.stats.L1Miss()

	select {
	case <-ctx.Done():
		return nil, _errors.CancelledError("layered.get")
	default:
	}

	// L2 probe
	val, err := c.l2.Get(ctx, key)
	if err != nil {
		if _errors.IsNotFound(err) {
			c.stats.L2Miss()
			c.stats.Miss()
			obs.RecordMiss(ctx, "layered", "get", 0)
			if c.cfg.NegativeTTL > 0 {
				_ = c.l1.Set(ctx, key, cachectx.NewNegativeValue(), c.cfg.NegativeTTL)
			}
			return nil, _errors.NotFound("layered.get", key)
		}
		c.stats.L2Error()
		obs.RecordError(ctx, "layered.l2", "get")
		span.SetError(err)
		return nil, _errors.WrapKey("layered.l2_get", key, err)
	}

	c.stats.L2Hit()
	obs.RecordHit(ctx, "layered.l2", "get", 0)

	if c.cfg.PromoteOnHit && !c.skipPromotion(key) {
		if ttl, ok := c.promotionTTL(ctx, key); ok {
			_ = c.l1.Set(ctx, key, val, ttl)
			c.stats.L2Promotion()
		}
	}
	return val, nil
}

// Set writes a value to L1 immediately and to L2 (sync or async write-back).
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.checkClosed("layered.set"); err != nil {
		return err
	}
	if key == "" {
		return _errors.EmptyKey("layered.set")
	}
	if ttl == 0 {
		ttl = c.cfg.L2Config.DefaultTTL
	}

	l1TTL := ttl
	if c.cfg.L1Config.DefaultTTL > 0 && (ttl == 0 || ttl > c.cfg.L1Config.DefaultTTL) {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	_ = c.l1.Set(ctx, key, value, l1TTL)

	if c.cfg.WriteBack {
		return c.enqueueWriteBack(key, value, ttl)
	}
	if err := c.l2.Set(ctx, key, value, ttl); err != nil {
		return _errors.WrapKey("layered.l2_set", key, err)
	}
	c.stats.SetOp()
	return nil
}

// Delete removes a key from both layers.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed("layered.delete"); err != nil {
		return err
	}
	_ = c.l1.Delete(ctx, key)
	if err := c.l2.Delete(ctx, key); err != nil && !_errors.IsNotFound(err) {
		return _errors.WrapKey("layered.delete", key, err)
	}
	c.stats.DeleteOp()
	return nil
}

// Exists reports whether key is live in either layer.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("layered.exists"); err != nil {
		return false, err
	}
	if ok, err := c.l1.Exists(ctx, key); ok || err != nil {
		return ok, err
	}
	return c.l2.Exists(ctx, key)
}

// TTL returns the remaining TTL for key (L2 is the authoritative source).
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("layered.ttl"); err != nil {
		return 0, err
	}
	return c.l2.TTL(ctx, key)
}

// Keys returns keys from L2.
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	_ = ctx
	if err := c.checkClosed("layered.keys"); err != nil {
		return nil, err
	}
	return c.l2.Keys(ctx, pattern)
}

// Size returns combined size.
func (c *Cache) Size(ctx context.Context) (int64, error) {
	if err := c.checkClosed("layered.size"); err != nil {
		return 0, err
	}

	keys := make(map[string]struct{})

	l1Keys, _ := c.l1.Keys(ctx, "*")
	for _, k := range l1Keys {
		keys[k] = struct{}{}
	}

	l2Keys, _ := c.l2.Keys(ctx, "*")
	for _, k := range l2Keys {
		keys[k] = struct{}{}
	}

	return int64(len(keys)), nil
}

// Clear removes all entries.
func (c *Cache) Clear(ctx context.Context) error {
	if err := c.checkClosed("layered.clear"); err != nil {
		return err
	}
	if err := c.l2.Clear(ctx); err != nil {
		return err
	}
	return c.l1.Clear(ctx)
}
