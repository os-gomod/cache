package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/obs"
)

// Get retrieves a value by key.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	_, span := obs.Start(ctx, "redis.get")
	defer span.End()

	if err := c.checkClosed("redis.get"); err != nil {
		span.SetError(err)
		return nil, err
	}
	c.stats.GetOp()
	val, err := c.client.Get(ctx, c.buildKey(key)).Bytes()
	if errors.Is(err, goredis.Nil) {
		c.stats.Miss()
		obs.RecordMiss(ctx, "redis", "get", 0)
		return nil, _errors.NotFound("redis.get", key)
	}
	if err != nil {
		c.stats.ErrorOp()
		obs.RecordError(ctx, "redis", "get")
		span.SetError(err)
		return nil, _errors.WrapKey("redis.get", key, err)
	}
	c.stats.Hit()
	obs.RecordHit(ctx, "redis", "get", 0)
	return val, nil
}

// Set stores a value with optional TTL (0 = use config default).
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_, span := obs.Start(ctx, "redis.set")
	defer span.End()

	if err := c.checkClosed("redis.set"); err != nil {
		span.SetError(err)
		return err
	}
	if ttl < 0 {
		ttl = c.cfg.DefaultTTL
	}
	if err := c.client.Set(ctx, c.buildKey(key), value, ttl).Err(); err != nil {
		c.stats.ErrorOp()
		obs.RecordError(ctx, "redis", "set")
		span.SetError(err)
		return _errors.WrapKey("redis.set", key, err)
	}
	c.stats.SetOp()
	return nil
}

// Delete removes a key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed("redis.delete"); err != nil {
		return err
	}
	if err := c.client.Del(ctx, c.buildKey(key)).Err(); err != nil {
		c.stats.ErrorOp()
		return _errors.WrapKey("redis.delete", key, err)
	}
	c.stats.DeleteOp()
	return nil
}

// Exists reports whether key is present in Redis.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed("redis.exists"); err != nil {
		return false, err
	}
	n, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	if err != nil {
		c.stats.ErrorOp()
		return false, _errors.WrapKey("redis.exists", key, err)
	}
	return n > 0, nil
}

// TTL returns the remaining TTL for a key.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.checkClosed("redis.ttl"); err != nil {
		return 0, err
	}
	d, err := c.client.TTL(ctx, c.buildKey(key)).Result()
	if err != nil {
		return 0, _errors.WrapKey("redis.ttl", key, err)
	}
	// go-redis returns Redis sentinel values as raw durations:
	// -2 means key missing, -1 means no expiry.
	if d == time.Duration(-2) || d == -2*time.Second {
		return 0, _errors.NotFound("redis.ttl", key)
	}
	if d == time.Duration(-1) || d == -1*time.Second {
		return -1 * time.Second, nil
	}
	return d, nil
}

// Ping checks the connection to Redis.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.checkClosed("redis.ping"); err != nil {
		return err
	}
	return c.client.Ping(ctx).Err()
}

// Keys returns keys matching pattern (prefix-aware).
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := c.checkClosed("redis.keys"); err != nil {
		return nil, err
	}
	fullPattern := c.buildKey(pattern)
	keys, err := c.client.Keys(ctx, fullPattern).Result()
	if err != nil {
		return nil, _errors.Wrap("redis.keys", err)
	}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = c.stripPrefix(k)
	}
	return result, nil
}

// Size returns the approximate number of keys tracked by this cache prefix.
func (c *Cache) Size(ctx context.Context) (int64, error) {
	if err := c.checkClosed("redis.size"); err != nil {
		return 0, err
	}
	pattern := c.buildKey("*")
	var (
		count  int64
		cursor uint64
	)
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, _errors.Wrap("redis.size", err)
		}
		count += int64(len(keys))
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}

// Expire sets a TTL on an existing key.
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.checkClosed("redis.expire"); err != nil {
		return err
	}
	return c.client.Expire(ctx, c.buildKey(key), ttl).Err()
}

// Persist removes the TTL from a key, making it persistent.
func (c *Cache) Persist(ctx context.Context, key string) error {
	if err := c.checkClosed("redis.persist"); err != nil {
		return err
	}
	return c.client.Persist(ctx, c.buildKey(key)).Err()
}

// Clear removes all keys matching the configured prefix from the current database.
func (c *Cache) Clear(ctx context.Context) error {
	if err := c.checkClosed("redis.clear"); err != nil {
		return err
	}
	pattern := c.buildKey("*")
	for {
		var (
			toDelete []string
			cursor   uint64
		)
		for {
			keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
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
				return _errors.Wrap("redis.clear", err)
			}
		}
	}
	return nil
}
