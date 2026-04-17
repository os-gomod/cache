package layer

import (
	"context"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

// GetMulti retrieves values for multiple keys, checking L1 first then L2 for misses.
func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := c.checkClosed("layered.get_multi"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "layered", Name: "get_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
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
		result.Hit = true
		return out, nil
	}
	l2vals, err := c.l2.GetMulti(ctx, missing...)
	if err != nil {
		result.Err = err
		return out, cacheerrors.Wrap("layered.get_multi", err)
	}
	for k, v := range l2vals {
		out[k] = v
	}
	if len(out) > 0 {
		result.Hit = true
	}
	return out, nil
}

// SetMulti stores multiple key-value pairs in both L1 and L2.
func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if err := c.checkClosed("layered.set_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "layered", Name: "set_multi", KeyCount: len(items)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	for k, v := range items {
		if err := c.Set(ctx, k, v, ttl); err != nil {
			result.Err = err
			return err
		}
	}
	return nil
}

// DeleteMulti removes multiple keys from both L1 and L2.
func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("layered.delete_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "layered", Name: "delete_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	for _, k := range keys {
		if err := c.Delete(ctx, k); err != nil {
			result.Err = err
			return err
		}
	}
	return nil
}

// GetOrSet retrieves the value for key, or calls fn to compute, cache, and return it.
func (c *Cache) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("layered.get_or_set"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "layered", Name: "get_or_set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if val, err := c.Get(ctx, key); err == nil {
		result.Hit = true
		result.ByteSize = len(val)
		return val, nil
	}
	return c.detector.Do(ctx, key, nil, nil,
		func(ctx context.Context) ([]byte, error) {
			if val, err := c.Get(ctx, key); err == nil {
				return val, nil
			}
			val, err := fn()
			if err != nil {
				return nil, err
			}
			_ = c.Set(ctx, key, val, ttl)
			return val, nil
		},
		nil,
	)
}

// GetSet sets the value for a key and returns the previous value.
func (c *Cache) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("layered.getset"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "layered", Name: "getset", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	oldVal, err := c.l2.GetSet(ctx, key, value, ttl)
	if err != nil && !cacheerrors.IsNotFound(err) {
		result.Err = err
		return nil, cacheerrors.WrapKey("layered.getset_l2", key, err)
	}
	l1TTL := ttl
	if ttl == 0 {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	_ = c.l1.Set(ctx, key, value, l1TTL)
	c.stats.SetOp()
	if oldVal != nil {
		result.ByteSize = len(oldVal)
	}
	return oldVal, nil
}

// CompareAndSwap atomically sets key to newVal if its current value equals oldVal.
func (c *Cache) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("layered.cas"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "layered", Name: "cas", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	swapped, err := c.l2.CompareAndSwap(ctx, key, oldVal, newVal, ttl)
	if err != nil {
		result.Err = err
		return false, cacheerrors.WrapKey("layered.cas", key, err)
	}
	if swapped {
		_ = c.l1.Delete(ctx, key)
		c.noPromote.Store(key, struct{}{})
	}
	return swapped, nil
}

// Increment atomically adds delta to the integer value stored at key in L2.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("layered.increment"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "layered", Name: "increment", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	v, err := c.l2.Increment(ctx, key, delta)
	if err != nil {
		result.Err = err
		return 0, cacheerrors.WrapKey("layered.increment", key, err)
	}
	_ = c.l1.Delete(ctx, key)
	return v, nil
}

// Decrement atomically subtracts delta from the integer value stored at key in L2.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.Increment(ctx, key, -delta)
}

// SetNX sets the key-value pair in both L1 and L2 only if the key does not already exist.
func (c *Cache) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("layered.setnx"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "layered", Name: "setnx", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if ttl == 0 {
		ttl = c.cfg.L2Config.DefaultTTL
	}
	set, err := c.l2.SetNX(ctx, key, value, ttl)
	if err != nil {
		result.Err = err
		return false, cacheerrors.WrapKey("layered.setnx_l2", key, err)
	}
	if !set {
		return false, nil
	}
	l1TTL := ttl
	if c.cfg.L1Config.DefaultTTL > 0 && (ttl == 0 || ttl > c.cfg.L1Config.DefaultTTL) {
		l1TTL = c.cfg.L1Config.DefaultTTL
	}
	_ = c.l1.Set(ctx, key, value, l1TTL)
	c.stats.SetOp()
	return true, nil
}
