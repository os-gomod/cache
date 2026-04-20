package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

// observeErr wraps a simple error-returning Redis operation with observability.
// It checks the cache is open, records the operation, and returns any error.
func (c *Cache) observeErr(ctx context.Context, opName, key string, fn func(context.Context) error) error {
	if err := c.checkClosed(opName); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: opName, Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if err := fn(ctx); err != nil {
		result.Err = err
		return err
	}
	return nil
}

// observeInt64 wraps an int64-returning Redis operation with observability.
func (c *Cache) observeInt64(ctx context.Context, opName, key string, fn func(context.Context) (int64, error)) (int64, error) {
	if err := c.checkClosed(opName); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "redis", Name: opName, Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	val, err := fn(ctx)
	if err != nil {
		result.Err = err
	}
	return val, err
}

// observeResult wraps a result-returning Redis operation with observability.
// It returns the zero value of T on error.
func observeResult[T any](ctx context.Context, c *Cache, opName, key string, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := c.checkClosed(opName); err != nil {
		return zero, err
	}
	op := observability.Op{Backend: "redis", Name: opName, Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	val, err := fn(ctx)
	if err != nil {
		result.Err = err
		return zero, err
	}
	return val, nil
}

// GetMulti retrieves values for multiple keys using MGET, returning a map of found entries.
func (c *Cache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := c.checkClosed("redis.get_multi"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "get_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if len(keys) == 0 {
		return map[string][]byte{}, nil
	}
	fullKeys := make([]string, len(keys))
	keyMap := make(map[string]string, len(keys))
	for i, k := range keys {
		fk := c.buildKey(k)
		fullKeys[i] = fk
		keyMap[fk] = k
	}
	results, err := c.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		result.Err = err
		return nil, cacheerrors.Wrap("redis.get_multi", err)
	}
	out := make(map[string][]byte, len(keys))
	for i, res := range results {
		if res == nil {
			c.stats.Miss()
			continue
		}
		if b, ok := res.(string); ok {
			out[keyMap[fullKeys[i]]] = []byte(b)
			c.stats.Hit()
		}
	}
	if len(out) > 0 {
		result.Hit = true
	}
	return out, nil
}

