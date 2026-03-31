// Package redis_test provides tests for Redis-backed cache.
package redis_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/redis"
)

// testRedisAvailable checks if Redis is available for integration tests.
func testRedisAvailable(t *testing.T) *redis.Cache {
	t.Helper()
	cache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(10),
		redis.WithKeyPrefix("test:"),
	)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	return cache
}

// benchRedisAvailable checks if Redis is available for benchmarks.
func benchRedisAvailable(b *testing.B) *redis.Cache {
	b.Helper()
	cache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(10),
		redis.WithKeyPrefix("test:"),
	)
	if err != nil {
		b.Skip("Redis not available, skipping benchmark")
	}
	return cache
}

// testCache is a test helper that creates a cache and cleanup function.
func testCache(t *testing.T) (*redis.Cache, func()) {
	t.Helper()
	cache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(10),
		redis.WithKeyPrefix("test:"),
	)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	cleanup := func() {
		_ = cache.Clear(context.Background())
		_ = cache.Close(context.Background())
	}
	return cache, cleanup
}

func TestNewCache(t *testing.T) {
	cache := testRedisAvailable(t)
	defer cache.Close(context.Background())

	if cache == nil {
		t.Fatal("cache is nil")
	}
	if cache.Closed() {
		t.Error("cache should not be closed")
	}
}

func TestCache_GetSet2(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-get-set"
	value := []byte("test-value")

	// Get non-existent key
	_, err := cache.Get(ctx, key)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}

	// Set value
	err = cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get existing key
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("got %s, want %s", got, value)
	}
}

func TestCache_GetSet_WithTTL(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-ttl"
	value := []byte("value")
	ttl := 100 * time.Millisecond

	err := cache.Set(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("got %s, want %s", got, value)
	}

	// Wait for expiration
	time.Sleep(ttl + 50*time.Millisecond)

	// Should be gone
	_, err = cache.Get(ctx, key)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after TTL, got %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-delete"
	value := []byte("value")

	// Set value
	err := cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = cache.Get(ctx, key)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestCache_Exists(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-exists"

	exists, err := cache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("key should not exist")
	}

	err = cache.Set(ctx, key, []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	exists, err = cache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("key should exist")
	}
}

func TestCache_TTL(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-ttl-get"
	ttl := 5 * time.Second

	err := cache.Set(ctx, key, []byte("value"), ttl)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if got <= 0 || got > ttl {
		t.Errorf("TTL = %v, want ~%v", got, ttl)
	}
}

func TestCache_Expire(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-expire"

	err := cache.Set(ctx, key, []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Set expiration
	err = cache.Expire(ctx, key, 10*time.Second)
	if err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	ttl, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("TTL = %v, want >0", ttl)
	}
}

func TestCache_Persist(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-persist"

	err := cache.Set(ctx, key, []byte("value"), time.Second)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Remove expiration
	err = cache.Persist(ctx, key)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	ttl, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl != -1*time.Second {
		t.Errorf("TTL = %v, want -1 (no expiry)", ttl)
	}
}

