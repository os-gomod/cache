package testing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/cache/v2/internal/contracts"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// CacheSuite is a table-driven contract test suite for contracts.Cache.
// Backend authors provide a factory function that creates a fresh cache
// instance for each sub-test, and RunAll exercises every required behaviour.
//
// Usage:
//
//	func TestMyBackendCache(t *testing.T) {
//	    suite := &testing.CacheSuite{
//	        NewCache: func(t *testing.T) contracts.Cache {
//	            return NewMyBackend(t)
//	        },
//	    }
//	    suite.RunAll(t)
//	}
type CacheSuite struct {
	// NewCache creates a fresh, empty cache instance for each test.
	// The returned cache must be fully initialized and ready to accept
	// operations. The test suite calls Close on the cache after each test.
	NewCache func(t *testing.T) contracts.Cache

	// SkipTests is an optional set of test names to skip. This is useful
	// for backends that do not support certain operations (e.g. a minimal
	// backend may not implement TTL-based expiration).
	SkipTests map[string]bool
}

// skip returns true if the given test name should be skipped.
func (s *CacheSuite) skip(name string) bool {
	if s.SkipTests == nil {
		return false
	}
	return s.SkipTests[name]
}

// TestGetSetDelete verifies basic set, get, and delete operations.
func (s *CacheSuite) TestGetSetDelete(t *testing.T) {
	if s.skip("TestGetSetDelete") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	err := cache.Set(ctx, "key1", []byte("value1"), 0)
	require.NoError(t, err)

	val, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)

	err = cache.Delete(ctx, "key1")
	require.NoError(t, err)
}

// TestGetNotFound verifies that Get returns an error for missing keys.
func (s *CacheSuite) TestGetNotFound(t *testing.T) {
	if s.skip("TestGetNotFound") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	_, err := cache.Get(ctx, "nonexistent")
	require.Error(t, err)
}

// TestGetMulti verifies multi-key retrieval.
func (s *CacheSuite) TestGetMulti(t *testing.T) {
	if s.skip("TestGetMulti") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, cache.Set(ctx, "b", []byte("2"), 0))
	require.NoError(t, cache.Set(ctx, "c", []byte("3"), 0))

	results, err := cache.GetMulti(ctx, "a", "b", "missing")
	require.NoError(t, err)
	assert.Equal(t, []byte("1"), results["a"])
	assert.Equal(t, []byte("2"), results["b"])
	_, found := results["missing"]
	assert.False(t, found, "missing key should not be in results")
}

// TestSetMulti verifies multi-key set operations.
func (s *CacheSuite) TestSetMulti(t *testing.T) {
	if s.skip("TestSetMulti") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	items := map[string][]byte{
		"x": []byte("10"),
		"y": []byte("20"),
	}
	err := cache.SetMulti(ctx, items, 0)
	require.NoError(t, err)

	val, err := cache.Get(ctx, "x")
	require.NoError(t, err)
	assert.Equal(t, []byte("10"), val)
}

// TestDeleteMulti verifies multi-key deletion.
func (s *CacheSuite) TestDeleteMulti(t *testing.T) {
	if s.skip("TestDeleteMulti") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "d1", []byte("v"), 0))
	require.NoError(t, cache.Set(ctx, "d2", []byte("v"), 0))

	err := cache.DeleteMulti(ctx, "d1", "d2")
	require.NoError(t, err)

	_, err = cache.Get(ctx, "d1")
	assert.Error(t, err)
}

// TestExists verifies the Exists operation.
func (s *CacheSuite) TestExists(t *testing.T) {
	if s.skip("TestExists") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	exists, err := cache.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, cache.Set(ctx, "yes", []byte("v"), 0))
	exists, err = cache.Exists(ctx, "yes")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestTTL verifies that TTL-based expiration works correctly.
func (s *CacheSuite) TestTTL(t *testing.T) {
	if s.skip("TestTTL") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "ttl-key", []byte("v"), 100*time.Millisecond))

	// Should exist immediately.
	exists, err := cache.Exists(ctx, "ttl-key")
	require.NoError(t, err)
	assert.True(t, exists)

	// Wait for expiration.
	Wait(t, 150*time.Millisecond)

	exists, err = cache.Exists(ctx, "ttl-key")
	require.NoError(t, err)
	assert.False(t, exists, "key should be expired")
}

// TestKeys verifies key enumeration.
func (s *CacheSuite) TestKeys(t *testing.T) {
	if s.skip("TestKeys") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "k1", []byte("v"), 0))
	require.NoError(t, cache.Set(ctx, "k2", []byte("v"), 0))

	keys, err := cache.Keys(ctx, "*")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 2)
}

