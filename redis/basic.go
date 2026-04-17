package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

// Get retrieves the value for the given key from Redis.
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
		return nil, cacheerrors.NotFound("redis.get", key)
	}
	if err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return nil, cacheerrors.WrapKey("redis.get", key, err)
	}
	c.stats.Hit()
	result.Hit = true
	result.ByteSize = len(val)
	return val, nil
}

// Set stores a key-value pair with the given TTL in Redis.
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
		return cacheerrors.WrapKey("redis.set", key, err)
	}
	c.stats.SetOp()
	return nil
}

// Delete removes a key from Redis.
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
		return cacheerrors.WrapKey("redis.delete", key, err)
	}
	c.stats.DeleteOp()
	return nil
}

// Exists reports whether the key exists in Redis.
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
		return false, cacheerrors.WrapKey("redis.exists", key, err)
	}
	result.Hit = n > 0
	return n > 0, nil
}

// TTL returns the remaining time-to-live for the given key.
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
		return 0, cacheerrors.WrapKey("redis.ttl", key, err)
	}
	if d == time.Duration(-2) || d == -2*time.Second {
		return 0, cacheerrors.NotFound("redis.ttl", key)
	}
	if d == time.Duration(-1) || d == -1*time.Second {
		return -1 * time.Second, nil
	}
	return d, nil
}

// Ping checks whether the Redis server is reachable.
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

// Keys returns all keys matching the given pattern, with the key prefix stripped.
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
		return nil, cacheerrors.Wrap("redis.keys", err)
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = c.stripPrefix(k)
	}
	return out, nil
}

// Size returns the approximate number of keys in the cache's key namespace.
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
			return 0, cacheerrors.Wrap("redis.size", err)
		}
		count += int64(len(keys))
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}

// Expire updates the TTL for an existing key.
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

// Persist removes the TTL from a key, making it persist indefinitely.
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

// Clear removes all keys in the cache's key namespace from Redis.
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
				return cacheerrors.Wrap("redis.clear", err)
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
				return cacheerrors.Wrap("redis.clear", err)
			}
		}
	}
	return nil
}
