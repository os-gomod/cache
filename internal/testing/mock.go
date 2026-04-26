package testing

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// MockCache is an in-memory mock implementation of contracts.Cache
// designed for unit testing. It stores all data in Go maps with a
// read-write mutex for thread safety.
//
// Features:
//   - Full contracts.Cache implementation
//   - Configurable error injection via InjectError
//   - TTL-based expiration via background timer
//   - Observable internal state via Data()
//
// Usage:
//
//	mock := NewMockCache()
//	mock.Set(ctx, "key", []byte("value"), time.Minute)
//	val, err := mock.Get(ctx, "key")
type MockCache struct {
	mu     sync.RWMutex
	data   map[string][]byte
	ttls   map[string]time.Time
	closed atomic.Bool
	name   string
	stats  contracts.StatsSnapshot

	// injectedErr is the error returned by the next operation (one-shot).
	// After being returned, it is cleared to nil.
	injectedErr error
}

// NewMockCache creates a new MockCache with an empty data store and
// statistics initialized to the current time.
func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
		name: "mock",
		stats: contracts.StatsSnapshot{
			StartTime: time.Now(),
		},
	}
}

// InjectError sets an error that will be returned by the next operation.
// The error is consumed after one use, so subsequent operations behave
// normally. This is useful for testing error handling paths.
func (m *MockCache) InjectError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.injectedErr = err
}

// consumeErr atomically returns and clears the injected error.
func (m *MockCache) consumeErr() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.injectedErr
	m.injectedErr = nil
	return err
}