// TestClear verifies that Clear removes all entries.
func (s *CacheSuite) TestClear(t *testing.T) {
	if s.skip("TestClear") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "c1", []byte("v"), 0))
	require.NoError(t, cache.Set(ctx, "c2", []byte("v"), 0))

	err := cache.Clear(ctx)
	require.NoError(t, err)

	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)
}

// TestSize verifies the Size operation.
func (s *CacheSuite) TestSize(t *testing.T) {
	if s.skip("TestSize") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	require.NoError(t, cache.Set(ctx, "s1", []byte("v"), 0))
	size, err = cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), size)
}

// TestCompareAndSwap verifies atomic compare-and-swap.
func (s *CacheSuite) TestCompareAndSwap(t *testing.T) {
	if s.skip("TestCompareAndSwap") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "cas", []byte("old"), 0))

	swapped, err := cache.CompareAndSwap(ctx, "cas", []byte("old"), []byte("new"), 0)
	require.NoError(t, err)
	assert.True(t, swapped)

	val, err := cache.Get(ctx, "cas")
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), val)

	// Second swap with wrong old value should fail.
	swapped, err = cache.CompareAndSwap(ctx, "cas", []byte("wrong"), []byte("newer"), 0)
	require.NoError(t, err)
	assert.False(t, swapped)
}

// TestSetNX verifies the set-if-not-exists operation.
func (s *CacheSuite) TestSetNX(t *testing.T) {
	if s.skip("TestSetNX") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()

	set, err := cache.SetNX(ctx, "nx", []byte("first"), 0)
	require.NoError(t, err)
	assert.True(t, set)

	set, err = cache.SetNX(ctx, "nx", []byte("second"), 0)
	require.NoError(t, err)
	assert.False(t, set, "SetNX should fail when key already exists")

	val, err := cache.Get(ctx, "nx")
	require.NoError(t, err)
	assert.Equal(t, []byte("first"), val)
}

// TestIncrement verifies atomic increment.
//
//nolint:dupl // structural similarity is intentional
func (s *CacheSuite) TestIncrement(t *testing.T) {
	if s.skip("TestIncrement") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()

	val, err := cache.Increment(ctx, "counter", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = cache.Increment(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

// TestDecrement verifies atomic decrement.
//
//nolint:dupl // structural similarity is intentional
func (s *CacheSuite) TestDecrement(t *testing.T) {
	if s.skip("TestDecrement") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()

	// Initialize to 10 via increment.
	val, err := cache.Increment(ctx, "dec", 10)
	require.NoError(t, err)
	assert.Equal(t, int64(10), val)

	val, err = cache.Decrement(ctx, "dec", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(7), val)
}

// TestGetSet verifies the atomic get-and-set operation.
func (s *CacheSuite) TestGetSet(t *testing.T) {
	if s.skip("TestGetSet") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "gs", []byte("old"), 0))

	old, err := cache.GetSet(ctx, "gs", []byte("new"), 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), old)

	val, err := cache.Get(ctx, "gs")
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), val)
}

// TestClose verifies that Close completes without error.
func (s *CacheSuite) TestClose(t *testing.T) {
	if s.skip("TestClose") {
		t.Skip()
	}
	cache := s.NewCache(t)
	err := cache.Close(context.Background())
	assert.NoError(t, err)
}

// TestDoubleClose verifies that Close is idempotent.
func (s *CacheSuite) TestDoubleClose(t *testing.T) {
	if s.skip("TestDoubleClose") {
		t.Skip()
	}
	cache := s.NewCache(t)
	assert.NoError(t, cache.Close(context.Background()))
	assert.NoError(t, cache.Close(context.Background()))
}

// TestPing verifies that Ping succeeds on a healthy cache.
func (s *CacheSuite) TestPing(t *testing.T) {
	if s.skip("TestPing") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	err := cache.Ping(context.Background())
	assert.NoError(t, err)
}

// TestStats verifies that Stats returns a valid snapshot.
func (s *CacheSuite) TestStats(t *testing.T) {
	if s.skip("TestStats") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	snapshot := cache.Stats()
	assert.False(t, snapshot.StartTime.IsZero())

	// Perform some operations to generate stats.
	require.NoError(t, cache.Set(ctx, "stat-key", []byte("v"), 0))
	_, _ = cache.Get(ctx, "stat-key")
	_, _ = cache.Get(ctx, "nonexistent") // miss

	snapshot = cache.Stats()
	assert.GreaterOrEqual(t, snapshot.Sets, int64(1))
	assert.GreaterOrEqual(t, snapshot.Hits, int64(1))
	assert.GreaterOrEqual(t, snapshot.Misses, int64(1))
}

