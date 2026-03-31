package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/redis"
)

// testRedisAvailable checks if Redis is available for integration tests.
func testRedisAvailableForTyped(t *testing.T) {
	redisCache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
	)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	_ = redisCache.Close(context.Background())
}

func setupTypedRedisCache(t *testing.T) *TypedCache[User] {
	testRedisAvailableForTyped(t)

	redisCache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
		redis.WithKeyPrefix("typed_test:"),
	)
	require.NoError(t, err)

	tc := NewJSONTypedCache[User](redisCache)

	ctx := context.Background()
	_ = tc.Clear(ctx)

	return tc
}

func setupTypedRedisInt64Cache(t *testing.T) *TypedInt64Cache {
	testRedisAvailableForTyped(t)

	redisCache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
		redis.WithKeyPrefix("typed_int64_test:"),
	)
	require.NoError(t, err)

	tc := NewTypedInt64Cache(redisCache)

	ctx := context.Background()
	_ = tc.Clear(ctx)

	return tc
}

func setupTypedRedisStringCache(t *testing.T) *TypedStringCache {
	testRedisAvailableForTyped(t)

	redisCache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
		redis.WithKeyPrefix("typed_string_test:"),
	)
	require.NoError(t, err)

	tc := NewTypedStringCache(redisCache)

	ctx := context.Background()
	_ = tc.Clear(ctx)

	return tc
}

func setupTypedRedisBytesCache(t *testing.T) *TypedBytesCache {
	testRedisAvailableForTyped(t)

	redisCache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
		redis.WithKeyPrefix("typed_bytes_test:"),
	)
	require.NoError(t, err)

	tc := NewTypedBytesCache(redisCache)

	ctx := context.Background()
	_ = tc.Clear(ctx)

	return tc
}

// ----------------------------------------------------------------------------
// Constructor Tests
// ----------------------------------------------------------------------------

// func TestNewRedisTypedCache(t *testing.T) {
// 	testRedisAvailableForTyped(t)

// 	tc, err := NewRedisTypedCache[User](nil)
// 	require.NoError(t, err)
// 	assert.NotNil(t, tc)
// 	defer tc.Close(context.Background())

// 	assert.Equal(t, CacheRedis, tc.CacheType())
// 	assert.NotEmpty(t, tc.Name())
// }

func TestNewRedisTypedCache_WithOptions(t *testing.T) {
	testRedisAvailableForTyped(t)

	opts := []redis.Option{
		redis.WithAddress("localhost:6379"),
		redis.WithDB(9),
		redis.WithKeyPrefix("custom:"),
		redis.WithPoolSize(20),
	}

	tc, err := NewRedisTypedCache[User](opts)
	require.NoError(t, err)
	assert.NotNil(t, tc)
	_ = tc.Close(context.Background())
}

// ----------------------------------------------------------------------------
// Basic Operations Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_Get_Set(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, user, got)
}

func TestTypedRedisCache_Get_NotFound(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	_, err := tc.Get(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestTypedRedisCache_Get_WithTTL(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "user:1", user, 1*time.Second)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, user, got)

	time.Sleep(1100 * time.Millisecond)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
}

func TestTypedRedisCache_Delete(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	err = tc.Delete(ctx, "user:1")
	require.NoError(t, err)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
}

func TestTypedRedisCache_Exists(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	exists, err := tc.Exists(ctx, "user:1")
	require.NoError(t, err)
	assert.False(t, exists)

	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	exists, err = tc.Exists(ctx, "user:1")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTypedRedisCache_TTL(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "user:1", user, 30*time.Second)
	require.NoError(t, err)

	ttl, err := tc.TTL(ctx, "user:1")
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 30*time.Second)
}

// ----------------------------------------------------------------------------
// Multi Operations Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_GetMulti(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
		"user:3": {ID: 3, Name: "Charlie", Age: 35},
	}

	for k, v := range users {
		err := tc.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	results, err := tc.GetMulti(ctx, "user:1", "user:2", "user:3", "user:4")
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, users["user:1"], results["user:1"])
	assert.Equal(t, users["user:2"], results["user:2"])
	assert.Equal(t, users["user:3"], results["user:3"])
}

func TestTypedRedisCache_GetMulti_Empty(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	results, err := tc.GetMulti(ctx)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestTypedRedisCache_SetMulti(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
		"user:3": {ID: 3, Name: "Charlie", Age: 35},
	}

	err := tc.SetMulti(ctx, users, 0)
	require.NoError(t, err)

	for k, v := range users {
		got, getErr := tc.Get(ctx, k)
		require.NoError(t, getErr)
		assert.Equal(t, v, got)
	}
}

