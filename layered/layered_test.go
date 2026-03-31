// Package layered_test provides tests for two-level (L1 + L2) cache.
package layered_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/cache/config"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/layered"
)

// testRedisAvailable checks if Redis is available for integration tests.
func testRedisAvailable(t *testing.T) {
	t.Helper()
	cache, err := layered.New(
		layered.WithL2Address("localhost:6379"),
		layered.WithL2DB(10),
	)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	_ = cache.Close(context.Background())
}

// setupTestCache creates a cache with default test configuration.
func setupTestCache(t *testing.T) *layered.Cache {
	t.Helper()
	testRedisAvailable(t)

	cache, err := layered.New(
		layered.WithL1MaxEntries(1000),
		layered.WithL1TTL(2*time.Minute),
		layered.WithL1CleanupInterval(30*time.Second),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2DB(10),
		layered.WithL2KeyPrefix("layered_test:"),
		layered.WithL2TTL(30*time.Minute),
		layered.WithPromoteOnHit(true),
		layered.WithNegativeTTL(30*time.Second),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_ = cache.Clear(ctx)

	return cache
}

// setupShortL1TTLTestCache creates a cache with short L1 TTL for expiration testing.
func setupShortL1TTLTestCache(t *testing.T) *layered.Cache {
	t.Helper()
	testRedisAvailable(t)

	cache, err := layered.New(
		layered.WithL1MaxEntries(1000),
		layered.WithL1TTL(50*time.Millisecond),
		layered.WithL1CleanupInterval(25*time.Millisecond),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2DB(10),
		layered.WithL2KeyPrefix("layered_test:"),
		layered.WithL2TTL(30*time.Minute),
		layered.WithPromoteOnHit(true),
		layered.WithNegativeTTL(30*time.Second),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_ = cache.Clear(ctx)

	return cache
}

// ----------------------------------------------------------------------------
// Constructor Tests
// ----------------------------------------------------------------------------

func TestNew(t *testing.T) {
	testRedisAvailable(t)

	cache, err := layered.New(
		layered.WithL1MaxEntries(100),
		layered.WithL2Address("localhost:6379"),
	)

	require.NoError(t, err)
	defer func() { _ = cache.Close(context.Background()) }()
	assert.NotNil(t, cache)
}

func TestNewWithConfig(t *testing.T) {
	testRedisAvailable(t)

	cfg := config.DefaultLayered()
	cfg.L1Config.MaxEntries = 500
	cfg.L2Config.Addr = "localhost:6379"
	cfg.L2Config.DB = 10

	cache, err := layered.NewWithConfig(cfg)

	require.NoError(t, err)
	defer func() { _ = cache.Close(context.Background()) }()
	assert.NotNil(t, cache)
}

func TestNewWithConfig_Nil(t *testing.T) {
	testRedisAvailable(t)

	cache, err := layered.NewWithConfig(nil)

	require.NoError(t, err)
	assert.NotNil(t, cache)
	_ = cache.Close(context.Background())
}

func TestNewWithContext(t *testing.T) {
	testRedisAvailable(t)

	ctx := context.Background()
	cache, err := layered.NewWithContext(ctx,
		layered.WithL1MaxEntries(100),
		layered.WithL2Address("localhost:6379"),
	)

	require.NoError(t, err)
	assert.NotNil(t, cache)
	_ = cache.Close(context.Background())
}

// ----------------------------------------------------------------------------
// Basic Operations Tests
// ----------------------------------------------------------------------------

func TestCache_Get_Set(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "get_set_test"
	value := []byte("test-value")

	err := cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

func TestCache_Get_FromL2AfterL1Expiry(t *testing.T) {
	cache := setupShortL1TTLTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "l2_fallback_test"
	value := []byte("persistent-value")

	err := cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	time.Sleep(120 * time.Millisecond)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	stats := cache.Stats()
	assert.Greater(t, stats.L2Promotions, int64(0))
}

func TestCache_Get_NotFound(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	_, err := cache.Get(ctx, "nonexistent")
	assert.Error(t, err)
	assert.True(t, _errors.IsNotFound(err))
}

func TestCache_Get_NegativeCaching(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "negative_test"

	_, err := cache.Get(ctx, key)
	assert.Error(t, err)

	_, err = cache.Get(ctx, key)
	assert.Error(t, err)

	stats := cache.Stats()
	assert.Greater(t, stats.L1Hits, int64(0))
}

func TestCache_Delete(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "delete_test"
	value := []byte("value")

	err := cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	err = cache.Delete(ctx, key)
	require.NoError(t, err)

	_, err = cache.Get(ctx, key)
	assert.Error(t, err)
}

func TestCache_Exists(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "exists_test"

	exists, err := cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	err = cache.Set(ctx, key, []byte("value"), 0)
	require.NoError(t, err)

	exists, err = cache.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCache_TTL(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "ttl_test"

	err := cache.Set(ctx, key, []byte("value"), 30*time.Second)
	require.NoError(t, err)

	ttl, err := cache.TTL(ctx, key)
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 30*time.Second)
}

// ----------------------------------------------------------------------------
// Multi Operations Tests
// ----------------------------------------------------------------------------

func TestCache_GetMulti(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	items := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	for k, v := range items {
		err := cache.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	results, err := cache.GetMulti(ctx, "key1", "key2", "key3", "key4")
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, items["key1"], results["key1"])
	assert.Equal(t, items["key2"], results["key2"])
	assert.Equal(t, items["key3"], results["key3"])
}

func TestCache_SetMulti(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	items := map[string][]byte{
		"multi1": []byte("value1"),
		"multi2": []byte("value2"),
		"multi3": []byte("value3"),
	}

	err := cache.SetMulti(ctx, items, 0)
	require.NoError(t, err)

	for k, v := range items {
		got, getErr := cache.Get(ctx, k)
		require.NoError(t, getErr)
		assert.Equal(t, v, got)
	}
}

func TestCache_DeleteMulti(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	items := map[string][]byte{
		"del1": []byte("value1"),
		"del2": []byte("value2"),
		"del3": []byte("value3"),
	}

	for k, v := range items {
		err := cache.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	err := cache.DeleteMulti(ctx, "del1", "del2")
	require.NoError(t, err)

	_, err = cache.Get(ctx, "del1")
	assert.Error(t, err)
	_, err = cache.Get(ctx, "del2")
	assert.Error(t, err)

	_, err = cache.Get(ctx, "del3")
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// Advanced Operations Tests
// ----------------------------------------------------------------------------

func TestCache_GetOrSet(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "getorset_test"
	var called atomic.Int32

	fn := func() ([]byte, error) {
		called.Add(1)
		return []byte("computed-value"), nil
	}

	val, err := cache.GetOrSet(ctx, key, fn, 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("computed-value"), val)
	assert.Equal(t, int32(1), called.Load())

	val, err = cache.GetOrSet(ctx, key, fn, 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("computed-value"), val)
	assert.Equal(t, int32(1), called.Load())
}

func TestCache_CompareAndSwap(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "cas_test"
	oldVal := []byte("old-value")
	newVal := []byte("new-value")

	err := cache.Set(ctx, key, oldVal, 0)
	require.NoError(t, err)

	success, err := cache.CompareAndSwap(ctx, key, oldVal, newVal, 0)
	require.NoError(t, err)
	assert.True(t, success)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, newVal, got)

	_, err = cache.L1().Get(ctx, key)
	assert.Error(t, err)

	success, err = cache.CompareAndSwap(ctx, key, oldVal, []byte("another"), 0)
	require.NoError(t, err)
	assert.False(t, success)

	success, err = cache.CompareAndSwap(ctx, "nonexistent", oldVal, newVal, 0)
	require.NoError(t, err)
	assert.False(t, success)
}

func TestCache_Increment(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "counter"

	val, err := cache.Increment(ctx, key, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = cache.Increment(ctx, key, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)

	_, err = cache.L1().Get(ctx, key)
	assert.Error(t, err)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("6"), got)
}

func TestCache_Decrement(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "counter"

	val, err := cache.Decrement(ctx, key, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), val)
}

func TestCache_SetNX(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "setnx_test"
	value := []byte("value")

	success, err := cache.SetNX(ctx, key, value, 0)
	require.NoError(t, err)
	assert.True(t, success)

	success, err = cache.SetNX(ctx, key, []byte("new-value"), 0)
	require.NoError(t, err)
	assert.False(t, success)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

func TestCache_GetSet(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "getset_test"
	oldVal := []byte("old")
	newVal := []byte("new")

	err := cache.Set(ctx, key, oldVal, 0)
	require.NoError(t, err)

	got, err := cache.GetSet(ctx, key, newVal, 0)
	require.NoError(t, err)
	assert.Equal(t, oldVal, got)

	current, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, newVal, current)
}

// ----------------------------------------------------------------------------
// Write-Back Mode Tests
// ----------------------------------------------------------------------------

func TestCache_WriteBack(t *testing.T) {
	testRedisAvailable(t)

	cache, err := layered.New(
		layered.WithL1MaxEntries(100),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2DB(10),
		layered.WithWriteBack(true),
	)
	require.NoError(t, err)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "writeback_test"
	value := []byte("async-value")

	err = cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	got, err := cache.L1().Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	time.Sleep(200 * time.Millisecond)

	got, err = cache.L2().Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

// ----------------------------------------------------------------------------
// Promotion Tests
// ----------------------------------------------------------------------------

func TestCache_PromoteOnHit(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "promote_test"
	value := []byte("value")

	err := cache.L2().Set(ctx, key, value, 0)
	require.NoError(t, err)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	got, err = cache.L1().Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	stats := cache.Stats()
	assert.Greater(t, stats.L2Promotions, int64(0))
}

func TestCache_NoPromoteOnHit(t *testing.T) {
	testRedisAvailable(t)

	cache, err := layered.New(
		layered.WithL1MaxEntries(100),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2DB(10),
		layered.WithPromoteOnHit(false),
	)
	require.NoError(t, err)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "no_promote_test"
	value := []byte("value")

	err = cache.L2().Set(ctx, key, value, 0)
	require.NoError(t, err)

	got, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)

	_, err = cache.L1().Get(ctx, key)
	assert.Error(t, err)

	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.L2Promotions)
}

// ----------------------------------------------------------------------------
// L1 Invalidation Tests
// ----------------------------------------------------------------------------

func TestCache_InvalidateL1(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "invalidate_test"
	value := []byte("value")

	err := cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	_, err = cache.L1().Get(ctx, key)
	require.NoError(t, err)

	err = cache.InvalidateL1(ctx, key)
	require.NoError(t, err)

	_, err = cache.L1().Get(ctx, key)
	assert.Error(t, err)

	got, err := cache.L2().Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

func TestCache_Refresh(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "refresh_test"
	value := []byte("value")

	err := cache.L2().Set(ctx, key, value, 0)
	require.NoError(t, err)

	err = cache.Refresh(ctx, key)
	require.NoError(t, err)

	got, err := cache.L1().Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

// ----------------------------------------------------------------------------
// Cache Management Tests
// ----------------------------------------------------------------------------

func TestCache_Keys(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	keys := []string{"user:1", "user:2", "product:1", "product:2"}

	for _, k := range keys {
		err := cache.Set(ctx, k, []byte("value"), 0)
		require.NoError(t, err)
	}

	userKeys, err := cache.Keys(ctx, "user:*")
	require.NoError(t, err)
	assert.Len(t, userKeys, 2)

	allKeys, err := cache.Keys(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, allKeys, 4)
}

func TestCache_Size(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	for i := 0; i < 50; i++ {
		key := "key_" + string(rune(i))
		err := cache.Set(ctx, key, []byte("value"), 0)
		require.NoError(t, err)
	}

	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, int64(50))
}

func TestCache_Clear(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	for i := 0; i < 100; i++ {
		err := cache.Set(ctx, "key"+string(rune(i)), []byte("value"), 0)
		require.NoError(t, err)
	}

	err := cache.Clear(ctx)
	require.NoError(t, err)

	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)
}

// ----------------------------------------------------------------------------
// Stats Tests
// ----------------------------------------------------------------------------

func TestCache_Stats(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	err := cache.Set(ctx, "stats1", []byte("value1"), 0)
	require.NoError(t, err)
	err = cache.Set(ctx, "stats2", []byte("value2"), 0)
	require.NoError(t, err)

	_, err = cache.Get(ctx, "stats1")
	require.NoError(t, err)
	_, err = cache.Get(ctx, "stats1")
	require.NoError(t, err)
	_, err = cache.Get(ctx, "nonexistent")
	assert.Error(t, err)

	stats := cache.Stats()

	assert.GreaterOrEqual(t, stats.Sets, int64(2))
	assert.GreaterOrEqual(t, stats.Gets, int64(3))
	assert.GreaterOrEqual(t, stats.Hits, int64(2))
	assert.GreaterOrEqual(t, stats.Misses, int64(1))
}

func TestCache_LayeredStats(t *testing.T) {
	cache := setupShortL1TTLTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "layered_stats"
	value := []byte("value")

	err := cache.Set(ctx, key, value, 0)
	require.NoError(t, err)

	_, err = cache.Get(ctx, key)
	require.NoError(t, err)

	time.Sleep(120 * time.Millisecond)

	_, err = cache.Get(ctx, key)
	require.NoError(t, err)

	stats := cache.Stats()
	assert.Greater(t, stats.L1Hits, int64(0))
	assert.Greater(t, stats.L2Hits, int64(0))
	assert.Greater(t, stats.L2Promotions, int64(0))
}

// ----------------------------------------------------------------------------
// Lifecycle Tests
// ----------------------------------------------------------------------------

func TestCache_Closed(t *testing.T) {
	cache := setupTestCache(t)

	assert.False(t, cache.Closed())

	err := cache.Close(context.Background())
	require.NoError(t, err)

	assert.True(t, cache.Closed())
}

func TestCache_OperationsAfterClose(t *testing.T) {
	cache := setupTestCache(t)

	ctx := context.Background()
	err := cache.Close(ctx)
	require.NoError(t, err)

	_, err = cache.Get(ctx, "key")
	assert.Error(t, err)
	assert.True(t, _errors.IsCacheClosed(err))

	err = cache.Set(ctx, "key", []byte("value"), 0)
	assert.Error(t, err)
	assert.True(t, _errors.IsCacheClosed(err))
}

func TestCache_L1_L2_Accessors(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	assert.NotNil(t, cache.L1())
	assert.NotNil(t, cache.L2())
}

// ----------------------------------------------------------------------------
// Context Tests
// ----------------------------------------------------------------------------

func TestCache_WithCancel(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cache.Get(ctx, "key")
	if err != nil {
		assert.True(t, _errors.IsCancelled(err) || _errors.IsNotFound(err))
	}
}

func TestCache_NoCache(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "nocache_test"

	err := cache.Set(ctx, key, []byte("value"), 0)
	require.NoError(t, err)

	noCacheCtx := cachectx.NoCache(ctx)
	_, err = cache.Get(noCacheCtx, key)
	assert.Error(t, err)
	assert.True(t, _errors.IsNotFound(err))
}

// ----------------------------------------------------------------------------
// Concurrency Tests
// ----------------------------------------------------------------------------

func TestCache_ConcurrentReadWrite(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	var wg sync.WaitGroup
	iterations := 100
	concurrency := 10
	var setCount, getCount atomic.Int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := "key_" + string(rune(id)) + "_" + string(rune(j))
				value := []byte("value_" + string(rune(j)))
				if err := cache.Set(ctx, key, value, 0); err == nil {
					setCount.Add(1)
				}
				if _, err := cache.Get(ctx, key); err == nil {
					getCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	assert.Greater(t, setCount.Load(), int64(0))
	assert.Greater(t, getCount.Load(), int64(0))
}

func TestCache_ConcurrentGetOrSet(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "concurrent_getorset"
	var called atomic.Int32

	fn := func() ([]byte, error) {
		called.Add(1)
		if called.Load() < 0 {
			return nil, _errors.NotFound("TestCache_ConcurrentGetOrSet", key)
		}
		time.Sleep(100 * time.Millisecond)
		return []byte("value"), nil
	}

	var wg sync.WaitGroup
	concurrent := 20

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := cache.GetOrSet(ctx, key, fn, 0)
			assert.NoError(t, err)
			assert.Equal(t, []byte("value"), val)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), called.Load())
}

// ----------------------------------------------------------------------------
// Edge Cases Tests
// ----------------------------------------------------------------------------

func TestCache_EmptyKey(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	err := cache.Set(ctx, "", []byte("value"), 0)
	assert.Error(t, err)

	_, err = cache.Get(ctx, "")
	assert.Error(t, err)
}

func TestCache_NilValue(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	err := cache.Set(ctx, "key", nil, 0)
	require.NoError(t, err)

	got, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCache_LargeValue(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	largeValue := make([]byte, 1024*1024)

	err := cache.Set(ctx, "large", largeValue, 0)
	require.NoError(t, err)

	got, err := cache.Get(ctx, "large")
	require.NoError(t, err)
	assert.Equal(t, largeValue, got)
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkCache_Set(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench:" + string(rune(i))
		_ = cache.Set(ctx, key, value, 0)
	}
}

func BenchmarkCache_Get_L1Hit(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "bench:get"
	value := []byte("value")

	_ = cache.Set(ctx, key, value, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, key)
	}
}

func BenchmarkCache_Get_L2Hit(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "bench:l2hit"
	value := []byte("value")

	_ = cache.L2().Set(ctx, key, value, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, key)
	}
}

