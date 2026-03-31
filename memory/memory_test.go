// Package memory_test provides tests for high-performance, sharded in-process cache.
package memory_test

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/memory"
)

// testCache is a test helper that creates a cache with cleanup.
func testCache(t *testing.T, opts ...memory.Option) (*memory.Cache, func()) {
	t.Helper()
	cache, err := memory.New(opts...)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	cleanup := func() {
		_ = cache.Clear(context.Background())
		_ = cache.Close(context.Background())
	}
	return cache, cleanup
}

// benchTestCache checks if Redis is available for benchmarks.
func benchTestCache(b *testing.B) (*memory.Cache, func()) {
	b.Helper()
	cache, err := memory.New()
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	cleanup := func() {
		_ = cache.Clear(context.Background())
		_ = cache.Close(context.Background())
	}
	return cache, cleanup
}

// defaultCache creates a cache with reasonable defaults for testing.
func defaultCache(t *testing.T) (*memory.Cache, func()) {
	return testCache(t,
		memory.WithMaxEntries(1000),
		memory.WithMaxMemoryMB(100),
		memory.WithCleanupInterval(100*time.Millisecond),
		memory.WithTTL(time.Second),
	)
}

func TestNewCache(t *testing.T) {
	tests := []struct {
		name    string
		opts    []memory.Option
		wantErr bool
	}{
		{
			name:    "default config",
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "with max entries",
			opts:    []memory.Option{memory.WithMaxEntries(100)},
			wantErr: false,
		},
		{
			name:    "with max memory",
			opts:    []memory.Option{memory.WithMaxMemoryMB(50)},
			wantErr: false,
		},
		{
			name:    "with TTL",
			opts:    []memory.Option{memory.WithTTL(time.Minute)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := memory.New(tt.opts...)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cache == nil {
				t.Error("cache is nil")
			}
			_ = cache.Close(context.Background())
		})
	}
}

func TestCache_GetSet(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-key"
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
	cache, cleanup := testCache(t,
		memory.WithTTL(100*time.Millisecond),
		memory.WithCleanupInterval(10*time.Millisecond),
	)
	defer cleanup()

	ctx := context.Background()
	key := "test-ttl"
	value := []byte("value")
	ttl := 50 * time.Millisecond

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
	time.Sleep(ttl + 100*time.Millisecond)

	// Should be gone
	_, err = cache.Get(ctx, key)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after TTL, got %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := testCache(t, memory.WithTTL(time.Hour))
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
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-expire"
	value := []byte("value")

	// Set key with no TTL (persistent)
	err := cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Check TTL - should be no expiration
	ttl, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl != 0 {
		t.Errorf("initial TTL = %v, want 0 (no expiry)", ttl)
	}

	// Set expiration
	newTTL := 10 * time.Second
	err = cache.Expire(ctx, key, newTTL)
	if err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	// Verify TTL changed
	ttl, err = cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 || ttl > newTTL {
		t.Errorf("TTL after Expire = %v, want ~%v", ttl, newTTL)
	}

	// Verify value still exists
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("value = %s, want %s", got, value)
	}
}

