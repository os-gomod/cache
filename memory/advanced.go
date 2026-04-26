package memory

import (
	"bytes"
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/runtime"
)

// CompareAndSwap atomically replaces oldVal with newVal if the current value
// for the key matches oldVal exactly. Returns true if the swap was performed.
func (s *Store) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := s.validateKey("memory.cas", key); err != nil {
		return false, err
	}
	if err := s.checkClosed("memory.cas"); err != nil {
		return false, err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:    "compare_and_swap",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) (bool, error) {
		sh := s.shardFor(key)
		now := time.Now().UnixNano()

		sh.mu.Lock()
		e, ok := sh.get(key)
		if !ok || e.IsExpired(now) {
			sh.mu.Unlock()
			return false, errors.Factory.NotFound("memory.cas", key)
		}
		if !bytes.Equal(e.Value, oldVal) {
			sh.mu.Unlock()
			return false, nil
		}

		newEntry := NewEntry(key, newVal, effectiveTTL, now)
		perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
		sh.set(key, newEntry, perShardMax)
		sh.mu.Unlock()

		s.stats.SetOp()
		return true, nil
	})
}

// SetNX sets a key-value pair only if the key does not already exist (or has
// expired). Returns true if the key was set, false if it already existed.
func (s *Store) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := s.validateKey("memory.setnx", key); err != nil {
		return false, err
	}
	if err := s.checkClosed("memory.setnx"); err != nil {
		return false, err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:    "setnx",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) (bool, error) {
		sh := s.shardFor(key)
		now := time.Now().UnixNano()

		sh.mu.Lock()
		e, ok := sh.get(key)
		if ok && !e.IsExpired(now) {
			sh.mu.Unlock()
			return false, nil
		}

		newEntry := NewEntry(key, value, effectiveTTL, now)
		perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
		sh.set(key, newEntry, perShardMax)
		sh.mu.Unlock()

		s.stats.SetOp()
		s.stats.AddItems(1)
		s.stats.AddMemory(newEntry.Size)
		return true, nil
	})
}

// Increment atomically increments a numeric value by delta and returns the
// new value. If the key does not exist, it is initialized to 0 before
// incrementing. The stored value is converted to/from []byte using
// strconv.FormatInt/ParseInt.
func (s *Store) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := s.validateKey("memory.increment", key); err != nil {
		return 0, err
	}
	if err := s.checkClosed("memory.increment"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "increment",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) (int64, error) {
		sh := s.shardFor(key)
		now := time.Now().UnixNano()

		sh.mu.Lock()
		e, ok := sh.get(key)
		var current int64
		if ok && !e.IsExpired(now) {
			current, _ = strconv.ParseInt(string(e.Value), 10, 64)
		}
		current += delta
		newVal := []byte(strconv.FormatInt(current, 10))
		newEntry := NewEntry(key, newVal, s.resolveTTL(0), now)
		newEntry.Frequency = 1
		perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
		sh.set(key, newEntry, perShardMax)
		sh.mu.Unlock()

		s.stats.SetOp()
		return current, nil
	})
}

// Decrement atomically decrements a numeric value by delta and returns the
// new value.
func (s *Store) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return s.Increment(ctx, key, -delta)
}

// GetSet atomically sets a new value for the key and returns the previous
// value. If the key did not exist, nil is returned for the old value.
func (s *Store) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := s.validateKey("memory.getset", key); err != nil {
		return nil, err
	}
	if err := s.checkClosed("memory.getset"); err != nil {
		return nil, err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:    "getset",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) ([]byte, error) {
		sh := s.shardFor(key)
		now := time.Now().UnixNano()

		sh.mu.Lock()
		var oldVal []byte
		e, ok := sh.get(key)
		if ok && !e.IsExpired(now) {
			oldVal = make([]byte, len(e.Value))
			copy(oldVal, e.Value)
		}

		newEntry := NewEntry(key, value, effectiveTTL, now)
		perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
		sh.set(key, newEntry, perShardMax)
		sh.mu.Unlock()

		s.stats.SetOp()
		return oldVal, nil
	})
}

// Keys returns all non-expired keys in the store that match the given glob
// pattern. An empty pattern matches all keys.
func (s *Store) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := s.checkClosed("memory.keys"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "keys",
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) ([]string, error) {
		var allKeys []string
		for _, sh := range s.shards {
			sh.mu.RLock()
			keys := sh.keys(pattern)
			sh.mu.RUnlock()
			allKeys = append(allKeys, keys...)
		}
		return allKeys, nil
	})
}

// Clear removes all entries from all shards and resets statistics.
func (s *Store) Clear(ctx context.Context) error {
	if err := s.checkClosed("memory.clear"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "clear",
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(_ctx context.Context) error {
		for _, sh := range s.shards {
			sh.mu.Lock()
			sh.clear()
			sh.mu.Unlock()
		}
		s.stats.Reset()
		return nil
	})
}

// Size returns the total number of non-expired entries across all shards.
func (s *Store) Size(ctx context.Context) (int64, error) {
	if err := s.checkClosed("memory.size"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "size",
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) (int64, error) {
		var total int64
		for _, sh := range s.shards {
			sh.mu.RLock()
			count, _ := sh.totalSize()
			sh.mu.RUnlock()
			total += count
		}
		return total, nil
	})
}

// GetOrSet retrieves the value for the given key. If the key is missing or
// expired, fn is called to produce the value, which is then stored with the
// given TTL. The fn function is called at most once per key, even under
// concurrent access (using sync.SingleFlight semantics).
func (s *Store) GetOrSet(
	ctx context.Context,
	key string,
	fn func() ([]byte, error),
	ttl time.Duration,
) ([]byte, error) {
	// Try to get first
	val, err := s.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if !errors.Factory.IsNotFound(err) {
		return nil, err
	}

	// Key not found: call fn and set
	val, err = fn()
	if err != nil {
		return nil, err
	}

	setErr := s.Set(ctx, key, val, ttl)
	if setErr != nil {
		return nil, setErr
	}
	return val, nil
}

// Ensure Store implements contracts.Cache at compile time.
var (
	_ contracts.Reader        = (*Store)(nil)
	_ contracts.Writer        = (*Store)(nil)
	_ contracts.AtomicOps     = (*Store)(nil)
	_ contracts.Scanner       = (*Store)(nil)
	_ contracts.Lifecycle     = (*Store)(nil)
	_ contracts.StatsProvider = (*Store)(nil)
	// Suppress unused import warnings.
	_ = sync.Mutex{}
	_ = bytes.Equal
)