func BenchmarkCache_Get_Miss(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "nonexistent")
	}
}

func BenchmarkCache_GetOrSet(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	fn := func() ([]byte, error) {
		return []byte("computed-value"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench:getorset:" + string(rune(i))
		_, _ = cache.GetOrSet(ctx, key, fn, 0)
	}
}

func BenchmarkCache_CompareAndSwap(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "bench:cas"
	oldVal := []byte("old")
	newVal := []byte("new")

	_ = cache.Set(ctx, key, oldVal, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.CompareAndSwap(ctx, key, oldVal, newVal, 0)
	}
}

func BenchmarkCache_Increment(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	key := "bench:counter"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Increment(ctx, key, 1)
	}
}

func BenchmarkCache_SetMulti(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items := map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
			"key3": []byte("value3"),
			"key4": []byte("value4"),
			"key5": []byte("value5"),
		}
		_ = cache.SetMulti(ctx, items, 0)
	}
}

func BenchmarkCache_GetMulti(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()
	items := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
		"key4": []byte("value4"),
		"key5": []byte("value5"),
	}
	_ = cache.SetMulti(ctx, items, 0)

	keys := []string{"key1", "key2", "key3", "key4", "key5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetMulti(ctx, keys...)
	}
}

func BenchmarkCache_Parallel(b *testing.B) {
	cache := setupTestCache(&testing.T{})
	defer func() { _ = cache.Close(context.Background()) }()

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "bench:parallel:" + string(rune(i))
			_ = cache.Set(ctx, key, []byte("value"), 0)
			_, _ = cache.Get(ctx, key)
			i++
		}
	})
}
