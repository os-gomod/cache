package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

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
		return nil, _errors.Wrap("redis.get_multi", err)
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

	// Try cache hit first.
	val, err := c.Get(ctx, key)
	if err == nil {
		result.Hit = true
		result.ByteSize = len(val)
		return val, nil
	}

	// Cache miss: use singleflight to deduplicate, with optional
	// distributed lock for multi-instance stampede protection.
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
			return nil, _errors.WrapKey("redis.getset", key, err)
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
		return nil, _errors.WrapKey("redis.getset", key, err)
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
		return false, _errors.WrapKey("redis.cas", key, err)
	}
	return res == 1, nil
}

// Increment and Decrement do not support an optional TTL, but they still use
// Redis's atomic INCRBY and DECRBY commands under the hood.
// nolint:dupl // Increment and Decrement methods are similar enough that deduplication would add more complexity than
// it's worth.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("redis.increment"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "redis", Name: "increment", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.IncrBy(ctx, c.buildKey(key), delta).Result()
	if err != nil {
		result.Err = err
	}
	return val, err
}

// Decrement is implemented using Redis's DECRBY command, which atomically decrements the value. Note that this method
// does not support setting a TTL, so it's not strictly atomic in the sense of a full compare-and-swap operation, but it
// does ensure that concurrent decrements will not interfere with each other.
// nolint:dupl // Increment and Decrement methods are similar enough that deduplication would add more complexity than
// it's worth.
func (c *Cache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.checkClosed("redis.decrement"); err != nil {
		return 0, err
	}
	op := observability.Op{Backend: "redis", Name: "decrement", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.DecrBy(ctx, c.buildKey(key), delta).Result()
	if err != nil {
		result.Err = err
	}
	return val, err
}

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

func (c *Cache) HSet(ctx context.Context, key, field string, value any) error {
	if err := c.checkClosed("redis.hset"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "hset", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.HSet(ctx, c.buildKey(key), field, value).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

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
		return nil, _errors.NotFound("redis.hget", key+":"+field)
	}
	if err != nil {
		result.Err = err
		return nil, err
	}
	result.Hit = true
	result.ByteSize = len(val)
	return val, nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if err := c.checkClosed("redis.hgetall"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "hgetall", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.HGetAll(ctx, c.buildKey(key)).Result()
	if err != nil {
		result.Err = err
		return nil, err
	}
	return val, nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) HDel(ctx context.Context, key string, fields ...string) error {
	if err := c.checkClosed("redis.hdel"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "hdel", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.HDel(ctx, c.buildKey(key), fields...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) LPush(ctx context.Context, key string, values ...any) error {
	if err := c.checkClosed("redis.lpush"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "lpush", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.LPush(ctx, c.buildKey(key), values...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) RPush(ctx context.Context, key string, values ...any) error {
	if err := c.checkClosed("redis.rpush"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "rpush", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.RPush(ctx, c.buildKey(key), values...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
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
		return nil, _errors.NotFound(op, key)
	}
	if err != nil {
		result.Err = err
		return nil, err
	}
	result.ByteSize = len(val)
	return val, nil
}

func (c *Cache) LPop(ctx context.Context, key string) ([]byte, error) {
	return c.pop(ctx, "redis.lpop", key, func() *goredis.StringCmd {
		return c.client.LPop(ctx, c.buildKey(key))
	})
}

func (c *Cache) RPop(ctx context.Context, key string) ([]byte, error) {
	return c.pop(ctx, "redis.rpop", key, func() *goredis.StringCmd {
		return c.client.RPop(ctx, c.buildKey(key))
	})
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := c.checkClosed("redis.lrange"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "lrange", Key: key}
	t0 := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(t0)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.LRange(ctx, c.buildKey(key), start, stop).Result()
	if err != nil {
		result.Err = err
		return nil, err
	}
	return val, nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) SAdd(ctx context.Context, key string, members ...any) error {
	if err := c.checkClosed("redis.sadd"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "sadd", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.SAdd(ctx, c.buildKey(key), members...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	if err := c.checkClosed("redis.smembers"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "smembers", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.SMembers(ctx, c.buildKey(key)).Result()
	if err != nil {
		result.Err = err
		return nil, err
	}
	return val, nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) SRem(ctx context.Context, key string, members ...any) error {
	if err := c.checkClosed("redis.srem"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "srem", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.SRem(ctx, c.buildKey(key), members...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

func (c *Cache) ZAdd(ctx context.Context, key string, score float64, member string) error {
	if err := c.checkClosed("redis.zadd"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "zadd", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.ZAdd(ctx, c.buildKey(key), goredis.Z{Score: score, Member: member}).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := c.checkClosed("redis.zrange"); err != nil {
		return nil, err
	}
	op := observability.Op{Backend: "redis", Name: "zrange", Key: key}
	t0 := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(t0)
		c.chain.After(ctx, op, result)
	}()

	val, err := c.client.ZRange(ctx, c.buildKey(key), start, stop).Result()
	if err != nil {
		result.Err = err
		return nil, err
	}
	return val, nil
}

//nolint:dupl // Redis collection wrappers intentionally stay explicit per command.
func (c *Cache) ZRem(ctx context.Context, key string, members ...any) error {
	if err := c.checkClosed("redis.zrem"); err != nil {
		return err
	}
	op := observability.Op{Backend: "redis", Name: "zrem", Key: key}
	start := time.Now()
	ctx = c.chain.Before(ctx, op)
	var result observability.Result
	defer func() {
		result.Latency = time.Since(start)
		c.chain.After(ctx, op, result)
	}()

	if err := c.client.ZRem(ctx, c.buildKey(key), members...).Err(); err != nil {
		result.Err = err
		return err
	}
	return nil
}