func TestTypedRedisCache_DeleteMulti(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
		"user:3": {ID: 3, Name: "Charlie", Age: 35},
	}

	for k, v := range users {
		err := tc.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	err := tc.DeleteMulti(ctx, "user:1", "user:2")
	require.NoError(t, err)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
	_, err = tc.Get(ctx, "user:2")
	assert.Error(t, err)

	_, err = tc.Get(ctx, "user:3")
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// Advanced Operations Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_GetOrSet(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	called := 0

	fn := func() (User, error) {
		called++
		return User{ID: 1, Name: "Alice", Age: 30}, nil
	}

	// First call - should compute
	user, err := tc.GetOrSet(ctx, "user:1", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "Alice", user.Name)

	// Second call - should use cache
	user, err = tc.GetOrSet(ctx, "user:1", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "Alice", user.Name)
}

func TestTypedRedisCache_GetOrSet_WithError(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	called := 0

	fn := func() (User, error) {
		called++
		return User{}, assert.AnError
	}

	_, err := tc.GetOrSet(ctx, "user:1", fn, 0)
	assert.Error(t, err)
	assert.Equal(t, 1, called)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
}

func TestTypedRedisCache_CompareAndSwap(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}

	err := tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	success, err := tc.CompareAndSwap(ctx, "user:1", oldUser, newUser, 0)
	require.NoError(t, err)
	assert.True(t, success)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, newUser, got)
}

func TestTypedRedisCache_CompareAndSwap_Failure(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}
	wrongUser := User{ID: 1, Name: "Wrong", Age: 99}

	err := tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	success, err := tc.CompareAndSwap(ctx, "user:1", wrongUser, newUser, 0)
	require.NoError(t, err)
	assert.False(t, success)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, oldUser, got)
}

func TestTypedRedisCache_SetNX(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	// First set - should succeed
	success, err := tc.SetNX(ctx, "user:1", user, 0)
	require.NoError(t, err)
	assert.True(t, success)

	// Second set - should fail
	success, err = tc.SetNX(ctx, "user:1", user, 0)
	require.NoError(t, err)
	assert.False(t, success)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, user, got)
}

func TestTypedRedisCache_GetSet(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}

	err := tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	got, err := tc.GetSet(ctx, "user:1", newUser, 0)
	require.NoError(t, err)
	assert.Equal(t, oldUser, got)

	current, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, newUser, current)
}

// ----------------------------------------------------------------------------
// TypedInt64Cache Tests
// ----------------------------------------------------------------------------

func TestTypedRedisInt64Cache_Increment(t *testing.T) {
	tc := setupTypedRedisInt64Cache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	val, err := tc.Increment(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	val, err = tc.Increment(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)

	val, err = tc.Increment(ctx, "counter", -2)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

func TestTypedRedisInt64Cache_Decrement(t *testing.T) {
	tc := setupTypedRedisInt64Cache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	val, err := tc.Decrement(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(-5), val)

	val, err = tc.Decrement(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(-8), val)
}

func TestTypedRedisInt64Cache_Get_Set(t *testing.T) {
	tc := setupTypedRedisInt64Cache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	err := tc.Set(ctx, "value", int64(42), 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "value")
	require.NoError(t, err)
	assert.Equal(t, int64(42), val)
}

// ----------------------------------------------------------------------------
// TypedStringCache Tests
// ----------------------------------------------------------------------------

func TestTypedRedisStringCache_Get_Set(t *testing.T) {
	tc := setupTypedRedisStringCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	err := tc.Set(ctx, "greeting", "Hello, World!", 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "greeting")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", val)
}

func TestTypedRedisStringCache_GetOrSet(t *testing.T) {
	tc := setupTypedRedisStringCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	called := 0

	fn := func() (string, error) {
		called++
		return "computed-value", nil
	}

	val, err := tc.GetOrSet(ctx, "key", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, "computed-value", val)
	assert.Equal(t, 1, called)

	val, err = tc.GetOrSet(ctx, "key", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, "computed-value", val)
	assert.Equal(t, 1, called)
}

// ----------------------------------------------------------------------------
// TypedBytesCache Tests
// ----------------------------------------------------------------------------

func TestTypedRedisBytesCache_Get_Set(t *testing.T) {
	tc := setupTypedRedisBytesCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	data := []byte{0x01, 0x02, 0x03, 0x04}

	err := tc.Set(ctx, "data", data, 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "data")
	require.NoError(t, err)
	assert.Equal(t, data, val)
}

func TestTypedRedisBytesCache_LargeValue(t *testing.T) {
	tc := setupTypedRedisBytesCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	largeData := make([]byte, 1024*1024) // 1MB

	err := tc.Set(ctx, "large", largeData, 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "large")
	require.NoError(t, err)
	assert.Equal(t, largeData, val)
}

// ----------------------------------------------------------------------------
// Utility Operations Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_Keys(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	users := map[string]User{
		"user:1":    {ID: 1, Name: "Alice", Age: 30},
		"user:2":    {ID: 2, Name: "Bob", Age: 25},
		"product:1": {ID: 1, Name: "Laptop", Age: 0},
	}

	for k, v := range users {
		err := tc.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	keys, err := tc.Keys(ctx, "user:*")
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	allKeys, err := tc.Keys(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, allKeys, 3)
}

func TestTypedRedisCache_Clear(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	err = tc.Clear(ctx)
	require.NoError(t, err)

	keys, err := tc.Keys(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}

func TestTypedRedisCache_Size(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	size, err := tc.Size(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, int64(0))

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	size, err = tc.Size(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, int64(1))
}

func TestTypedRedisCache_Stats(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	_ = tc.Set(ctx, "user:1", user, 0)
	_, _ = tc.Get(ctx, "user:1")

	stats := tc.Stats()
	assert.NotNil(t, stats)
}

// ----------------------------------------------------------------------------
// Lifecycle Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_Closed(t *testing.T) {
	tc := setupTypedRedisCache(t)

	assert.False(t, tc.Closed())

	err := tc.Close(context.Background())
	require.NoError(t, err)
	assert.True(t, tc.Closed())
}

func TestTypedRedisCache_Close(t *testing.T) {
	tc := setupTypedRedisCache(t)

	ctx := context.Background()
	err := tc.Close(ctx)
	require.NoError(t, err)

	_, err = tc.Get(ctx, "key")
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// Concurrency Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_ConcurrentAccess(t *testing.T) {
	tc := setupTypedRedisInt64Cache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	var wg sync.WaitGroup
	concurrency := 20
	iterations := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = tc.Increment(ctx, "counter", 1)
			}
		}()
	}

	wg.Wait()

	val, err := tc.Get(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(concurrency*iterations), val)
}