// TestClosedAfterClose verifies that operations fail after Close.
func (s *CacheSuite) TestClosedAfterClose(t *testing.T) {
	if s.skip("TestClosedAfterClose") {
		t.Skip()
	}
	cache := s.NewCache(t)

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "pre-close", []byte("v"), 0))
	require.NoError(t, cache.Close(context.Background()))
	assert.True(t, cache.Closed())

	// Operations should fail after close.
	_, err := cache.Get(ctx, "pre-close")
	assert.Error(t, err)
}

// TestEmptyKey verifies that empty keys are rejected.
func (s *CacheSuite) TestEmptyKey(t *testing.T) {
	if s.skip("TestEmptyKey") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	err := cache.Set(ctx, "", []byte("v"), 0)
	assert.Error(t, err, "empty key should be rejected")

	_, err = cache.Get(ctx, "")
	assert.Error(t, err, "empty key get should be rejected")
}

// TestLargeValue verifies that large values are handled correctly.
func (s *CacheSuite) TestLargeValue(t *testing.T) {
	if s.skip("TestLargeValue") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	large := make([]byte, 1024*1024) // 1 MiB
	for i := range large {
		large[i] = byte(i % 256)
	}

	err := cache.Set(ctx, "large", large, 0)
	require.NoError(t, err)

	val, err := cache.Get(ctx, "large")
	require.NoError(t, err)
	assert.Equal(t, large, val)
}

// TestConcurrentAccess verifies that the cache handles concurrent
// operations without data races.
func (s *CacheSuite) TestConcurrentAccess(t *testing.T) {
	if s.skip("TestConcurrentAccess") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	const goroutines = 20
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range opsPerGoroutine {
				key := fmt.Sprintf("concurrent-%d-%d", id, j)
				_ = cache.Set(ctx, key, []byte("v"), 0)
				_, _ = cache.Get(ctx, key)
				_ = cache.Delete(ctx, key)
			}
		}(i)
	}
	wg.Wait()

	// After all concurrent ops, the cache should be empty (all deleted).
	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)
}

// TestExpiration verifies that entries with short TTLs are expired
// and become inaccessible.
func (s *CacheSuite) TestExpiration(t *testing.T) {
	if s.skip("TestExpiration") {
		t.Skip()
	}
	cache := s.NewCache(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "exp1", []byte("v1"), 50*time.Millisecond))
	require.NoError(t, cache.Set(ctx, "exp2", []byte("v2"), 0)) // no expiration

	Wait(t, 100*time.Millisecond)

	_, err := cache.Get(ctx, "exp1")
	assert.Error(t, err, "expired key should return error")

	val, err := cache.Get(ctx, "exp2")
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), val)
}

// RunAll executes every test in the suite as a sub-test of t. Each
// sub-test gets a fresh cache instance from NewCache.
func (s *CacheSuite) RunAll(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"TestGetSetDelete", s.TestGetSetDelete},
		{"TestGetNotFound", s.TestGetNotFound},
		{"TestGetMulti", s.TestGetMulti},
		{"TestSetMulti", s.TestSetMulti},
		{"TestDeleteMulti", s.TestDeleteMulti},
		{"TestExists", s.TestExists},
		{"TestTTL", s.TestTTL},
		{"TestKeys", s.TestKeys},
		{"TestClear", s.TestClear},
		{"TestSize", s.TestSize},
		{"TestCompareAndSwap", s.TestCompareAndSwap},
		{"TestSetNX", s.TestSetNX},
		{"TestIncrement", s.TestIncrement},
		{"TestDecrement", s.TestDecrement},
		{"TestGetSet", s.TestGetSet},
		{"TestClose", s.TestClose},
		{"TestDoubleClose", s.TestDoubleClose},
		{"TestPing", s.TestPing},
		{"TestStats", s.TestStats},
		{"TestClosedAfterClose", s.TestClosedAfterClose},
		{"TestEmptyKey", s.TestEmptyKey},
		{"TestLargeValue", s.TestLargeValue},
		{"TestConcurrentAccess", s.TestConcurrentAccess},
		{"TestExpiration", s.TestExpiration},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}

// ---------------------------------------------------------------------------
// AssertCacheError
// ---------------------------------------------------------------------------

// AssertCacheError is a test helper that asserts err is a *cacheerrors.CacheError
// with the expected error code.
func AssertCacheError(t *testing.T, err error, code cacheerrors.ErrorCode) {
	t.Helper()
	require.Error(t, err)

	var cerr *cacheerrors.CacheError
	require.True(t, errors.As(err, &cerr), "error must be a CacheError, got: %T", err)
	assert.Equal(t, code, cerr.Code, "error code mismatch")
}