func TestCache_GetMulti(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	items := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	for k, v := range items {
		err := cache.Set(ctx, k, v, 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	results, err := cache.GetMulti(ctx, "key1", "key2", "missing", "key3")
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
	if string(results["key1"]) != "value1" {
		t.Errorf("key1 = %s, want value1", results["key1"])
	}
	if string(results["key2"]) != "value2" {
		t.Errorf("key2 = %s, want value2", results["key2"])
	}
	if string(results["key3"]) != "value3" {
		t.Errorf("key3 = %s, want value3", results["key3"])
	}
}

func TestCache_SetMulti(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	items := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	err := cache.SetMulti(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	for k, v := range items {
		got, errGet := cache.Get(ctx, k)
		if errGet != nil {
			t.Errorf("Get %s failed: %v", k, errGet)
		}
		if !bytes.Equal(got, v) {
			t.Errorf("%s = %s, want %s", k, got, v)
		}
	}
}

func TestCache_DeleteMulti(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	keys := []string{"key1", "key2", "key3"}

	for _, k := range keys {
		err := cache.Set(ctx, k, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	err := cache.DeleteMulti(ctx, keys...)
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	for _, k := range keys {
		_, errGet := cache.Get(ctx, k)
		if !_errors.IsNotFound(errGet) {
			t.Errorf("key %s should be deleted", k)
		}
	}
}

func TestCache_SetNX(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-setnx"
	value := []byte("value")

	// Should succeed on first try
	ok, err := cache.SetNX(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if !ok {
		t.Error("SetNX should return true for new key")
	}

	// Should fail on second try
	ok, err = cache.SetNX(ctx, key, []byte("other"), 0)
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if ok {
		t.Error("SetNX should return false for existing key")
	}

	// Verify original value unchanged
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("value = %s, want %s", got, value)
	}
}

func TestCache_GetSet(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-getset"
	oldValue := []byte("old")
	newValue := []byte("new")

	// First call should return nil
	old, err := cache.GetSet(ctx, key, oldValue, 0)
	if err != nil {
		t.Fatalf("GetSet failed: %v", err)
	}
	if old != nil {
		t.Errorf("first GetSet returned %v, want nil", old)
	}

	// Verify value was set
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != "old" {
		t.Errorf("value = %s, want old", got)
	}

	// Second call should return old value
	old, err = cache.GetSet(ctx, key, newValue, 0)
	if err != nil {
		t.Fatalf("GetSet failed: %v", err)
	}
	if string(old) != "old" {
		t.Errorf("GetSet returned %s, want old", old)
	}

	// Verify new value
	got, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("value = %s, want new", got)
	}
}

func TestCache_Increment(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-incr"

	val, err := cache.Increment(ctx, key, 5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 5 {
		t.Errorf("Increment = %d, want 5", val)
	}

	val, err = cache.Increment(ctx, key, 3)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 8 {
		t.Errorf("Increment = %d, want 8", val)
	}
}

func TestCache_Decrement(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-decr"

	err := cache.Set(ctx, key, []byte("10"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := cache.Decrement(ctx, key, 3)
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}
	if val != 7 {
		t.Errorf("Decrement = %d, want 7", val)
	}
}

func TestCache_CompareAndSwap(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-cas"
	oldVal := []byte("old")
	newVal := []byte("new")

	// Set initial value
	err := cache.Set(ctx, key, oldVal, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Successful CAS
	ok, err := cache.CompareAndSwap(ctx, key, oldVal, newVal, 0)
	if err != nil {
		t.Fatalf("CAS failed: %v", err)
	}
	if !ok {
		t.Error("CAS should succeed")
	}

	// Verify value changed
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("after CAS, got %s, want new", got)
	}

	// Failed CAS with wrong old value
	ok, err = cache.CompareAndSwap(ctx, key, oldVal, []byte("other"), 0)
	if err != nil {
		t.Fatalf("CAS failed: %v", err)
	}
	if ok {
		t.Error("CAS should fail with wrong old value")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-getorset"
	expected := []byte("computed-value")
	computed := false

	fn := func() ([]byte, error) {
		computed = true
		return expected, nil
	}

	// First call should compute
	val, err := cache.GetOrSet(ctx, key, fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if !bytes.Equal(val, expected) {
		t.Errorf("got %s, want %s", val, expected)
	}
	if !computed {
		t.Error("fn was not called")
	}

	// Second call should use cached value
	computed = false
	val, err = cache.GetOrSet(ctx, key, fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if !bytes.Equal(val, expected) {
		t.Errorf("got %s, want %s", val, expected)
	}
	if computed {
		t.Error("fn was called again")
	}
}

func TestCache_HashOperations(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-hash"
	field := "field1"
	value := "value1"

	// HSet
	err := cache.HSet(ctx, key, field, value)
	if err != nil {
		t.Fatalf("HSet failed: %v", err)
	}

	// HGet
	got, err := cache.HGet(ctx, key, field)
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if string(got) != value {
		t.Errorf("HGet = %s, want %s", got, value)
	}

	// HGetAll
	all, err := cache.HGetAll(ctx, key)
	if err != nil {
		t.Fatalf("HGetAll failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("HGetAll returned %d fields, want 1", len(all))
	}

	// HDel
	err = cache.HDel(ctx, key, field)
	if err != nil {
		t.Fatalf("HDel failed: %v", err)
	}

	_, err = cache.HGet(ctx, key, field)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after HDel, got %v", err)
	}
}

func TestCache_ListOperations(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-list"
	values := []any{"value1", "value2", "value3"}

	// LPush
	err := cache.LPush(ctx, key, values...)
	if err != nil {
		t.Fatalf("LPush failed: %v", err)
	}

	// LRange
	got, err := cache.LRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("LRange failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("LRange returned %d items, want 3", len(got))
	}

	// LPop
	val, err := cache.LPop(ctx, key)
	if err != nil {
		t.Fatalf("LPop failed: %v", err)
	}
	if string(val) != "value3" {
		t.Errorf("LPop = %s, want value3", val)
	}

	// RPush
	err = cache.RPush(ctx, key, "value4")
	if err != nil {
		t.Fatalf("RPush failed: %v", err)
	}

	// RPop
	val, err = cache.RPop(ctx, key)
	if err != nil {
		t.Fatalf("RPop failed: %v", err)
	}
	if string(val) != "value4" {
		t.Errorf("RPop = %s, want value4", val)
	}
}

func TestCache_SetOperations(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-set"
	members := []any{"member1", "member2", "member3"}

	// SAdd
	err := cache.SAdd(ctx, key, members...)
	if err != nil {
		t.Fatalf("SAdd failed: %v", err)
	}

	// SMembers
	got, err := cache.SMembers(ctx, key)
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("SMembers returned %d members, want 3", len(got))
	}

	// SRem
	err = cache.SRem(ctx, key, "member1")
	if err != nil {
		t.Fatalf("SRem failed: %v", err)
	}

	got, err = cache.SMembers(ctx, key)
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("after SRem, got %d members, want 2", len(got))
	}
}

func TestCache_SortedSetOperations(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-zset"

	// ZAdd
	err := cache.ZAdd(ctx, key, 1.0, "member1")
	if err != nil {
		t.Fatalf("ZAdd failed: %v", err)
	}
	err = cache.ZAdd(ctx, key, 2.0, "member2")
	if err != nil {
		t.Fatalf("ZAdd failed: %v", err)
	}

	// ZRange
	got, err := cache.ZRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRange failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ZRange returned %d members, want 2", len(got))
	}

	// ZRem
	err = cache.ZRem(ctx, key, "member1")
	if err != nil {
		t.Fatalf("ZRem failed: %v", err)
	}

	got, err = cache.ZRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRange failed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("after ZRem, got %d members, want 1", len(got))
	}
}

func TestCache_Ping(t *testing.T) {
	cache := testRedisAvailable(t)
	defer cache.Close(context.Background())

	ctx := context.Background()
	err := cache.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestCache_Keys(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	keys := []string{"key1", "key2", "key3"}

	for _, k := range keys {
		err := cache.Set(ctx, k, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	got, err := cache.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Keys returned %d keys, want 3", len(got))
	}
}

func TestCache_Size(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()

	err := cache.Set(ctx, "key1", []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	err = cache.Set(ctx, "key2", []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size < 2 {
		t.Errorf("Size = %d, want >=2", size)
	}
}

func TestCache_Clear(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set multiple keys
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		err := cache.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 0 {
		t.Errorf("after Clear, size = %d, want 0", size)
	}
}

func TestCache_Stats(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()

	// Perform operations
	_, _ = cache.Get(ctx, "missing")
	_ = cache.Set(ctx, "key", []byte("value"), 0)
	_, _ = cache.Get(ctx, "key")

	snapshot := cache.Stats()
	if snapshot.Hits != 1 {
		t.Errorf("Hits = %d, want 1", snapshot.Hits)
	}
	if snapshot.Misses != 1 {
		t.Errorf("Misses = %d, want 1", snapshot.Misses)
	}
	if snapshot.Sets != 1 {
		t.Errorf("Sets = %d, want 1", snapshot.Sets)
	}
	if snapshot.Gets != 2 {
		t.Errorf("Gets = %d, want 2", snapshot.Gets)
	}
}

func TestCache_Closed(t *testing.T) {
	cache := testRedisAvailable(t)

	if cache.Closed() {
		t.Error("cache should not be closed initially")
	}

	err := cache.Close(context.Background())
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !cache.Closed() {
		t.Error("cache should be closed")
	}

	// Operations should fail after close
	_, err = cache.Get(context.Background(), "key")
	if !_errors.IsCacheClosed(err) {
		t.Errorf("expected Closed error, got %v", err)
	}
}

func TestCache_ConcurrentOperations(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	const numGoroutines = 50
	const opsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a' + (id % 26)))
				value := []byte("value")
				_ = cache.Set(ctx, key, value, 0)
				_, _ = cache.Get(ctx, key)
				_ = cache.Delete(ctx, key)
			}
			done <- true
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestCache_KeyPrefix(t *testing.T) {
	cache, cleanup := testCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "prefix-test"
	value := []byte("value")

	err := cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be able to get with same key
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("got %s, want %s", got, value)
	}

	// Keys should not include prefix in returned values
	keys, err := cache.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	for _, k := range keys {
		if k != "" && k[0] == 't' && k[1] == 'e' && k[2] == 's' && k[3] == 't' && k[4] == ':' {
			t.Errorf("key %s still has prefix", k)
		}
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache := benchRedisAvailable(b)
	defer cache.Close(context.Background())

	ctx := context.Background()
	key := "bench-key"
	_ = cache.Set(ctx, key, []byte("value"), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, key)
	}
}

func BenchmarkCache_Set(b *testing.B) {
	cache := benchRedisAvailable(b)
	defer cache.Close(context.Background())

	ctx := context.Background()
	key := "bench-key"
	value := []byte("value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, key, value, 0)
	}
}

func BenchmarkCache_GetMulti(b *testing.B) {
	cache := benchRedisAvailable(b)
	defer cache.Close(context.Background())

	ctx := context.Background()
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, k := range keys {
		_ = cache.Set(ctx, k, []byte("value"), 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetMulti(ctx, keys...)
	}
}

func BenchmarkCache_GetOrSet(b *testing.B) {
	cache := benchRedisAvailable(b)
	defer cache.Close(context.Background())

	ctx := context.Background()
	key := "bench-getorset"
	fn := func() ([]byte, error) {
		return []byte("value"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetOrSet(ctx, key, fn, 0)
	}
}

func BenchmarkCache_Increment(b *testing.B) {
	cache := benchRedisAvailable(b)
	defer cache.Close(context.Background())

	ctx := context.Background()
	key := "bench-incr"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Increment(ctx, key, 1)
	}
}
