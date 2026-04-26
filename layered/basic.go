package layered

import (
	"bytes"
	"context"
	"strconv"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/keyutil"
	"github.com/os-gomod/cache/v2/internal/runtime"
)

// Get retrieves a value from the layered cache. It first checks L1; on miss
// it checks L2 and optionally promotes the value to L1.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkClosed("layered.get"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "get",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]byte, error) {
		// Try L1 first
		val, err := s.l1.Get(ctx, key)
		if err == nil {
			s.stats.L1Hit()
			return val, nil
		}

		// L1 miss - check if it's a negative cache entry
		s.stats.L1Miss()

		// Try L2
		val, err = s.l2.Get(ctx, key)
		if err != nil {
			s.stats.L2Miss()
			s.stats.Miss()

			// Cache negative result in L1 if configured
			if s.cfg.negativeTTL > 0 && errors.Factory.IsNotFound(err) {
				s.promoteToL1(ctx, key, nil, s.cfg.negativeTTL)
			}
			return nil, err
		}

		s.stats.L2Hit()
		s.stats.Hit()

		// Promote to L1
		if s.cfg.promoteOnHit {
			s.promoteToL1(ctx, key, val, s.resolveTTLFromL2(ctx, key))
		}

		return val, nil
	})
}

// GetMulti retrieves multiple values. For each key, it checks L1 first,
// then L2, collecting all found values.
func (s *Store) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}
	if err := s.checkClosed("layered.get_multi"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:     "get_multi",
		KeyCount: len(keys),
		Backend:  "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(
		s.executor,
		ctx,
		op,
		func(ctx context.Context) (map[string][]byte, error) {
			result := make(map[string][]byte, len(keys))
			var l2Keys []string
			var l2Indices []int

			// Check L1 for all keys
			l1Result, err := s.l1.GetMulti(ctx, keys...)
			if err != nil {
				// If L1 fails entirely, fall through to L2 for all keys
				l2Keys = keys
				for i := range keys {
					l2Indices = append(l2Indices, i)
				}
				s.stats.L1Error()
			} else {
				for i, key := range keys {
					if val, ok := l1Result[key]; ok {
						result[key] = val
						s.stats.L1Hit()
					} else {
						l2Keys = append(l2Keys, key)
						l2Indices = append(l2Indices, i)
						s.stats.L1Miss()
					}
				}
			}

			// Check L2 for keys not found in L1
			if len(l2Keys) > 0 {
				l2Result, l2Err := s.l2.GetMulti(ctx, l2Keys...)
				if l2Err == nil {
					for _, key := range l2Keys {
						if val, ok := l2Result[key]; ok {
							result[key] = val
							s.stats.L2Hit()
							s.stats.Hit()

							if s.cfg.promoteOnHit {
								s.promoteToL1(ctx, key, val, s.resolveTTLFromL2(ctx, key))
							}
						} else {
							s.stats.L2Miss()
							s.stats.Miss()
						}
					}
				} else {
					s.stats.L2Error()
				}
			}

			return result, nil
		},
	)
}

// Set stores a value in both L1 and L2. If write-back is enabled, the L2
// write is queued asynchronously.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := keyutil.ValidateKey("layered.set", key); err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return err
	}
	if err := s.checkClosed("layered.set"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "set",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		// Always write to L1 first (fast path)
		if err := s.l1.Set(ctx, key, value, ttl); err != nil {
			s.stats.L1Error()
			s.stats.ErrorOp()
			return err
		}

		// Write to L2
		if s.cfg.writeBack && s.wbCh != nil {
			// Async write-back
			select {
			case s.wbCh <- writeBackJob{
				key:   key,
				value: value,
				ttl:   ttl,
			}:
				s.stats.WriteBackEnqueue()
			default:
				// Queue full - drop the write (or do sync write)
				s.stats.WriteBackDrop()
				// Fallback to sync write to avoid data loss
				if err := s.l2.Set(ctx, key, value, ttl); err != nil {
					s.stats.L2Error()
					s.stats.ErrorOp()
					return err
				}
			}
		} else {
			// Synchronous write-through
			if err := s.l2.Set(ctx, key, value, ttl); err != nil {
				s.stats.L2Error()
				s.stats.ErrorOp()
				return err
			}
		}

		s.stats.SetOp()
		return nil
	})
}

// SetMulti stores multiple key-value pairs in both L1 and L2.
func (s *Store) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}
	if err := s.checkClosed("layered.set_multi"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:     "set_multi",
		KeyCount: len(items),
		Backend:  "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		// Write to L1
		if err := s.l1.SetMulti(ctx, items, ttl); err != nil {
			s.stats.L1Error()
			s.stats.ErrorOp()
			return err
		}

		// Write to L2 (sync for multi)
		if err := s.l2.SetMulti(ctx, items, ttl); err != nil {
			s.stats.L2Error()
			s.stats.ErrorOp()
			return err
		}

		s.stats.SetOp()
		return nil
	})
}

// Delete removes a key from both L1 and L2.
//
//nolint:dupl // structural similarity with DeleteMulti is intentional
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := keyutil.ValidateKey("layered.delete", key); err != nil {
		//nolint:wrapcheck // error is already wrapped by internal packages
		return err
	}
	if err := s.checkClosed("layered.delete"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "delete",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		// Delete from both (best effort)
		_ = s.l1.Delete(ctx, key)
		_ = s.l2.Delete(ctx, key)
		s.stats.DeleteOp()
		return nil
	})
}