// Data returns a shallow copy of the internal data map for test assertions.
// The caller should not mutate the returned map.
func (m *MockCache) Data() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]byte, len(m.data))
	for k, v := range m.data {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// -------------------------------------------------------------------
// Reader
// -------------------------------------------------------------------

// Get retrieves a value by key.
func (m *MockCache) Get(_ context.Context, key string) ([]byte, error) {
	if err := m.checkClosed("Get", key); err != nil {
		return nil, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return nil, err
	}
	if key == "" {
		atomic.AddInt64(&m.stats.Errors, 1)
		return nil, cacheerrors.Factory.InvalidKey("Get", key, nil)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.data[key]
	if !ok {
		atomic.AddInt64(&m.stats.Misses, 1)
		return nil, cacheerrors.Factory.NotFound("Get", key)
	}

	// Check expiration.
	if exp, hasExp := m.ttls[key]; hasExp && time.Now().After(exp) {
		atomic.AddInt64(&m.stats.Misses, 1)
		return nil, cacheerrors.Factory.NotFound("Get", key)
	}

	atomic.AddInt64(&m.stats.Hits, 1)
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

// GetMulti retrieves multiple values by key.
func (m *MockCache) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if err := m.checkClosed("GetMulti", ""); err != nil {
		return nil, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return nil, err
	}

	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		val, err := m.Get(ctx, key)
		if err == nil {
			result[key] = val
		}
	}
	return result, nil
}

// Exists reports whether a key exists and is not expired.
func (m *MockCache) Exists(_ context.Context, key string) (bool, error) {
	if err := m.checkClosed("Exists", key); err != nil {
		return false, err
	}
	if err := m.consumeErr(); err != nil {
		return false, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.data[key]
	if !ok {
		return false, nil
	}

	if exp, hasExp := m.ttls[key]; hasExp && time.Now().After(exp) {
		return false, nil
	}

	return true, nil
}

// TTL returns the remaining time-to-live for a key.
func (m *MockCache) TTL(_ context.Context, key string) (time.Duration, error) {
	if err := m.checkClosed("TTL", key); err != nil {
		return 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, hasExp := m.ttls[key]
	if !hasExp {
		return 0, nil // no TTL set
	}

	remaining := time.Until(exp)
	if remaining <= 0 {
		return 0, cacheerrors.Factory.NotFound("TTL", key)
	}
	return remaining, nil
}

// -------------------------------------------------------------------
// Writer
// -------------------------------------------------------------------

// Set stores a key-value pair with the given TTL.
func (m *MockCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if err := m.checkClosed("Set", key); err != nil {
		return err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return err
	}
	if key == "" {
		atomic.AddInt64(&m.stats.Errors, 1)
		return cacheerrors.Factory.InvalidKey("Set", key, nil)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[key] = cp

	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	} else {
		delete(m.ttls, key)
	}

	atomic.AddInt64(&m.stats.Sets, 1)
	m.updateItemsLocked()
	return nil
}

// SetMulti stores multiple key-value pairs.
func (m *MockCache) SetMulti(
	ctx context.Context,
	items map[string][]byte,
	ttl time.Duration,
) error {
	if err := m.checkClosed("SetMulti", ""); err != nil {
		return err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return err
	}

	for key, value := range items {
		if err := m.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a key from the cache.
func (m *MockCache) Delete(_ context.Context, key string) error {
	if err := m.checkClosed("Delete", key); err != nil {
		return err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	delete(m.ttls, key)
	atomic.AddInt64(&m.stats.Deletes, 1)
	m.updateItemsLocked()
	return nil
}

// DeleteMulti removes multiple keys.
func (m *MockCache) DeleteMulti(ctx context.Context, keys ...string) error {
	if err := m.checkClosed("DeleteMulti", ""); err != nil {
		return err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return err
	}

	for _, key := range keys {
		if err := m.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// -------------------------------------------------------------------
// AtomicOps
// -------------------------------------------------------------------

// CompareAndSwap atomically replaces oldVal with newVal if the current
// value matches oldVal.
func (m *MockCache) CompareAndSwap(
	_ context.Context,
	key string,
	oldVal, newVal []byte,
	ttl time.Duration,
) (bool, error) {
	if err := m.checkClosed("CompareAndSwap", key); err != nil {
		return false, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.data[key]
	if !ok {
		return false, nil
	}
	if !bytes.Equal(current, oldVal) {
		return false, cacheerrors.Factory.New(cacheerrors.CodeLockFailed, "CompareAndSwap", key,
			"compare-and-swap value mismatch", nil)
	}

	cp := make([]byte, len(newVal))
	copy(cp, newVal)
	m.data[key] = cp

	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}

	return true, nil
}

// SetNX sets a key-value pair only if the key does not exist.
func (m *MockCache) SetNX(
	_ context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	if err := m.checkClosed("SetNX", key); err != nil {
		return false, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.data[key]; ok {
		return false, cacheerrors.Factory.New(cacheerrors.CodeAlreadyExists, "SetNX", key,
			"key already exists", nil)
	}

	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[key] = cp

	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}

	atomic.AddInt64(&m.stats.Sets, 1)
	m.updateItemsLocked()
	return true, nil
}

// Increment atomically increments a numeric value by delta.
func (m *MockCache) Increment(_ context.Context, key string, delta int64) (int64, error) {
	if err := m.checkClosed("Increment", key); err != nil {
		return 0, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	current := int64(0)
	if raw, ok := m.data[key]; ok {
		parsed, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return 0, cacheerrors.Factory.New(cacheerrors.CodeInvalid, "Increment", key,
				fmt.Sprintf("value for key %q is not a valid integer", key), err)
		}
		current = parsed
	}

	newVal := current + delta
	m.data[key] = []byte(strconv.FormatInt(newVal, 10))
	atomic.AddInt64(&m.stats.Sets, 1)
	m.updateItemsLocked()
	return newVal, nil
}

// Decrement atomically decrements a numeric value by delta.
func (m *MockCache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return m.Increment(ctx, key, -delta)
}

// GetSet atomically sets a new value and returns the old value.
func (m *MockCache) GetSet(
	_ context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ([]byte, error) {
	if err := m.checkClosed("GetSet", key); err != nil {
		return nil, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.data[key]

	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[key] = cp

	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}

	atomic.AddInt64(&m.stats.Sets, 1)
	m.updateItemsLocked()

	if old == nil {
		return nil, cacheerrors.Factory.NotFound("GetSet", key)
	}
	result := make([]byte, len(old))
	copy(result, old)
	return result, nil
}

// -------------------------------------------------------------------
// Scanner
// -------------------------------------------------------------------

// Keys returns all keys matching the pattern (simple prefix match).
func (m *MockCache) Keys(_ context.Context, pattern string) ([]string, error) {
	if err := m.checkClosed("Keys", ""); err != nil {
		return nil, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	now := time.Now()
	for k, exp := range m.ttls {
		if now.After(exp) {
			continue
		}
		if pattern == "" || pattern == "*" {
			keys = append(keys, k)
		} else if len(k) >= len(pattern) && k[:len(pattern)] == pattern {
			keys = append(keys, k)
		}
	}

	// Also include keys without TTL.
	for k := range m.data {
		if _, hasExp := m.ttls[k]; hasExp {
			continue // already processed
		}
		if pattern == "" || pattern == "*" {
			keys = append(keys, k)
		}
	}

	return keys, nil
}

// Clear removes all entries.
func (m *MockCache) Clear(_ context.Context) error {
	if err := m.checkClosed("Clear", ""); err != nil {
		return err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	atomic.StoreInt64(&m.stats.Items, 0)
	return nil
}

// Size returns the number of entries.
func (m *MockCache) Size(_ context.Context) (int64, error) {
	if err := m.checkClosed("Size", ""); err != nil {
		return 0, err
	}
	if err := m.consumeErr(); err != nil {
		atomic.AddInt64(&m.stats.Errors, 1)
		return 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.data)), nil
}

// -------------------------------------------------------------------
// Lifecycle
// -------------------------------------------------------------------

// Ping checks cache liveness.
func (m *MockCache) Ping(_ context.Context) error {
	if m.closed.Load() {
		return cacheerrors.Factory.Closed("Ping")
	}
	return nil
}

// Close shuts down the mock cache. It is safe to call multiple times.
func (m *MockCache) Close(_ context.Context) error {
	m.closed.Store(true)
	return nil
}

// Closed reports whether the cache has been closed.
func (m *MockCache) Closed() bool {
	return m.closed.Load()
}

// Name returns the backend identifier.
func (m *MockCache) Name() string {
	return m.name
}

// -------------------------------------------------------------------
// StatsProvider
// -------------------------------------------------------------------

// Stats returns a snapshot of cache statistics.
func (m *MockCache) Stats() contracts.StatsSnapshot {
	return contracts.StatsSnapshot{
		Hits:        atomic.LoadInt64(&m.stats.Hits),
		Misses:      atomic.LoadInt64(&m.stats.Misses),
		Sets:        atomic.LoadInt64(&m.stats.Sets),
		Deletes:     atomic.LoadInt64(&m.stats.Deletes),
		Evictions:   atomic.LoadInt64(&m.stats.Evictions),
		Errors:      atomic.LoadInt64(&m.stats.Errors),
		Items:       atomic.LoadInt64(&m.stats.Items),
		MemoryBytes: atomic.LoadInt64(&m.stats.MemoryBytes),
		StartTime:   m.stats.StartTime,
	}
}

// -------------------------------------------------------------------
// HealthChecker
// -------------------------------------------------------------------

// Check performs a health probe.
func (m *MockCache) Check(_ context.Context) error {
	if m.closed.Load() {
		return cacheerrors.Factory.Closed("Check")
	}
	return nil
}

// -------------------------------------------------------------------
// Internal helpers
// -------------------------------------------------------------------

// checkClosed returns an error if the cache is closed.
func (m *MockCache) checkClosed(op, _key string) error {
	if m.closed.Load() {
		return cacheerrors.Factory.Closed(op)
	}
	return nil
}

// updateItemsLocked updates the item count. Must be called with m.mu held.
func (m *MockCache) updateItemsLocked() {
	atomic.StoreInt64(&m.stats.Items, int64(len(m.data)))

	var totalBytes int64
	for _, v := range m.data {
		totalBytes += int64(len(v))
	}
	atomic.StoreInt64(&m.stats.MemoryBytes, totalBytes)
}

// ResetStats resets all statistics counters to zero.
func (m *MockCache) ResetStats() {
	atomic.StoreInt64(&m.stats.Hits, 0)
	atomic.StoreInt64(&m.stats.Misses, 0)
	atomic.StoreInt64(&m.stats.Sets, 0)
	atomic.StoreInt64(&m.stats.Deletes, 0)
	atomic.StoreInt64(&m.stats.Evictions, 0)
	atomic.StoreInt64(&m.stats.Errors, 0)
}