// SetMulti stores multiple key-value pairs with the given TTL using a pipeline.
func (c *Cache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if err := c.checkClosed("redis.set_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "set_multi", KeyCount: len(items)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if len(items) == 0 {
		return nil
	}
	if ttl < 0 {
		ttl = c.cfg.DefaultTTL
	}
	pipe := c.client.Pipeline()
	for key, val := range items {
		pipe.Set(ctx, c.buildKey(key), val, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		result.Err = err
		return err
	}
	return nil
}

// DeleteMulti removes multiple keys from Redis.
func (c *Cache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := c.checkClosed("redis.delete_multi"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "delete_multi", KeyCount: len(keys)}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	if len(keys) == 0 {
		return nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.buildKey(k)
	}
	if err := c.client.Del(ctx, fullKeys...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

// DeleteByPrefix removes all keys matching the given prefix.
func (c *Cache) DeleteByPrefix(ctx context.Context, prefix string) error {
	if err := c.checkClosed("redis.delete_by_prefix"); err != nil {
		return err
	}

	op := observability.Op{Backend: "redis", Name: "delete_by_prefix", Key: prefix}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	pattern := c.buildKey(prefix) + "*"

	var cursor uint64
	var keys []string

	// 1. Scan all matching keys
	for {
		var batch []string
		var err error
		batch, cursor, err = c.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			result.Err = err
			return cacheerrors.Wrap("redis.delete_by_prefix", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return nil
	}

	// 2. Delete in pipeline batches
	const batchSize = 500
	pipe := c.client.Pipeline()

	for i, k := range keys {
		pipe.Del(ctx, k)

		if (i+1)%batchSize == 0 || i == len(keys)-1 {
			if _, err := pipe.Exec(ctx); err != nil {
				result.Err = err
				return cacheerrors.Wrap("redis.delete_by_prefix", err)
			}
			pipe = c.client.Pipeline()
		}
	}

	return nil
}

// GetOrSet retrieves the value for key, or calls fn to compute, cache, and return it.
// When distributed stampede protection is enabled, it uses a Redis lock.
func (c *Cache) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("redis.get_or_set"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "get_or_set", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	val, err := c.Get(ctx, key)
	if err == nil {
		result.Hit = true
		result.ByteSize = len(val)
		return val, nil
	}
	return c.sg.Do(ctx, key, func() ([]byte, error) {
		if cachedVal, errGet := c.Get(ctx, key); errGet == nil {
			return cachedVal, nil
		}
		if c.cfg.EnableDistributedStampedeProtection {
			return c.getOrSetWithDistributedLock(ctx, key, fn, ttl)
		}
		return c.computeAndSet(ctx, key, fn, ttl)
	})
}

// GetSet sets the value for a key and returns the previous value.
func (c *Cache) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := c.checkClosed("redis.getset"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "getset", Key: key}
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
	fullKey := c.buildKey(key)
	if ttl > 0 {
		args := goredis.SetArgs{TTL: ttl, Get: true}
		old, err := c.client.SetArgs(ctx, fullKey, value, args).Bytes()
		if errors.Is(err, goredis.Nil) {
			c.stats.SetOp()
			return nil, nil
		}
		if err != nil {
			c.stats.ErrorOp()
			result.Err = err
			return nil, cacheerrors.WrapKey("redis.getset", key, err)
		}
		c.stats.SetOp()
		result.ByteSize = len(old)
		return old, nil
	}
	raw, err := c.getSetScript.Run(ctx, c.client, []string{fullKey}, value).Result()
	if errors.Is(err, goredis.Nil) {
		c.stats.SetOp()
		return nil, nil
	}
	if err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return nil, cacheerrors.WrapKey("redis.getset", key, err)
	}
	c.stats.SetOp()
	switch v := raw.(type) {
	case string:
		result.ByteSize = len(v)
		return []byte(v), nil
	case []byte:
		result.ByteSize = len(v)
		return v, nil
	default:
		return nil, nil
	}
}

func (c *Cache) computeAndSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	val, err := fn()
	if err != nil {
		return nil, err
	}
	_ = c.Set(ctx, key, val, ttl)
	return val, nil
}

// CompareAndSwap atomically sets key to newVal if its current value equals oldVal using a Lua script.
func (c *Cache) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("redis.cas"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "redis", Name: "cas", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	var ttlMS int64
	if ttl > 0 {
		ttlMS = ttl.Milliseconds()
	}
	res, err := c.casScript.Run(
		ctx, c.client, []string{c.buildKey(key)},
		oldVal, newVal, strconv.FormatInt(ttlMS, 10),
	).Int64()
	if err != nil {
		c.stats.ErrorOp()
		result.Err = err
		return false, cacheerrors.WrapKey("redis.cas", key, err)
	}
	return res == 1, nil
}

// Increment atomically adds delta to the integer value stored at key, returning the new value.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return c.observeInt64(ctx, "redis.increment", key, func(ctx context.Context) (int64, error) {
		return c.client.IncrBy(ctx, c.buildKey(key), delta).Result()
	})
}

// Decrement atomically subtracts delta from the integer value stored at key.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return c.observeInt64(ctx, "redis.decrement", key, func(ctx context.Context) (int64, error) {
		return c.client.DecrBy(ctx, c.buildKey(key), delta).Result()
	})
}

// SetNX sets the key-value pair only if the key does not already exist.
func (c *Cache) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := c.checkClosed("redis.setnx"); err != nil {
		return false, err
	}
	op := observability.Op{Backend: "redis", Name: "setnx", Key: key}
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
	val, err := c.client.SetNX(ctx, c.buildKey(key), value, ttl).Result()
	if err != nil {
		result.Err = err
	}
	return val, err
}

// HSet sets a field in a Redis hash.
func (c *Cache) HSet(ctx context.Context, key, field string, value any) error {
	return c.observeErr(ctx, "redis.hset", key, func(ctx context.Context) error {
		return c.client.HSet(ctx, c.buildKey(key), field, value).Err()
	})
}

// HGet retrieves a field from a Redis hash.
func (c *Cache) HGet(ctx context.Context, key, field string) ([]byte, error) {
	if err := c.checkClosed("redis.hget"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "hget", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()
	val, err := c.client.HGet(ctx, c.buildKey(key), field).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, cacheerrors.NotFound("redis.hget", key+":"+field)
	}
	if err != nil {
		result.Err = err
		return nil, err
	}
	result.Hit = true
	result.ByteSize = len(val)
	return val, nil
}

