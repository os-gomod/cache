package redis

import (
	"context"
	"strconv"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/stampede"
)

func (c *Cache) getOrSetWithDistributedLock(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	lockKey := c.buildStampedeLockKey(key)
	token := c.nextToken()
	lock, acquired, err := stampede.AcquireLock(
		ctx,
		c.client,
		lockKey,
		token,
		c.cfg.StampedeLockTTL,
	)
	if err != nil {
		return c.computeAndSet(ctx, key, fn, ttl)
	}
	if acquired {
		return c.computeWithLock(ctx, key, fn, ttl, lock)
	}
	return c.waitForValue(ctx, key, fn, ttl, lockKey)
}

func (c *Cache) computeWithLock(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
	lock *stampede.DistributedLock,
) ([]byte, error) {
	defer func() { _ = lock.Release(context.WithoutCancel(ctx)) }()
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	} else if !cacheerrors.IsNotFound(err) {
		return nil, err
	}
	return c.computeAndSet(ctx, key, fn, ttl)
}

func (c *Cache) waitForValue(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
	lockKey string,
) ([]byte, error) {
	wait := c.cfg.StampedeWaitTimeout
	if wait <= 0 {
		return c.computeAndSet(ctx, key, fn, ttl)
	}
	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(c.cfg.StampedeRetryInterval)
	defer ticker.Stop()
	for {
		if val, err := c.Get(ctx, key); err == nil {
			return val, nil
		} else if !cacheerrors.IsNotFound(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return c.computeAndSet(ctx, key, fn, ttl)
		}
		token := c.nextToken()
		lock, acquired, err := stampede.AcquireLock(
			ctx,
			c.client,
			lockKey,
			token,
			c.cfg.StampedeLockTTL,
		)
		if err != nil {
			return c.computeAndSet(ctx, key, fn, ttl)
		}
		if acquired {
			return c.computeWithLock(ctx, key, fn, ttl, lock)
		}
		select {
		case <-ctx.Done():
			return nil, cacheerrors.CancelledError("redis.get_or_set")
		case <-ticker.C:
		}
	}
}

func (c *Cache) nextToken() string {
	seq := c.stampedeTokenSeq.Add(1)
	return strconv.FormatUint(seq, 10) + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
