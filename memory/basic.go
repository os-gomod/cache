package memory

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/keyutil"
	"github.com/os-gomod/cache/v2/internal/runtime"
)

// Get retrieves the value for the given key from the in-memory store.
// Returns errors.NotFound if the key does not exist or has expired.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.validateKey("memory.get", key); err != nil {
		return nil, err
	}
	if err := s.checkClosed("memory.get"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "get",
		Key:     key,
		Backend: "memory",
	}

	value, err := runtime.ExecuteTyped(
		s.executor,
		ctx,
		op,
		func(_ctx context.Context) ([]byte, error) {
			sh := s.shardFor(key)
			now := time.Now().UnixNano()

			sh.mu.RLock()
			e, ok := sh.get(key)
			if !ok || e.IsExpired(now) {
				sh.mu.RUnlock()
				s.stats.Miss()
				return nil, errors.Factory.NotFound("memory.get", key)
			}
			value := make([]byte, len(e.Value))
			copy(value, e.Value)
			e.Touch(now)
			sh.mu.RUnlock()

			s.stats.Hit()
			return value, nil
		},
	)

	//nolint:wrapcheck // error is already wrapped by internal packages
	return value, err
}

// GetMulti retrieves multiple values by key. Missing keys are omitted from
// the returned map.
func (s *Store) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := keyutil.ValidateKeys("memory.get_multi", keys); err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return nil, err
	}
	if err := s.checkClosed("memory.get_multi"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:     "get_multi",
		KeyCount: len(keys),
		Backend:  "memory",
	}

	result, err := runtime.ExecuteTyped(
		s.executor,
		ctx,
		op,
		func(_ctx context.Context) (map[string][]byte, error) {
			now := time.Now().UnixNano()
			m := make(map[string][]byte, len(keys))

			for _, key := range keys {
				sh := s.shardFor(key)
				sh.mu.RLock()
				e, ok := sh.get(key)
				if ok && !e.IsExpired(now) {
					value := make([]byte, len(e.Value))
					copy(value, e.Value)
					e.Touch(now)
					m[key] = value
					s.stats.Hit()
				} else {
					s.stats.Miss()
				}
				sh.mu.RUnlock()
			}
			return m, nil
		},
	)

	//nolint:wrapcheck // error is already wrapped by internal packages
	return result, err
}

// Set stores a key-value pair with the given TTL. If ttl is zero or negative,
// the default TTL from the configuration is used.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.validateKey("memory.set", key); err != nil {
		return err
	}
	if err := s.checkClosed("memory.set"); err != nil {
		return err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:    "set",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(_ctx context.Context) error {
		now := time.Now().UnixNano()
		e := NewEntry(key, value, effectiveTTL, now)
		sh := s.shardFor(key)

		sh.mu.Lock()
		perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
		ev := sh.set(key, e, perShardMax)
		sh.mu.Unlock()

		if len(ev.keys) > 0 {
			s.stats.EvictionOp()
			s.stats.AddMemory(-ev.bytes)
		}
		s.stats.SetOp()
		s.stats.AddItems(1)
		s.stats.AddMemory(e.Size)

		return nil
	})
}

// SetMulti stores multiple key-value pairs with the given TTL.
func (s *Store) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}
	if err := s.checkClosed("memory.set_multi"); err != nil {
		return err
	}

	effectiveTTL := s.resolveTTL(ttl)

	op := contracts.Operation{
		Name:     "set_multi",
		KeyCount: len(items),
		Backend:  "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(_ctx context.Context) error {
		now := time.Now().UnixNano()
		for key, value := range items {
			e := NewEntry(key, value, effectiveTTL, now)
			sh := s.shardFor(key)
			sh.mu.Lock()
			perShardMax := s.cfg.maxMemoryBytes / int64(len(s.shards))
			ev := sh.set(key, e, perShardMax)
			sh.mu.Unlock()

			if len(ev.keys) > 0 {
				s.stats.EvictionOp()
			}
			s.stats.SetOp()
			s.stats.AddItems(1)
			s.stats.AddMemory(e.Size)
		}
		return nil
	})
}

// Delete removes a key from the store. Deleting a non-existent key is a no-op.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.validateKey("memory.delete", key); err != nil {
		return err
	}
	if err := s.checkClosed("memory.delete"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "delete",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(_ctx context.Context) error {
		sh := s.shardFor(key)
		sh.mu.Lock()
		e, ok := sh.delete(key)
		sh.mu.Unlock()

		if ok {
			s.stats.DeleteOp()
			s.stats.AddItems(-1)
			s.stats.AddMemory(-e.Size)
		}
		return nil
	})
}

// DeleteMulti removes multiple keys from the store.
func (s *Store) DeleteMulti(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.checkClosed("memory.delete_multi"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:     "delete_multi",
		KeyCount: len(keys),
		Backend:  "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(_ctx context.Context) error {
		for _, key := range keys {
			sh := s.shardFor(key)
			sh.mu.Lock()
			e, ok := sh.delete(key)
			sh.mu.Unlock()

			if ok {
				s.stats.DeleteOp()
				s.stats.AddItems(-1)
				s.stats.AddMemory(-e.Size)
			}
		}
		return nil
	})
}

// Exists reports whether a non-expired key exists in the store.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.validateKey("memory.exists", key); err != nil {
		return false, err
	}
	if err := s.checkClosed("memory.exists"); err != nil {
		return false, err
	}

	op := contracts.Operation{
		Name:    "exists",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(_ctx context.Context) (bool, error) {
		sh := s.shardFor(key)
		now := time.Now().UnixNano()

		sh.mu.RLock()
		e, ok := sh.get(key)
		if ok && e.IsExpired(now) {
			ok = false
		}
		sh.mu.RUnlock()

		return ok, nil
	})
}

// TTL returns the remaining time-to-live for the given key. If the key has
// no expiration, a duration greater than 365 days is returned. Returns an
// error if the key does not exist or has expired.
func (s *Store) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := s.validateKey("memory.ttl", key); err != nil {
		return 0, err
	}
	if err := s.checkClosed("memory.ttl"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "ttl",
		Key:     key,
		Backend: "memory",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(
		s.executor,
		ctx,
		op,
		func(_ctx context.Context) (time.Duration, error) {
			sh := s.shardFor(key)
			now := time.Now().UnixNano()

			sh.mu.RLock()
			e, ok := sh.get(key)
			if !ok || e.IsExpired(now) {
				sh.mu.RUnlock()
				return 0, errors.Factory.NotFound("memory.ttl", key)
			}
			ttl := e.TTL(now)
			sh.mu.RUnlock()

			return ttl, nil
		},
	)
}