// HGetAll retrieves all fields and values from a Redis hash.
func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return observeResult(ctx, c, "redis.hgetall", key, func(ctx context.Context) (map[string]string, error) {
		return c.client.HGetAll(ctx, c.buildKey(key)).Result()
	})
}

// HDel removes one or more fields from a Redis hash.
func (c *Cache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.observeErr(ctx, "redis.hdel", key, func(ctx context.Context) error {
		return c.client.HDel(ctx, c.buildKey(key), fields...).Err()
	})
}

// LPush prepends values to a Redis list.
func (c *Cache) LPush(ctx context.Context, key string, values ...any) error {
	return c.observeErr(ctx, "redis.lpush", key, func(ctx context.Context) error {
		return c.client.LPush(ctx, c.buildKey(key), values...).Err()
	})
}

// RPush appends values to a Redis list.
func (c *Cache) RPush(ctx context.Context, key string, values ...any) error {
	return c.observeErr(ctx, "redis.rpush", key, func(ctx context.Context) error {
		return c.client.RPush(ctx, c.buildKey(key), values...).Err()
	})
}

func (c *Cache) pop(
	ctx context.Context,
	op, key string,
	fn func() *goredis.StringCmd,
) ([]byte, error) {
	if err := c.checkClosed(op); err != nil {
		return nil, err
	}
	obsOp := observability.Op{Backend: "redis", Name: op, Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, obsOp)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, obsOp, result)
	}()
	val, err := fn().Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, cacheerrors.NotFound(op, key)
	}
	if err != nil {
		result.Err = err
		return nil, err
	}
	result.ByteSize = len(val)
	return val, nil
}

// LPop removes and returns the first element of a Redis list.
func (c *Cache) LPop(ctx context.Context, key string) ([]byte, error) {
	return c.pop(ctx, "redis.lpop", key, func() *goredis.StringCmd {
		return c.client.LPop(ctx, c.buildKey(key))
	})
}

// RPop removes and returns the last element of a Redis list.
func (c *Cache) RPop(ctx context.Context, key string) ([]byte, error) {
	return c.pop(ctx, "redis.rpop", key, func() *goredis.StringCmd {
		return c.client.RPop(ctx, c.buildKey(key))
	})
}

// LRange returns a range of elements from a Redis list.
// nolint:dupl // The LPop/RPop and LRange implementations are similar but not worth abstracting further.
func (c *Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return observeResult(ctx, c, "redis.lrange", key, func(ctx context.Context) ([]string, error) {
		return c.client.LRange(ctx, c.buildKey(key), start, stop).Result()
	})
}

// SAdd adds members to a Redis set.
func (c *Cache) SAdd(ctx context.Context, key string, members ...any) error {
	return c.observeErr(ctx, "redis.sadd", key, func(ctx context.Context) error {
		return c.client.SAdd(ctx, c.buildKey(key), members...).Err()
	})
}

// SMembers returns all members of a Redis set.
func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	return observeResult(ctx, c, "redis.smembers", key, func(ctx context.Context) ([]string, error) {
		return c.client.SMembers(ctx, c.buildKey(key)).Result()
	})
}

// SRem removes members from a Redis set.
func (c *Cache) SRem(ctx context.Context, key string, members ...any) error {
	return c.observeErr(ctx, "redis.srem", key, func(ctx context.Context) error {
		return c.client.SRem(ctx, c.buildKey(key), members...).Err()
	})
}

// ZAdd adds a member with the given score to a Redis sorted set.
func (c *Cache) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return c.observeErr(ctx, "redis.zadd", key, func(ctx context.Context) error {
		return c.client.ZAdd(ctx, c.buildKey(key), goredis.Z{Score: score, Member: member}).Err()
	})
}

// ZRange returns a range of members from a Redis sorted set by index.
// nolint:dupl // The LPop/RPop and LRange implementations are similar but not worth abstracting further.
func (c *Cache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return observeResult(ctx, c, "redis.zrange", key, func(ctx context.Context) ([]string, error) {
		return c.client.ZRange(ctx, c.buildKey(key), start, stop).Result()
	})
}

// ZRem removes one or more members from a Redis sorted set.
func (c *Cache) ZRem(ctx context.Context, key string, members ...any) error {
	return c.observeErr(ctx, "redis.zrem", key, func(ctx context.Context) error {
		return c.client.ZRem(ctx, c.buildKey(key), members...).Err()
	})
}
