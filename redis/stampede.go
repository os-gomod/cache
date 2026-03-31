package redis

import (
	"context"
	"strconv"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

func (c *Cache) getOrSetWithDistributedLock(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	lockKey := c.buildStampedeLockKey(key)
	token := c.nextToken()

	acquired, err := c.acquireLock(ctx, lockKey, token)
	if err != nil {
		return c.computeAndSet(ctx, key, fn, ttl)
	}
	if acquired {
		return c.computeWithLock(ctx, key, fn, ttl, lockKey, token)
	}
	return c.waitForValue(ctx, key, fn, ttl, lockKey)
}

func (c *Cache) computeWithLock(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
	lockKey, token string,
) ([]byte, error) {
	defer c.releaseLock(context.WithoutCancel(ctx), lockKey, token)
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	} else if !_errors.IsNotFound(err) {
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
		} else if !_errors.IsNotFound(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return c.computeAndSet(ctx, key, fn, ttl)
		}
		token := c.nextToken()
		acquired, err := c.acquireLock(ctx, lockKey, token)
		if err != nil {
			return c.computeAndSet(ctx, key, fn, ttl)
		}
		if acquired {
			return c.computeWithLock(ctx, key, fn, ttl, lockKey, token)
		}
		select {
		case <-ctx.Done():
			return nil, _errors.CancelledError("redis.get_or_set")
		case <-ticker.C:
		}
	}
}

func (c *Cache) acquireLock(ctx context.Context, lockKey, token string) (bool, error) {
	return c.client.SetNX(ctx, lockKey, token, c.cfg.StampedeLockTTL).Result()
}

func (c *Cache) releaseLock(ctx context.Context, lockKey, token string) {
	releaseCtx, cancel := context.WithTimeout(ctx, c.cfg.WriteTimeout)
	defer cancel()
	_, _ = c.unlockScript.Run(releaseCtx, c.client, []string{lockKey}, token).Result()
}

func (c *Cache) nextToken() string {
	seq := c.stampedeTokenSeq.Add(1)
	return strconv.FormatUint(seq, 10) + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
