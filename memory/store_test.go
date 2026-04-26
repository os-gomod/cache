package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestStore_New(t *testing.T) {
	store, err := New(
		WithMaxEntries(1000),
		WithMaxMemoryBytes(1024*1024),
		WithShardCount(16),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if store.Name() != "memory" {
		t.Errorf("Name() = %q, want %q", store.Name(), "memory")
	}
	if store.Closed() {
		t.Error("newly created store should not be closed")
	}

	// Cleanup
	store.Close(context.Background())
}

func TestStore_Ping(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping() on open store error = %v", err)
	}

	// Close and verify ping fails
	store.Close(context.Background())
	if err := store.Ping(context.Background()); err == nil {
		t.Error("Ping() on closed store should return error")
	}
}

func TestStore_BasicGetSet(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// Set and Get
	if err := store.Set(ctx, "key1", []byte("value1"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	val, err := store.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("Get() = %q, want %q", string(val), "value1")
	}

	// Get non-existent key
	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get() of nonexistent key should return error")
	}
}

func TestStore_Delete(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value1"), time.Minute)
	store.Delete(ctx, "key1")

	_, err = store.Get(ctx, "key1")
	if err == nil {
		t.Error("Get() after delete should return error")
	}

	// Delete non-existent key should not error
	if err := store.Delete(ctx, "nonexistent"); err != nil {
		t.Errorf("Delete() of nonexistent key error = %v", err)
	}
}

func TestStore_Exists(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	exists, err := store.Exists(ctx, "key1")
	if err != nil || exists {
		t.Error("Exists() of nonexistent key should return false, nil")
	}

	store.Set(ctx, "key1", []byte("value1"), time.Minute)
	exists, err = store.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Error("Exists() of existing key should return true, nil")
	}
}

func TestStore_TTL(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value1"), 10*time.Second)
	ttl, err := store.TTL(ctx, "key1")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > 10*time.Second {
		t.Errorf("TTL() = %v, want (0, 10s]", ttl)
	}

	// TTL of nonexistent key
	_, err = store.TTL(ctx, "nonexistent")
	if err == nil {
		t.Error("TTL() of nonexistent key should return error")
	}
}

func TestStore_Expiration(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// Set with very short TTL
	store.Set(ctx, "key1", []byte("value1"), 50*time.Millisecond)
	store.Set(ctx, "key2", []byte("value2"), 5*time.Minute)

	// Verify both exist
	val, err := store.Get(ctx, "key1")
	if err != nil || string(val) != "value1" {
		t.Error("key1 should exist initially")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// key1 should be expired
	_, err = store.Get(ctx, "key1")
	if err == nil {
		t.Error("Get() of expired key should return error")
	}

	// key2 should still exist
	val, err = store.Get(ctx, "key2")
	if err != nil || string(val) != "value2" {
		t.Error("key2 should still exist after key1 expired")
	}
}

func TestStore_GetMulti(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)

	result, err := store.GetMulti(ctx, "k1", "k2", "k3")
	if err != nil {
		t.Fatalf("GetMulti() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("GetMulti() returned %d keys, want 2", len(result))
	}
	if string(result["k1"]) != "v1" || string(result["k2"]) != "v2" {
		t.Error("GetMulti() returned wrong values")
	}
}

func TestStore_SetMulti(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	items := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
	}
	if err := store.SetMulti(ctx, items, time.Minute); err != nil {
		t.Fatalf("SetMulti() error = %v", err)
	}

	val, err := store.Get(ctx, "k1")
	if err != nil || string(val) != "v1" {
		t.Error("SetMulti() should have stored k1")
	}
}

func TestStore_DeleteMulti(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)
	store.Set(ctx, "k3", []byte("v3"), time.Minute)

	if err := store.DeleteMulti(ctx, "k1", "k2"); err != nil {
		t.Fatalf("DeleteMulti() error = %v", err)
	}

	_, err = store.Get(ctx, "k1")
	if err == nil {
		t.Error("k1 should be deleted")
	}
	_, err = store.Get(ctx, "k2")
	if err == nil {
		t.Error("k2 should be deleted")
	}
	val, err := store.Get(ctx, "k3")
	if err != nil || string(val) != "v3" {
		t.Error("k3 should still exist")
	}
}

func TestStore_SetNX(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// SetNX on new key should succeed
	set, err := store.SetNX(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil || !set {
		t.Error("SetNX() on new key should return true, nil")
	}

	// SetNX on existing key should return false
	set, err = store.SetNX(ctx, "key1", []byte("value2"), time.Minute)
	if err != nil || set {
		t.Error("SetNX() on existing key should return false, nil")
	}

	// Verify value is still value1
	val, err := store.Get(ctx, "key1")
	if err != nil || string(val) != "value1" {
		t.Error("SetNX() should not have overwritten existing value")
	}
}

func TestStore_Increment(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// Increment non-existent key
	val, err := store.Increment(ctx, "counter", 5)
	if err != nil || val != 5 {
		t.Errorf("Increment() new key = %d, want 5", val)
	}

	// Increment existing key
	val, err = store.Increment(ctx, "counter", 3)
	if err != nil || val != 8 {
		t.Errorf("Increment() existing key = %d, want 8", val)
	}

	// Decrement
	val, err = store.Decrement(ctx, "counter", 2)
	if err != nil || val != 6 {
		t.Errorf("Decrement() = %d, want 6", val)
	}
}

func TestStore_GetSet(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("old_value"), time.Minute)

	oldVal, err := store.GetSet(ctx, "key1", []byte("new_value"), time.Minute)
	if err != nil {
		t.Fatalf("GetSet() error = %v", err)
	}
	if string(oldVal) != "old_value" {
		t.Errorf("GetSet() old = %q, want %q", string(oldVal), "old_value")
	}

	val, err := store.Get(ctx, "key1")
	if err != nil || string(val) != "new_value" {
		t.Error("GetSet() should have updated the value")
	}

	// GetSet on non-existent key
	oldVal, err = store.GetSet(ctx, "nonexistent", []byte("val"), time.Minute)
	if err != nil {
		t.Fatalf("GetSet() error = %v", err)
	}
	if oldVal != nil {
		t.Errorf("GetSet() on nonexistent key old = %v, want nil", oldVal)
	}
}