func TestTypedRedisCache_ConcurrentGetOrSet(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	called := 0
	var mu sync.Mutex

	fn := func() (User, error) {
		mu.Lock()
		called++
		mu.Unlock()

		// Adding a dummy condition prevents the "always nil" warning
		if called < 0 {
			return User{}, _errors.NotFound(
				"TestTypedRedisCache_ConcurrentGetOrSet",
				"User not found",
			)
		}

		time.Sleep(50 * time.Millisecond)
		return User{ID: 1, Name: "Alice", Age: 30}, nil
	}

	var wg sync.WaitGroup
	concurrency := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = tc.GetOrSet(ctx, "user:1", fn, 0)
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, called)
}

// ----------------------------------------------------------------------------
// Edge Cases Tests
// ----------------------------------------------------------------------------

func TestTypedRedisCache_EmptyKey(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	err := tc.Set(ctx, "", user, 0)
	assert.Error(t, err)

	_, err = tc.Get(ctx, "")
	assert.Error(t, err)
}

func TestTypedRedisCache_NilValue(t *testing.T) {
	tc := setupTypedRedisCache(t)
	defer tc.Close(context.Background())

	ctx := context.Background()

	err := tc.Set(ctx, "key", User{}, 0)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, User{}, got)
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkTypedRedisCache_Set(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench:" + string(rune(i))
		_ = tc.Set(ctx, key, user, 0)
	}
}

func BenchmarkTypedRedisCache_Get(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}
	_ = tc.Set(ctx, "bench:get", user, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Get(ctx, "bench:get")
	}
}

func BenchmarkTypedRedisCache_Get_Miss(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Get(ctx, "nonexistent")
	}
}

func BenchmarkTypedRedisCache_GetOrSet(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()
	fn := func() (User, error) {
		return User{ID: 1, Name: "Alice", Age: 30}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench:" + string(rune(i))
		_, _ = tc.GetOrSet(ctx, key, fn, 0)
	}
}

func BenchmarkTypedRedisInt64Cache_Increment(b *testing.B) {
	tc := setupTypedRedisInt64Cache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Increment(ctx, "counter", 1)
	}
}

func BenchmarkTypedRedisCache_SetMulti(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()
	users := make(map[string]User)
	for i := 0; i < 10; i++ {
		users["user:"+string(rune(i))] = User{ID: i, Name: "User", Age: 30}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tc.SetMulti(ctx, users, 0)
	}
}

func BenchmarkTypedRedisCache_GetMulti(b *testing.B) {
	tc := setupTypedRedisCache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		key := "user:" + string(rune(i))
		keys[i] = key
		_ = tc.Set(ctx, key, User{ID: i, Name: "User", Age: 30}, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.GetMulti(ctx, keys...)
	}
}

func BenchmarkTypedRedisCache_Parallel(b *testing.B) {
	tc := setupTypedRedisInt64Cache(&testing.T{})
	defer tc.Close(context.Background())

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = tc.Increment(ctx, "counter", 1)
		}
	})
}
