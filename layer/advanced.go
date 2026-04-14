package layer

import (
	"context"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

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
		return out, _errors.Wrap("layered.get_multi", err)
	}
	for k, v := range l2vals {
		out[k] = v
	}
	if len(out) > 0 {
		result.Hit = true
	}
	return out, nil
}

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

	// Try L1 first.
	if val, err := c.Get(ctx, key); err == nil {
		result.Hit = true
		result.ByteSize = len(val)
		// Use stampede detector for L1 hits: if the entry is approaching
		// its soft expiry, trigger a background refresh via the detector.
		// For the layered cache we use the detector on the miss path only
		// since we don't have access to the eviction.Entry here.
		return val, nil
	}

	// L1 miss: use stampede detector with singleflight to deduplicate
	// the expensive L2 lookup + fn computation.
	return c.detector.Do(ctx, key, nil, nil,
		func(ctx context.Context) ([]byte, error) {
			// Double-check: L1 might have been populated by another goroutine.
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
	if err != nil && !_errors.IsNotFound(err) {
		result.Err = err
		return nil, _errors.WrapKey("layered.getset_l2", key, err)
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
		return false, _errors.WrapKey("layered.cas", key, err)
	}
	if swapped {
		_ = c.l1.Delete(ctx, key)
		c.noPromote.Store(key, struct{}{})
	}
	return swapped, nil
}

// Increment and Decrement are not strictly atomic because they do not support
// an optional TTL. They do use the shard mutex, so concurrent updates to the
// same key are serialized and remain consistent. True atomicity with TTL would
// require a compare-and-swap loop or a backend that supports atomic TTL-aware
// counters, such as Redis.
// nolint:dupl // Increment and Decrement methods are similar enough that deduplication would add more complexity than
// it's worth.
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
		return 0, _errors.WrapKey("layered.increment", key, err)
	}
	_ = c.l1.Delete(ctx, key)
	return v, nil
}

// Decrement is implemented using the same logic as Increment, but it simply negates the delta to achieve the decrement
// effect. Note that this method does not support setting a TTL, so it's not strictly atomic in the sense of a full
// compare-and-swap operation, but it does ensure that concurrent decrements will not interfere with each other and will
// produce a consistent result. nolint:dupl // Increment and Decrement methods are similar enough that deduplication
// would add more complexity than it's worth.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.Increment(ctx, key, -delta)
}

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

	// Atomically set in L2 only — no Exists+Set race.
	set, err := c.l2.SetNX(ctx, key, value, ttl)
	if err != nil {
		result.Err = err
		return false, _errors.WrapKey("layered.setnx_l2", key, err)
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
