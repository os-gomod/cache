package layered

import (
	"context"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// GetMulti retrieves multiple values.
func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := c.checkClosed("layered.get_multi"); err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(keys))
	var missing []string
	for _, k := range keys {
		if v, err := c.Get(ctx, k); err == nil {
			out[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	l2vals, err := c.l2.GetMulti(ctx, missing...)
	if err != nil {
		return out, _errors.Wrap("layered.get_multi", err)
	}
	for k, v := range l2vals {
		out[k] = v
	}
	return out, nil
}

// SetMulti stores multiple values.
func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if err := c.checkClosed("layered.set_multi"); err != nil {
		return err
	}
	for k, v := range items {
		if err := c.Set(ctx, k, v, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMulti removes multiple keys.
func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("layered.delete_multi"); err != nil {
		return err
	}
	for _, k := range keys {
		if err := c.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

// GetOrSet returns the cached value or computes and stores it.
func (c *Cache) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("layered.get_or_set"); err != nil {
		return nil, err
	}
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	}
	return c.sg.Do(ctx, key, func() ([]byte, error) {
		if val, err := c.Get(ctx, key); err == nil {
			return val, nil
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		_ = c.Set(ctx, key, val, ttl)
		return val, nil
	})
}

// GetSet atomically sets key to value and returns the old value (L2 is authoritative).
func (c *Cache) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("layered.getset"); err != nil {
		return nil, err
	}
	oldVal, err := c.l2.GetSet(ctx, key, value, ttl)
	if err != nil && !_errors.IsNotFound(err) {
		return nil, _errors.WrapKey("layered.getset_l2", key, err)
	}
	l1TTL := ttl
	if ttl == 0 {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	_ = c.l1.Set(ctx, key, value, l1TTL)
	c.stats.SetOp()
	return oldVal, nil
}

// CompareAndSwap atomically replaces a value in L2 and invalidates L1.
func (c *Cache) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("layered.cas"); err != nil {
		return false, err
	}
	swapped, err := c.l2.CompareAndSwap(ctx, key, oldVal, newVal, ttl)
	if err != nil {
		return false, _errors.WrapKey("layered.cas", key, err)
	}
	if swapped {
		_ = c.l1.Delete(ctx, key)
		c.noPromote.Store(key, struct{}{})
	}
	return swapped, nil
}

// Increment atomically increments a counter stored at key in L2.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("layered.increment"); err != nil {
		return 0, err
	}
	v, err := c.l2.Increment(ctx, key, delta)
	if err != nil {
		return 0, _errors.WrapKey("layered.increment", key, err)
	}
	_ = c.l1.Delete(ctx, key)
	return v, nil
}

// Decrement atomically decrements a counter stored at key in L2.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.Increment(ctx, key, -delta)
}

// SetNX sets key to value only if it does not already exist (L2 is authoritative).
func (c *Cache) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("layered.setnx"); err != nil {
		return false, err
	}
	exists, err := c.l2.Exists(ctx, key)
	if err != nil {
		return false, _errors.WrapKey("layered.setnx_exists", key, err)
	}
	if exists {
		return false, nil
	}
	if ttl == 0 {
		ttl = c.cfg.L2Config.DefaultTTL
	}
	if err = c.l2.Set(ctx, key, value, ttl); err != nil {
		return false, _errors.WrapKey("layered.setnx_l2", key, err)
	}
	l1TTL := ttl
	if c.cfg.L1Config.DefaultTTL > 0 && (ttl == 0 || ttl > c.cfg.L1Config.DefaultTTL) {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	_ = c.l1.Set(ctx, key, value, l1TTL)
	c.stats.SetOp()
	return true, nil
}
