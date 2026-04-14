package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
	"github.com/os-gomod/cache/observability"
)

func newTestCache(t *testing.T, opts ...Option) *Cache {
	t.Helper()
	c, err := New(opts...)
	if err != nil {
		t.Fatalf("failed to create memory cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// ---------------------------------------------------------------------------
// Basic KV Operations
// ---------------------------------------------------------------------------

func TestCache_SetGet(t *testing.T) {
	c := newTestCache(t)

	if err := c.Set(context.Background(), "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := c.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("got %q, want %q", val, "value1")
	}
}

func TestCache_Get_NotFound(t *testing.T) {
	c := newTestCache(t)

	_, err := c.Get(context.Background(), "missing")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 0)
	if err := c.Delete(context.Background(), "key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := c.Get(context.Background(), "key1")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestCache_Exists(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 0)

	ok, err := c.Exists(context.Background(), "key1")
	if err != nil || !ok {
		t.Errorf("Exists(key1) = %v, %v; want true, nil", ok, err)
	}

	ok, err = c.Exists(context.Background(), "missing")
	if err != nil || ok {
		t.Errorf("Exists(missing) = %v, %v; want false, nil", ok, err)
	}
}

func TestCache_TTL(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 5*time.Second)

	ttl, err := c.TTL(context.Background(), "key1")
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 || ttl > 5*time.Second {
		t.Errorf("TTL = %v, want between 0 and 5s", ttl)
	}
}

func TestCache_TTL_NotFound(t *testing.T) {
	c := newTestCache(t)

	_, err := c.TTL(context.Background(), "missing")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Batch Operations
// ---------------------------------------------------------------------------

func TestCache_SetMulti_GetMulti(t *testing.T) {
	c := newTestCache(t)

	items := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
		"k3": []byte("v3"),
	}
	if err := c.SetMulti(context.Background(), items, 0); err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	result, err := c.GetMulti(context.Background(), "k1", "k2", "k3", "missing")
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("GetMulti returned %d items, want 3", len(result))
	}
	if string(result["k1"]) != "v1" {
		t.Errorf("GetMulti[k1] = %q, want %q", result["k1"], "v1")
	}
}

func TestCache_DeleteMulti(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "k1", []byte("v1"), 0)
	_ = c.Set(context.Background(), "k2", []byte("v2"), 0)

	if err := c.DeleteMulti(context.Background(), "k1", "k2"); err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	ok, _ := c.Exists(context.Background(), "k1")
	if ok {
		t.Error("k1 should be deleted")
	}
}

// ---------------------------------------------------------------------------
// Atomic Operations
// ---------------------------------------------------------------------------

func TestCache_SetNX(t *testing.T) {
	c := newTestCache(t)

	set, err := c.SetNX(context.Background(), "nx-key", []byte("first"), 0)
	if err != nil || !set {
		t.Errorf("first SetNX = %v, %v; want true, nil", set, err)
	}

	set, err = c.SetNX(context.Background(), "nx-key", []byte("second"), 0)
	if err != nil || set {
		t.Errorf("second SetNX = %v, %v; want false, nil", set, err)
	}

	val, _ := c.Get(context.Background(), "nx-key")
	if string(val) != "first" {
		t.Errorf("value = %q, want %q", val, "first")
	}
}

func TestCache_Increment_Decrement(t *testing.T) {
	c := newTestCache(t)

	val, err := c.Increment(context.Background(), "counter", 5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 5 {
		t.Errorf("Increment = %d, want 5", val)
	}

	val, err = c.Decrement(context.Background(), "counter", 3)
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}
	if val != 2 {
		t.Errorf("Decrement = %d, want 2", val)
	}
}

func TestCache_CompareAndSwap(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "cas-key", []byte("old"), 0)

	swapped, err := c.CompareAndSwap(
		context.Background(),
		"cas-key",
		[]byte("old"),
		[]byte("new"),
		0,
	)
	if err != nil {
		t.Fatalf("CAS failed: %v", err)
	}
	if !swapped {
		t.Error("CAS should succeed with matching old value")
	}

	val, _ := c.Get(context.Background(), "cas-key")
	if string(val) != "new" {
		t.Errorf("after CAS, value = %q, want %q", val, "new")
	}
}

func TestCache_CompareAndSwap_Mismatch(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "cas-key", []byte("actual"), 0)

	swapped, _ := c.CompareAndSwap(
		context.Background(),
		"cas-key",
		[]byte("wrong"),
		[]byte("new"),
		0,
	)
	if swapped {
		t.Error("CAS should fail with mismatched old value")
	}
}

func TestCache_GetSet(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "gs-key", []byte("old"), 0)

	old, err := c.GetSet(context.Background(), "gs-key", []byte("new"), 0)
	if err != nil {
		t.Fatalf("GetSet failed: %v", err)
	}
	if string(old) != "old" {
		t.Errorf("GetSet returned old = %q, want %q", old, "old")
	}

	val, _ := c.Get(context.Background(), "gs-key")
	if string(val) != "new" {
		t.Errorf("after GetSet, value = %q, want %q", val, "new")
	}
}

// ---------------------------------------------------------------------------
// GetOrSet (Singleflight)
// ---------------------------------------------------------------------------

