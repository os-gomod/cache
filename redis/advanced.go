package redis

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/v2/internal/contracts"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/runtime"
)

// CompareAndSwap atomically replaces oldVal with newVal if the current value
// matches oldVal exactly. Uses a Lua script for atomicity. Returns NotFound error
// if the key doesn't exist, or InvalidConfig error if the value doesn't match.
func (s *Store) CompareAndSwap(ctx context.Context, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	if err := s.checkClosed("redis.cas"); err != nil {
		return false, err
	}

	effectiveTTL := s.resolveTTL(ttl)
	rk := s.buildKey(key)
	ttlSeconds := int64(effectiveTTL.Seconds())

	op := contracts.Operation{
		Name:    "compare_and_swap",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		result, err := casScript.Run(ctx, s.client, []string{rk},
			string(oldVal), string(newVal), ttlSeconds).Int64()
		if err != nil {
			s.stats.ErrorOp()
			return false, cacheerrors.Factory.Connection("redis.cas", err)
		}

		if result == 1 {
			s.stats.SetOp()
			return true, nil
		}

		// Distinguish between "key not found" and "value mismatch"
		if result == -1 {
			// Key doesn't exist
			return false, cacheerrors.Factory.NotFound("redis.cas", key)
		}

		// result == 0: value mismatch
		return false, cacheerrors.Factory.InvalidConfig("redis.cas", "value mismatch")
	})
}

// SetNX sets a key-value pair only if the key does not already exist.
// Uses Redis native SET NX command.
func (s *Store) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if err := s.checkClosed("redis.setnx"); err != nil {
		return false, err
	}

	effectiveTTL := s.resolveTTL(ttl)
	rk := s.buildKey(key)

	op := contracts.Operation{
		Name:    "setnx",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		result, err := s.client.SetNX(ctx, rk, value, effectiveTTL).Result()
		if err != nil {
			s.stats.ErrorOp()
			return false, cacheerrors.Factory.Connection("redis.setnx", err)
		}
		if result {
			s.stats.SetOp()
		}
		return result, nil
	})
}

// Increment atomically increments a numeric value by delta. If the key does
// not exist, it is initialized to 0 before incrementing.
func (s *Store) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := s.checkClosed("redis.increment"); err != nil {
		return 0, err
	}

	rk := s.buildKey(key)

	op := contracts.Operation{
		Name:    "increment",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (int64, error) {
		result, err := s.client.IncrBy(ctx, rk, delta).Result()
		if err != nil {
			s.stats.ErrorOp()
			return 0, cacheerrors.Factory.Connection("redis.increment", err)
		}
		s.stats.SetOp()
		return result, nil
	})
}

// Decrement atomically decrements a numeric value by delta.
func (s *Store) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return s.Increment(ctx, key, -delta)
}

// GetSet atomically sets a new value and returns the old value.
// Uses Redis native GETSET command (or GET + SET in a Lua script).
func (s *Store) GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error) {
	if err := s.checkClosed("redis.getset"); err != nil {
		return nil, err
	}

	effectiveTTL := s.resolveTTL(ttl)
	rk := s.buildKey(key)
	ttlSeconds := int64(effectiveTTL.Seconds())

	op := contracts.Operation{
		Name:    "getset",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]byte, error) {
		result, err := getSetScript.Run(ctx, s.client, []string{rk},
			string(value), ttlSeconds).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				s.stats.SetOp()
				return nil, nil
			}
			s.stats.ErrorOp()
			return nil, cacheerrors.Factory.Connection("redis.getset", err)
		}
		var data []byte
		if result != nil {
			data, _ = result.([]byte)
		}
		s.stats.SetOp()
		return data, nil
	})
}

// Suppress unused import warnings.
var (
	_ = strconv.FormatInt
	_ = bytes.Equal
)
