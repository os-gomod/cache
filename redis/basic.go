package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-gomod/cache/v2/internal/contracts"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/keyutil"
	"github.com/os-gomod/cache/v2/internal/runtime"
)

// Get retrieves the value for the given key from Redis.
// Returns errors.NotFound if the key does not exist.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkClosed("redis.get"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "get",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]byte, error) {
		rk := s.buildKey(key)
		result, err := s.client.Get(ctx, rk).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				s.stats.Miss()
				return nil, cacheerrors.Factory.NotFound("redis.get", key)
			}
			s.stats.ErrorOp()
			return nil, cacheerrors.Factory.Connection("redis.get", err)
		}
		s.stats.Hit()
		return result, nil
	})
}

// GetMulti retrieves multiple values by key. Missing keys are omitted from
// the returned map.
func (s *Store) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}
	if err := s.checkClosed("redis.get_multi"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:     "get_multi",
		KeyCount: len(keys),
		Backend:  "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (map[string][]byte, error) {
		// Build piped commands for all keys
		pipes := make(map[string]string, len(keys))
		for _, key := range keys {
			pipes[s.buildKey(key)] = key
		}

		pipe := s.client.Pipeline()
		cmds := make(map[string]*redis.StringCmd, len(pipes))
		for rk := range pipes {
			cmds[rk] = pipe.Get(ctx, rk)
		}

		_, err := pipe.Exec(ctx)
		if err != nil && !errors.Is(err, redis.Nil) {
			s.stats.ErrorOp()
			return nil, cacheerrors.Factory.Connection("redis.get_multi", err)
		}

		result := make(map[string][]byte, len(keys))
		for rk, origKey := range pipes {
			val, cmdErr := cmds[rk].Bytes()
			if cmdErr != nil {
				if !errors.Is(cmdErr, redis.Nil) {
					s.stats.ErrorOp()
				}
				s.stats.Miss()
				continue
			}
			result[origKey] = val
			s.stats.Hit()
		}
		return result, nil
	})
}

// Set stores a key-value pair with the given TTL in Redis.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.checkClosed("redis.set"); err != nil {
		return err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:    "set",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		rk := s.buildKey(key)
		if err := s.client.Set(ctx, rk, value, effectiveTTL).Err(); err != nil {
			s.stats.ErrorOp()
			return cacheerrors.Factory.Connection("redis.set", err)
		}
		s.stats.SetOp()
		return nil
	})
}

// SetMulti stores multiple key-value pairs with the given TTL using a
// Redis pipeline for efficiency.
func (s *Store) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}
	if err := s.checkClosed("redis.set_multi"); err != nil {
		return err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:     "set_multi",
		KeyCount: len(items),
		Backend:  "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		pipe := s.client.Pipeline()
		for key, value := range items {
			rk := s.buildKey(key)
			pipe.Set(ctx, rk, value, effectiveTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			s.stats.ErrorOp()
			return cacheerrors.Factory.Connection("redis.set_multi", err)
		}
		s.stats.SetOp()
		return nil
	})
}

// Delete removes a key from Redis. Deleting a non-existent key is a no-op.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.checkClosed("redis.delete"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "delete",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		rk := s.buildKey(key)
		if err := s.client.Del(ctx, rk).Err(); err != nil {
			s.stats.ErrorOp()
			return cacheerrors.Factory.Connection("redis.delete", err)
		}
		s.stats.DeleteOp()
		return nil
	})
}

// DeleteMulti removes multiple keys from Redis using a pipeline.
func (s *Store) DeleteMulti(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.checkClosed("redis.delete_multi"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:     "delete_multi",
		KeyCount: len(keys),
		Backend:  "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		rks := make([]string, len(keys))
		for i, key := range keys {
			rks[i] = s.buildKey(key)
		}
		if err := s.client.Del(ctx, rks...).Err(); err != nil {
			s.stats.ErrorOp()
			return cacheerrors.Factory.Connection("redis.delete_multi", err)
		}
		s.stats.DeleteOp()
		return nil
	})
}

// Exists reports whether a key exists in Redis.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.checkClosed("redis.exists"); err != nil {
		return false, err
	}

	op := contracts.Operation{
		Name:    "exists",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		rk := s.buildKey(key)
		n, err := s.client.Exists(ctx, rk).Result()
		if err != nil {
			s.stats.ErrorOp()
			return false, cacheerrors.Factory.Connection("redis.exists", err)
		}
		return n > 0, nil
	})
}

// TTL returns the remaining time-to-live for the given key in Redis.
func (s *Store) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := s.checkClosed("redis.ttl"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "ttl",
		Key:     key,
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (time.Duration, error) {
		rk := s.buildKey(key)
		dur, err := s.client.TTL(ctx, rk).Result()
		if err != nil {
			s.stats.ErrorOp()
			return 0, cacheerrors.Factory.Connection("redis.ttl", err)
		}
		if dur < 0 {
			// -1 = no expiry, -2 = key doesn't exist
			return 0, cacheerrors.Factory.NotFound("redis.ttl", key)
		}
		return dur, nil
	})
}

// Keys returns all keys matching the given pattern using the SCAN command.
func (s *Store) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := s.checkClosed("redis.keys"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "keys",
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]string, error) {
		scanPattern := s.buildKey(pattern)
		if pattern == "" {
			scanPattern = s.cfg.keyPrefix + "*"
		}
		var allKeys []string
		var cursor uint64

		for {
			var keys []string
			var err error
			keys, cursor, err = s.client.Scan(ctx, cursor, scanPattern, 100).Result()
			if err != nil {
				s.stats.ErrorOp()
				return nil, cacheerrors.Factory.Connection("redis.keys", err)
			}
			for _, k := range keys {
				// Strip prefix from returned keys
				allKeys = append(allKeys, keyutil.StripPrefix(s.cfg.keyPrefix, k))
			}
			if cursor == 0 {
				break
			}
		}
		return allKeys, nil
	})
}

// Clear removes all keys with the configured prefix from Redis using SCAN+DEL.
func (s *Store) Clear(ctx context.Context) error {
	if err := s.checkClosed("redis.clear"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "clear",
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		pattern := s.cfg.keyPrefix + "*"
		var cursor uint64

		for {
			var keys []string
			var err error
			keys, cursor, err = s.client.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				s.stats.ErrorOp()
				return cacheerrors.Factory.Connection("redis.clear", err)
			}
			if len(keys) > 0 {
				if delErr := s.client.Del(ctx, keys...).Err(); delErr != nil {
					s.stats.ErrorOp()
					return cacheerrors.Factory.Connection("redis.clear", delErr)
				}
			}
			if cursor == 0 {
				break
			}
		}
		s.stats.Reset()
		return nil
	})
}

// Size returns the approximate number of keys with the configured prefix
// using SCAN to count them.
func (s *Store) Size(ctx context.Context) (int64, error) {
	if err := s.checkClosed("redis.size"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "size",
		Backend: "redis",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (int64, error) {
		pattern := s.cfg.keyPrefix + "*"
		var count int64
		var cursor uint64

		for {
			var keys []string
			var err error
			keys, cursor, err = s.client.Scan(ctx, cursor, pattern, 1000).Result()
			if err != nil {
				s.stats.ErrorOp()
				return 0, cacheerrors.Factory.Connection("redis.size", err)
			}
			count += int64(len(keys))
			if cursor == 0 {
				break
			}
		}
		return count, nil
	})
}

// Suppress unused import.
var _ = fmt.Sprint