// DeleteMulti removes multiple keys from both L1 and L2.
//
//nolint:dupl // structural similarity with Delete is intentional
func (s *Store) DeleteMulti(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.checkClosed("layered.delete_multi"); err != nil {
		return err
	}

	//nolint:dupl // structural similarity is intentional
	op := contracts.Operation{
		Name:     "delete_multi",
		KeyCount: len(keys),
		Backend:  "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		_ = s.l1.DeleteMulti(ctx, keys...)
		_ = s.l2.DeleteMulti(ctx, keys...)
		s.stats.DeleteOp()
		return nil
	})
}

// Exists checks if a key exists in L1 first, then L2.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.checkClosed("layered.exists"); err != nil {
		return false, err
	}

	op := contracts.Operation{
		Name:    "exists",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		// Try L1
		exists, err := s.l1.Exists(ctx, key)
		if err == nil && exists {
			s.stats.L1Hit()
			return true, nil
		}

		s.stats.L1Miss()

		// Try L2
		exists, err = s.l2.Exists(ctx, key)
		if err != nil {
			s.stats.L2Error()
			return false, err
		}
		if exists {
			s.stats.L2Hit()
			s.stats.Hit()
		} else {
			s.stats.L2Miss()
			s.stats.Miss()
		}
		return exists, nil
	})
}

// TTL returns the remaining TTL from L1 if available, otherwise from L2.
func (s *Store) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := s.checkClosed("layered.ttl"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "ttl",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(
		s.executor,
		ctx,
		op,
		func(ctx context.Context) (time.Duration, error) {
			// Try L1 first
			ttl, err := s.l1.TTL(ctx, key)
			if err == nil {
				return ttl, nil
			}

			// Fall back to L2
			return s.l2.TTL(ctx, key)
		},
	)
}

// Keys returns all keys from L1 (which is typically faster and has the
// authoritative set of actively cached keys).
func (s *Store) Keys(ctx context.Context, pattern string) ([]string, error) {
	if err := s.checkClosed("layered.keys"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "keys",
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]string, error) {
		return s.l1.Keys(ctx, pattern)
	})
}

// Clear clears both L1 and L2 caches.
func (s *Store) Clear(ctx context.Context) error {
	if err := s.checkClosed("layered.clear"); err != nil {
		return err
	}

	op := contracts.Operation{
		Name:    "clear",
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return s.executor.Execute(ctx, op, func(ctx context.Context) error {
		_ = s.l1.Clear(ctx)
		_ = s.l2.Clear(ctx)
		s.stats.Reset()
		return nil
	})
}

// Size returns the number of entries in L1 (the authoritative count).
func (s *Store) Size(ctx context.Context) (int64, error) {
	if err := s.checkClosed("layered.size"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "size",
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (int64, error) {
		return s.l1.Size(ctx)
	})
}

// CompareAndSwap performs CAS on both L1 and L2.
func (s *Store) CompareAndSwap(
	ctx context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := s.checkClosed("layered.cas"); err != nil {
		return false, err
	}

	op := contracts.Operation{
		Name:    "compare_and_swap",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		// CAS on L2 first (source of truth)
		swapped, err := s.l2.CompareAndSwap(ctx, key, oldVal, newVal, ttl)
		if err != nil || !swapped {
			return swapped, err
		}

		// Update L1
		_ = s.l1.Set(ctx, key, newVal, ttl)
		s.stats.SetOp()
		return true, nil
	})
}

// SetNX sets a key in both L1 and L2 only if it doesn't exist.
func (s *Store) SetNX(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := s.checkClosed("layered.setnx"); err != nil {
		return false, err
	}

	op := contracts.Operation{
		Name:    "setnx",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (bool, error) {
		set, err := s.l2.SetNX(ctx, key, value, ttl)
		if err != nil || !set {
			return set, err
		}

		// Update L1
		_ = s.l1.Set(ctx, key, value, ttl)
		s.stats.SetOp()
		return true, nil
	})
}

// Increment increments a counter in both L1 and L2.
func (s *Store) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := s.checkClosed("layered.increment"); err != nil {
		return 0, err
	}

	op := contracts.Operation{
		Name:    "increment",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) (int64, error) {
		// Increment on L2 (source of truth)
		val, err := s.l2.Increment(ctx, key, delta)
		if err != nil {
			return 0, err
		}

		// Update L1
		_ = s.l1.Set(ctx, key, []byte(strconv.FormatInt(val, 10)), 0)
		s.stats.SetOp()
		return val, nil
	})
}

// Decrement decrements a counter in both L1 and L2.
func (s *Store) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return s.Increment(ctx, key, -delta)
}

// GetSet atomically sets a new value and returns the old value.
func (s *Store) GetSet(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := s.checkClosed("layered.getset"); err != nil {
		return nil, err
	}

	op := contracts.Operation{
		Name:    "getset",
		Key:     key,
		Backend: "layered",
	}

	//nolint:wrapcheck // error is already wrapped by internal packages
	return runtime.ExecuteTyped(s.executor, ctx, op, func(ctx context.Context) ([]byte, error) {
		// GetSet on L2 (source of truth)
		oldVal, err := s.l2.GetSet(ctx, key, value, ttl)
		if err != nil {
			return nil, err
		}

		// Update L1
		_ = s.l1.Set(ctx, key, value, ttl)
		s.stats.SetOp()
		return oldVal, nil
	})
}

// Suppress unused imports.
var _ = bytes.Equal