func TestCache_Expire_OnNonExistentKey(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "nonexistent-key"

	err := cache.Expire(ctx, key, time.Second)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCache_Expire_OnExpiredKey(t *testing.T) {
	cache, cleanup := testCache(t,
		memory.WithTTL(10*time.Millisecond),
		memory.WithCleanupInterval(5*time.Millisecond),
	)
	defer cleanup()

	ctx := context.Background()
	key := "expired-key"

	// Set key with short TTL
	err := cache.Set(ctx, key, []byte("value"), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Try to set expiration on expired key
	err = cache.Expire(ctx, key, time.Second)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCache_Expire_NegativeTTL(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-key"

	// Set key
	err := cache.Set(ctx, key, []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Try to set negative TTL
	err = cache.Expire(ctx, key, -1*time.Second)
	if err == nil {
		t.Error("expected error for negative TTL, got nil")
	}
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	// Verify value still exists
	_, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
}

func TestCache_Expire_UpdateExistingTTL(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-update-ttl"

	// Set key with initial TTL
	initialTTL := 5 * time.Second
	err := cache.Set(ctx, key, []byte("value"), initialTTL)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Check initial TTL
	ttl1, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl1 <= 0 || ttl1 > initialTTL {
		t.Errorf("initial TTL = %v, want ~%v", ttl1, initialTTL)
	}

	// Update TTL to a different value
	newTTL := 10 * time.Second
	err = cache.Expire(ctx, key, newTTL)
	if err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	// Verify TTL updated
	ttl2, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl2 <= 0 || ttl2 > newTTL {
		t.Errorf("updated TTL = %v, want ~%v", ttl2, newTTL)
	}
}

func TestCache_Persist(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-persist"
	value := []byte("value")

	// Set key with TTL
	ttl := 10 * time.Second
	err := cache.Set(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify TTL is set
	ttl1, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl1 <= 0 {
		t.Errorf("initial TTL = %v, want >0", ttl1)
	}

	// Remove expiration
	err = cache.Persist(ctx, key)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Verify TTL is now 0 (no expiration)
	ttl2, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl2 != 0 {
		t.Errorf("TTL after Persist = %v, want 0", ttl2)
	}

	// Verify value still exists
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("value = %s, want %s", got, value)
	}
}

func TestCache_Persist_OnNonExistentKey(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "nonexistent-key"

	err := cache.Persist(ctx, key)
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCache_Persist_OnPersistentKey(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "persistent-key"
	value := []byte("value")

	// Set key with no TTL
	err := cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify TTL is 0
	ttl, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl != 0 {
		t.Errorf("initial TTL = %v, want 0", ttl)
	}

	// Persist should succeed (no-op)
	err = cache.Persist(ctx, key)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Verify TTL still 0
	ttl2, err := cache.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl2 != 0 {
		t.Errorf("TTL after Persist = %v, want 0", ttl2)
	}
}

func TestCache_GetMulti(t *testing.T) {
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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

func TestCache_GetSet2(t *testing.T) {
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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

func TestCache_GetOrSet_Concurrent(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "concurrent-key"
	var executions int64
	var mu sync.Mutex

	// Create a function that takes time to execute
	fn := func() ([]byte, error) {
		mu.Lock()
		executions++
		count := executions
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		if count%2 == 0 {
			return nil, errors.New("intentional error")
		}
		return []byte("result"), nil
	}

	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := cache.GetOrSet(ctx, key, fn, 0)
			if err != nil {
				t.Errorf("GetOrSet failed: %v", err)
			}
			if string(val) != "result" {
				t.Errorf("got %s, want result", val)
			}
		}()
	}

	wg.Wait()

	if executions != 1 {
		t.Errorf("fn executed %d times, want 1", executions)
	}
}

func TestCache_Keys(t *testing.T) {
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
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
	if size != 2 {
		t.Errorf("Size = %d, want 2", size)
	}
}

func TestCache_Clear(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set multiple keys
	for i := 0; i < 10; i++ {
		key := "key" + strconv.Itoa(i)
		err := cache.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 10 {
		t.Errorf("before Clear, size = %d, want 10", size)
	}

	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	size, err = cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 0 {
		t.Errorf("after Clear, size = %d, want 0", size)
	}
}

func TestCache_Stats(t *testing.T) {
	cache, cleanup := defaultCache(t)
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
	cache, cleanup := defaultCache(t)
	defer cleanup()

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
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	const numGoroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "key" + strconv.Itoa(id%10)
				value := []byte("value" + strconv.Itoa(j))
				_ = cache.Set(ctx, key, value, 0)
				_, _ = cache.Get(ctx, key)
				_ = cache.Delete(ctx, key)
			}
		}(i)
	}

	wg.Wait()
}

func TestCache_Sharding(t *testing.T) {
	// Test that keys are distributed across shards
	cache, cleanup := testCache(t,
		memory.WithMaxEntries(1000),
		memory.WithShards(16),
	)
	defer cleanup()

	ctx := context.Background()
	const numKeys = 1000

	for i := 0; i < numKeys; i++ {
		key := "key" + strconv.Itoa(i)
		err := cache.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != numKeys {
		t.Errorf("size = %d, want %d", size, numKeys)
	}
}

func TestCache_CapacityEviction(t *testing.T) {
	var evictedKey string
	var evictedReason string

	cache, cleanup := testCache(t,
		memory.WithMaxEntries(5),
		memory.WithOnEvictionPolicy(func(key, reason string) {
			evictedKey = key
			evictedReason = reason
		}),
	)
	defer cleanup()

	ctx := context.Background()

	// Add 6 entries to trigger eviction
	for i := 0; i < 6; i++ {
		key := "key" + strconv.Itoa(i)
		err := cache.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	size, err := cache.Size(ctx)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}

	// Verify eviction callback was called
	if evictedKey == "" {
		t.Error("eviction callback not called")
	}
	if evictedReason != "capacity" {
		t.Errorf("eviction reason = %s, want capacity", evictedReason)
	}
}

func TestCache_ExpirationEviction(t *testing.T) {
	var evictedKey string
	var evictedReason string

	cache, cleanup := testCache(t,
		memory.WithTTL(50*time.Millisecond),
		memory.WithCleanupInterval(20*time.Millisecond),
		memory.WithOnEvictionPolicy(func(key, reason string) {
			evictedKey = key
			evictedReason = reason
		}),
	)
	defer cleanup()

	ctx := context.Background()

	// Add entry with short TTL
	err := cache.Set(ctx, "expiring-key", []byte("value"), 30*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Verify entry is gone
	_, err = cache.Get(ctx, "expiring-key")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Verify eviction callback was called
	if evictedKey == "" {
		t.Error("eviction callback not called")
	}
	if evictedReason != "expired" {
		t.Errorf("eviction reason = %s, want expired", evictedReason)
	}
}

func TestCache_NoCacheBypass(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-key"
	value := []byte("value")

	err := cache.Set(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Normal get should succeed
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("got %s, want %s", got, value)
	}
}

func TestCache_UpdateExisting(t *testing.T) {
	cache, cleanup := defaultCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "update-key"
	oldValue := []byte("old")
	newValue := []byte("new")

	err := cache.Set(ctx, key, oldValue, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	err = cache.Set(ctx, key, newValue, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, newValue) {
		t.Errorf("got %s, want %s", got, newValue)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-key"
	_ = cache.Set(ctx, key, []byte("value"), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, key)
	}
}

func BenchmarkCache_Set(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-key"
	value := []byte("value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, key, value, 0)
	}
}

func BenchmarkCache_GetOrSet(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

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

func BenchmarkCache_GetMulti(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

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

func BenchmarkCache_Increment(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-incr"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Increment(ctx, key, 1)
	}
}

func BenchmarkCache_CompareAndSwap(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-cas"
	oldVal := []byte("old")
	newVal := []byte("new")
	_ = cache.Set(ctx, key, oldVal, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.CompareAndSwap(ctx, key, oldVal, newVal, 0)
	}
}

func BenchmarkCache_Expire(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-expire"
	_ = cache.Set(ctx, key, []byte("value"), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Expire(ctx, key, time.Second)
	}
}

func BenchmarkCache_Persist(b *testing.B) {
	cache, cleanup := benchTestCache(b)
	defer cleanup()

	ctx := context.Background()
	key := "bench-persist"
	_ = cache.Set(ctx, key, []byte("value"), time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Persist(ctx, key)
	}
}
