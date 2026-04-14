package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := c.checkClosed("redis.get"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "get", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	c.stats.RecordGet()
	val, err := c.client.Get(ctx, c.buildKey(key)).Bytes()
	if errors.Is(err, goredis.Nil) {
		c.stats.Miss()
		return nil, _errors.NotFound("redis.get", key)
	}
	if err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return nil, _errors.WrapKey("redis.get", key, err)
	}
	c.stats.Hit()
	result.Hit = true
	result.ByteSize = len(val)
	return val, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.checkClosed("redis.set"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if ttl < 0 {
		ttl = c.cfg.DefaultTTL
	}
	if err := c.client.Set(ctx, c.buildKey(key), value, ttl).Err(); err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return _errors.WrapKey("redis.set", key, err)
	}
	c.stats.SetOp()
	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed("redis.delete"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "delete", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.Del(ctx, c.buildKey(key)).Err(); err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return _errors.WrapKey("redis.delete", key, err)
	}
	c.stats.DeleteOp()
	return nil
}

func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("redis.exists"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "redis", Name: "exists", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	n, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	if err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return false, _errors.WrapKey("redis.exists", key, err)
	}
	result.Hit = n > 0
	return n > 0, nil
}

func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("redis.ttl"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "redis", Name: "ttl", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	d, err := c.client.TTL(ctx, c.buildKey(key)).Result()
	if err != nil {
		result.Err = err
		return 0, _errors.WrapKey("redis.ttl", key, err)
	}
	if d == time.Duration(-2) || d == -2*time.Second {
		return 0, _errors.NotFound("redis.ttl", key)
	}
	if d == time.Duration(-1) || d == -1*time.Second {
		return -1 * time.Second, nil
	}
	return d, nil
}

func (c *Cache) Ping(ctx context.Context) error {
	if err := c.checkClosed("redis.ping"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "ping"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.Ping(ctx).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := c.checkClosed("redis.keys"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "keys"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	fullPattern := c.buildKey(pattern)
	keys, err := c.client.Keys(ctx, fullPattern).Result()
	if err != nil {
		result.Err = err
		return nil, _errors.Wrap("redis.keys", err)
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = c.stripPrefix(k)
	}
	return out, nil
}

func (c *Cache) Size(ctx context.Context) (int64, error) {
	if err := c.checkClosed("redis.size"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "redis", Name: "size"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	pattern := c.buildKey("*")
	var (
		count  int64
		cursor uint64
	)
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			result.Err = err
			return 0, _errors.Wrap("redis.size", err)
		}
		count += int64(len(keys))
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}

//nolint:dupl // Simple command wrappers intentionally mirror other Redis operations.
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.checkClosed("redis.expire"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "expire", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.Expire(ctx, c.buildKey(key), ttl).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

func (c *Cache) Persist(ctx context.Context, key string) error {
	if err := c.checkClosed("redis.persist"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "persist", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.Persist(ctx, c.buildKey(key)).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

func (c *Cache) Clear(ctx context.Context) error {
	if err := c.checkClosed("redis.clear"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "clear"}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	pattern := c.buildKey("*")
	for {
		var (
			toDelete []string
			cursor   uint64
		)
		for {
			keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				result.Err = err
				return _errors.Wrap("redis.clear", err)
			}
			toDelete = append(toDelete, keys...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
		if len(toDelete) == 0 {
			break
		}
		const batchSize = 100
		for start := 0; start < len(toDelete); start += batchSize {
			end := start + batchSize
			if end > len(toDelete) {
				end = len(toDelete)
			}
			if err := c.client.Del(ctx, toDelete[start:end]...).Err(); err != nil {
				result.Err = err
				return _errors.Wrap("redis.clear", err)
			}
		}
	}
	return nil
}