func TestCache_GetOrSet(t *testing.T) {
	c := newTestCache(t)

	var calls int
	val, err := c.GetOrSet(context.Background(), "gos-key", func() ([]byte, error) {
		calls++
		return []byte("computed"), nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if string(val) != "computed" {
		t.Errorf("GetOrSet = %q, want %q", val, "computed")
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1", calls)
	}

	// Second call should return cached value without calling fn.
	val, err = c.GetOrSet(context.Background(), "gos-key", func() ([]byte, error) {
		calls++
		return []byte("should-not-run"), nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("second GetOrSet failed: %v", err)
	}
	if string(val) != "computed" {
		t.Errorf("second GetOrSet = %q, want %q", val, "computed")
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1 (cached)", calls)
	}
}

// ---------------------------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------------------------

func TestCache_Keys(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "alpha", []byte("1"), 0)
	_ = c.Set(context.Background(), "beta", []byte("2"), 0)

	keys, err := c.Keys(context.Background(), "*")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Keys returned %d items, want 2", len(keys))
	}
}

func TestCache_Size(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "k1", []byte("v1"), 0)
	_ = c.Set(context.Background(), "k2", []byte("v2"), 0)

	size, err := c.Size(context.Background())
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 2 {
		t.Errorf("Size = %d, want 2", size)
	}
}

func TestCache_Clear(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "k1", []byte("v1"), 0)
	_ = c.Set(context.Background(), "k2", []byte("v2"), 0)

	if err := c.Clear(context.Background()); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	size, _ := c.Size(context.Background())
	if size != 0 {
		t.Errorf("Size after Clear = %d, want 0", size)
	}
}

func TestCache_Stats(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "k1", []byte("v1"), 0)
	_, _ = c.Get(context.Background(), "k1")
	_, _ = c.Get(context.Background(), "miss")

	snap := c.Stats()
	if snap.Sets != 1 {
		t.Errorf("Sets = %d, want 1", snap.Sets)
	}
	if snap.Hits != 1 {
		t.Errorf("Hits = %d, want 1", snap.Hits)
	}
	if snap.Misses != 1 {
		t.Errorf("Misses = %d, want 1", snap.Misses)
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestCache_Close(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if c.Closed() {
		t.Error("cache should not be closed initially")
	}

	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !c.Closed() {
		t.Error("cache should be closed after Close")
	}

	// Operations on closed cache should return error.
	err = c.Set(context.Background(), "k", []byte("v"), 0)
	if !_errors.IsCacheClosed(err) {
		t.Errorf("expected CacheClosed error, got %v", err)
	}
}

func TestCache_DoubleClose(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_ = c.Close(context.Background())
	_ = c.Close(context.Background()) // should not panic
}

// ---------------------------------------------------------------------------
// Observability Provider Injection
// ---------------------------------------------------------------------------

func TestCache_WithInterceptors(t *testing.T) {
	c := newTestCache(t, WithInterceptors(observability.NopInterceptor{}))

	if c.chain.IsEmpty() {
		t.Error("WithInterceptors should create a non-empty chain")
	}
}

func TestCache_SetInterceptors(t *testing.T) {
	c := newTestCache(t)

	c.SetInterceptors(observability.NopInterceptor{})

	if c.chain.IsEmpty() {
		t.Error("SetInterceptors should create a non-empty chain")
	}
}

func TestCache_SetInterceptors_Nil(t *testing.T) {
	c := newTestCache(t)
	original := c.chain

	c.SetInterceptors()

	if c.chain != original {
		t.Error("SetInterceptors() should not change the chain")
	}
}

// ---------------------------------------------------------------------------
// Expire / Persist
// ---------------------------------------------------------------------------

func TestCache_Expire(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 0)

	if err := c.Expire(context.Background(), "key1", 10*time.Second); err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	ttl, _ := c.TTL(context.Background(), "key1")
	if ttl <= 0 || ttl > 10*time.Second {
		t.Errorf("TTL after Expire = %v, want between 0 and 10s", ttl)
	}
}

func TestCache_Persist(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 5*time.Second)

	if err := c.Persist(context.Background(), "key1"); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Context Cancellation
// ---------------------------------------------------------------------------

func TestCache_Get_CancelledContext(t *testing.T) {
	c := newTestCache(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Get(ctx, "key")
	// Cancelled context should still work for memory cache
	// since it doesn't check context in most paths
	_ = err
}

// ---------------------------------------------------------------------------
// TTL Expiration
// ---------------------------------------------------------------------------

func TestCache_TTL_Expiration(t *testing.T) {
	c := newTestCache(t, WithCleanupInterval(10*time.Millisecond))

	_ = c.Set(context.Background(), "short-lived", []byte("val"), 50*time.Millisecond)

	val, err := c.Get(context.Background(), "short-lived")
	if err != nil || string(val) != "val" {
		t.Fatalf("immediate Get failed: val=%q err=%v", val, err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = c.Get(context.Background(), "short-lived")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound after TTL expiry, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Bypass Cache
// ---------------------------------------------------------------------------

func TestCache_BypassCache(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "key1", []byte("val"), 0)

	ctx := cachectx.NoCache(context.Background())
	_, err := c.Get(ctx, "key1")
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound with bypass context, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Concurrent Access
// ---------------------------------------------------------------------------

func TestCache_ConcurrentAccess(t *testing.T) {
	c := newTestCache(t, WithShards(4))
	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "key-" + string(rune('A'+id%26))
			for j := 0; j < opsPerGoroutine; j++ {
				_ = c.Set(context.Background(), key, []byte("val"), 0)
				_, _ = c.Get(context.Background(), key)
			}
		}(i)
	}
	wg.Wait()
}

// // ---------------------------------------------------------------------------
// // Backend Contract
// // ---------------------------------------------------------------------------

// func TestBackendContract(t *testing.T) {
// 	cachetesting.RunBackendContractSuite(t, func() Backend {
// 		c, err := New()
// 		if err != nil {
// 			t.Fatalf("New failed: %v", err)
// 		}
// 		return c
// 	})
// }