func TestStore_CompareAndSwap(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value1"), time.Minute)

	// Successful CAS
	swapped, err := store.CompareAndSwap(ctx, "key1", []byte("value1"), []byte("value2"), time.Minute)
	if err != nil || !swapped {
		t.Error("CompareAndSwap() with matching old value should succeed")
	}

	// Failed CAS
	swapped, err = store.CompareAndSwap(ctx, "key1", []byte("value1"), []byte("value3"), time.Minute)
	if err != nil || swapped {
		t.Error("CompareAndSwap() with non-matching old value should fail")
	}

	// CAS on non-existent key
	swapped, err = store.CompareAndSwap(ctx, "nonexistent", []byte("old"), []byte("new"), time.Minute)
	if err == nil || swapped {
		t.Error("CompareAndSwap() on nonexistent key should fail")
	}
}

func TestStore_Keys(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "user:1", []byte("a"), time.Minute)
	store.Set(ctx, "user:2", []byte("b"), time.Minute)
	store.Set(ctx, "item:1", []byte("c"), time.Minute)

	// Get all keys
	keys, err := store.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("Keys() returned %d keys, want 3", len(keys))
	}

	// Get keys with pattern
	keys, err = store.Keys(ctx, "user:*")
	if err != nil {
		t.Fatalf("Keys() with pattern error = %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Keys('user:*') returned %d keys, want 2", len(keys))
	}
}

func TestStore_Clear(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	size, err := store.Size(ctx)
	if err != nil || size != 0 {
		t.Errorf("Size() after Clear = %d, want 0", size)
	}
}

func TestStore_Size(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	size, err := store.Size(ctx)
	if err != nil || size != 0 {
		t.Errorf("Size() empty = %d, want 0", size)
	}

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)

	size, err = store.Size(ctx)
	if err != nil || size != 2 {
		t.Errorf("Size() = %d, want 2", size)
	}
}

func TestStore_CloseIdempotent(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close should be idempotent
	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("third Close() error = %v", err)
	}
	if !store.Closed() {
		t.Error("Closed() should return true after Close()")
	}
}

func TestStore_KeyValidation(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// Empty key
	_, err = store.Get(ctx, "")
	if err == nil {
		t.Error("Get() with empty key should return error")
	}

	err = store.Set(ctx, "", []byte("v"), time.Minute)
	if err == nil {
		t.Error("Set() with empty key should return error")
	}

	err = store.Delete(ctx, "")
	if err == nil {
		t.Error("Delete() with empty key should return error")
	}
}

func TestStore_ClosedRejectsOps(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	store.Close(context.Background())
	ctx := context.Background()

	_, err = store.Get(ctx, "key1")
	if err == nil {
		t.Error("Get() on closed store should return error")
	}

	err = store.Set(ctx, "key1", []byte("val"), time.Minute)
	if err == nil {
		t.Error("Set() on closed store should return error")
	}

	_, err = store.Exists(ctx, "key1")
	if err == nil {
		t.Error("Exists() on closed store should return error")
	}
}

func TestStore_GetOrSet(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()
	called := 0

	val, err := store.GetOrSet(ctx, "key1", func() ([]byte, error) {
		called++
		return []byte("computed"), nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val) != "computed" || called != 1 {
		t.Errorf("GetOrSet() first call val=%q called=%d", string(val), called)
	}

	// Second call should hit cache, fn should not be called again
	val, err = store.GetOrSet(ctx, "key1", func() ([]byte, error) {
		called++
		return []byte("computed2"), nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val) != "computed" || called != 1 {
		t.Errorf("GetOrSet() second call val=%q called=%d", string(val), called)
	}
}

func TestStore_Stats(t *testing.T) {
	store, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Get(ctx, "k1")          // hit
	store.Get(ctx, "nonexistent") // miss

	snap := store.Stats()
	if snap.Sets != 1 {
		t.Errorf("Stats().Sets = %d, want 1", snap.Sets)
	}
	if snap.Hits != 1 {
		t.Errorf("Stats().Hits = %d, want 1", snap.Hits)
	}
	if snap.Misses != 1 {
		t.Errorf("Stats().Misses = %d, want 1", snap.Misses)
	}
	if snap.StartTime.IsZero() {
		t.Error("Stats().StartTime should not be zero")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store, err := New(
		WithMaxEntries(10000),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()
	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	done := make(chan struct{}, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer func() { done <- struct{}{}; wg.Done() }()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "key:" + string(rune(id*opsPerGoroutine+j))
				store.Set(ctx, key, []byte("value"), time.Minute)
				store.Get(ctx, key)
				store.Delete(ctx, key)
			}
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}
