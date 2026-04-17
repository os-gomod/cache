package layer

import (
	"context"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/observability"
)

// Get retrieves the value for key, trying L1 first then falling back to L2.
// On L2 hits, the value may be promoted to L1 if promotion is enabled.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := c.checkClosed("layered.get"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "layered", Name: "get", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if key == "" {
		return nil, cacheerrors.EmptyKey("layered.get")
	}
	select {
	case <-ctx.Done():
		return nil, cacheerrors.CancelledError("layered.get")
	default:
	}
	if cachectx.ShouldBypassCache(ctx) {
		c.stats.Miss()
		return nil, cacheerrors.NotFound("layered.get", key)
	}
	c.stats.RecordGet()
	if val, err := c.l1.Get(ctx, key); err == nil {
		if cachectx.IsNegativeValue(val) {
			c.stats.L1Hit()
			return nil, cacheerrors.NotFound("layered.get", key)
		}
		c.stats.L1Hit()
		result.Hit = true
		result.ByteSize = len(val)
		return val, nil
	}
	c.stats.L1Miss()
	select {
	case <-ctx.Done():
		return nil, cacheerrors.CancelledError("layered.get")
	default:
	}
	val, err := c.l2.Get(ctx, key)
	if err != nil {
		if cacheerrors.IsNotFound(err) {
			c.stats.L2Miss()
			c.stats.Miss()
			if c.cfg.NegativeTTL > 0 {
				_ = c.l1.Set(ctx, key, cachectx.NewNegativeValue(), c.cfg.NegativeTTL)
			}
			return nil, cacheerrors.NotFound("layered.get", key)
		}
		c.stats.L2Error()
		result.Err = err
		return nil, cacheerrors.WrapKey("layered.l2_get", key, err)
	}
	c.stats.L2Hit()
	result.Hit = true
	result.ByteSize = len(val)
	if c.cfg.PromoteOnHit && !c.skipPromotion(key) {
		if ttl, ok := c.promotionTTL(ctx, key); ok {
			_ = c.l1.Set(ctx, key, val, ttl)
			c.stats.L2Promotion()
		}
	}
	return val, nil
}

// Set stores a key-value pair in both L1 and L2.
// When write-back is enabled, the L2 write is deferred.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.checkClosed("layered.set"); err != nil {
		return err
	}
	op := observability.Op{Backend: "layered", Name: "set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if key == "" {
		return cacheerrors.EmptyKey("layered.set")
	}
	if ttl == 0 {
		ttl = c.cfg.L2Config.DefaultTTL
	}
	l1TTL := ttl
	if c.cfg.L1Config.DefaultTTL > 0 && (ttl == 0 || ttl > c.cfg.L1Config.DefaultTTL) {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	if err := c.l1.Set(ctx, key, value, l1TTL); err != nil {
		c.stats.ErrorOp()
	}
	if c.cfg.WriteBack {
		select {
		case c.wbCh <- wbJob{key: key, value: value, ttl: ttl}:
			c.stats.WriteBackEnqueue()
			return nil
		default:
			c.stats.WriteBackDrop()
			result.Err = cacheerrors.ErrCacheFull
			return cacheerrors.ErrCacheFull
		}
	}
	if err := c.l2.Set(ctx, key, value, ttl); err != nil {
		result.Err = err
		return cacheerrors.WrapKey("layered.l2_set", key, err)
	}
	c.stats.SetOp()
	return nil
}

// Delete removes a key from both L1 and L2.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed("layered.delete"); err != nil {
		return err
	}
	op := observability.Op{Backend: "layered", Name: "delete", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	_ = c.l1.Delete(ctx, key)
	if err := c.l2.Delete(ctx, key); err != nil && !cacheerrors.IsNotFound(err) {
		result.Err = err
		return cacheerrors.WrapKey("layered.delete", key, err)
	}
	c.stats.DeleteOp()
	return nil
}

// Exists checks L1 then L2 for key existence.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("layered.exists"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "layered", Name: "exists", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if ok, err := c.l1.Exists(ctx, key); ok || err != nil {
		result.Hit = ok
		return ok, err
	}
	ok, err := c.l2.Exists(ctx, key)
	result.Hit = ok
	return ok, err
}

// TTL returns the remaining time-to-live from L2 for the given key.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("layered.ttl"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "layered", Name: "ttl", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	d, err := c.l2.TTL(ctx, key)
	if err != nil {
		result.Err = err
	}
	return d, err
}

// Keys returns all keys matching the pattern from L2.
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := c.checkClosed("layered.keys"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "layered", Name: "keys"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	keys, err := c.l2.Keys(ctx, pattern)
	if err != nil {
		result.Err = err
	}
	return keys, err
}

// Size returns the approximate number of unique keys across L1 and L2.
func (c *Cache) Size(ctx context.Context) (int64, error) {
	if err := c.checkClosed("layered.size"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "layered", Name: "size"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
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

// Clear removes all entries from both L1 and L2.
func (c *Cache) Clear(ctx context.Context) error {
	if err := c.checkClosed("layered.clear"); err != nil {
		return err
	}
	op := observability.Op{Backend: "layered", Name: "clear"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if err := c.l2.Clear(ctx); err != nil {
		result.Err = err
		return err
	}
	return c.l1.Clear(ctx)
}
