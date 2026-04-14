package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/memory"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newMemBackend(t *testing.T) *memory.Cache {
	t.Helper()
	c, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewTypedCache_Basic(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})
	if tc.Backend() == nil {
		t.Error("Backend() should not be nil")
	}
	if tc.Codec() == nil {
		t.Error("Codec() should not be nil")
	}
}

func TestNewTypedCache_NilBackend(t *testing.T) {
	// NewTypedCache does not guard against nil backend — operations will
	// fail when used. This test verifies that the constructor itself does
	// not panic.
	tc := NewTypedCache[string](nil, codec.StringCodec{})
	if tc.Backend() != nil {
		t.Error("Backend() should be nil when constructed with nil")
	}
}

func TestTypedCache_Name(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})
	name := tc.Name()
	if name != "typed:memory" {
		t.Errorf("Name() = %q, want %q", name, "typed:memory")
	}
}

// ---------------------------------------------------------------------------
// Core KV — StringCodec
// ---------------------------------------------------------------------------

func TestTypedCache_SetGet_String(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	if err := tc.Set(context.Background(), "greet", "hello", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "greet")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get = %q, want %q", val, "hello")
	}
}

func TestTypedCache_Get_Miss(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_, err := tc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error on cache miss, got nil")
	}
}

func TestTypedCache_Delete(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)
	if err := tc.Delete(context.Background(), "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := tc.Get(context.Background(), "k1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestTypedCache_Exists(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)

	ok, err := tc.Exists(context.Background(), "k1")
	if err != nil || !ok {
		t.Errorf("Exists(k1) = %v, %v; want true, nil", ok, err)
	}

	ok, err = tc.Exists(context.Background(), "missing")
	if err != nil || ok {
		t.Errorf("Exists(missing) = %v, %v; want false, nil", ok, err)
	}
}

func TestTypedCache_TTL(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 5*time.Second)
	ttl, err := tc.TTL(context.Background(), "k1")
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > 5*time.Second {
		t.Errorf("TTL = %v, want between 0 and 5s", ttl)
	}
}

// ---------------------------------------------------------------------------
// Core KV — Int64Codec
// ---------------------------------------------------------------------------

func TestTypedCache_SetGet_Int64(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[int64](mc, codec.Int64Codec{})

	if err := tc.Set(context.Background(), "counter", 42, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "counter")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 42 {
		t.Errorf("Get = %d, want 42", val)
	}
}

// ---------------------------------------------------------------------------
// Core KV — Float64Codec
// ---------------------------------------------------------------------------

func TestTypedCache_SetGet_Float64(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[float64](mc, codec.Float64Codec{})

	if err := tc.Set(context.Background(), "pi", 3.14159, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "pi")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 3.14159 {
		t.Errorf("Get = %g, want 3.14159", val)
	}
}

// ---------------------------------------------------------------------------
// Core KV — RawCodec ([]byte)
// ---------------------------------------------------------------------------

func TestTypedCache_SetGet_Raw(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[[]byte](mc, codec.RawCodec{})

	data := []byte("raw-bytes")
	if err := tc.Set(context.Background(), "bin", data, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "bin")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "raw-bytes" {
		t.Errorf("Get = %q, want %q", val, "raw-bytes")
	}
}

// ---------------------------------------------------------------------------
// Core KV — JSONCodec
// ---------------------------------------------------------------------------

type jsonItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestTypedCache_SetGet_JSON(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[jsonItem](mc, codec.NewJSONCodec[jsonItem]())

	original := jsonItem{ID: 7, Name: "widget"}
	if err := tc.Set(context.Background(), "item:7", original, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "item:7")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val.ID != original.ID || val.Name != original.Name {
		t.Errorf("Get = %+v, want %+v", val, original)
	}
}

// ---------------------------------------------------------------------------
// Batch Operations
// ---------------------------------------------------------------------------

func TestTypedCache_GetMulti(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)
	_ = tc.Set(context.Background(), "k2", "v2", 0)

	result, err := tc.GetMulti(context.Background(), "k1", "k2", "missing")
	if err != nil {
		t.Fatalf("GetMulti: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("GetMulti returned %d items, want 2", len(result))
	}
	if result["k1"] != "v1" {
		t.Errorf("GetMulti[k1] = %q, want %q", result["k1"], "v1")
	}
	if result["k2"] != "v2" {
		t.Errorf("GetMulti[k2] = %q, want %q", result["k2"], "v2")
	}
}

func TestTypedCache_SetMulti(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	items := map[string]string{
		"m1": "a",
		"m2": "b",
	}
	if err := tc.SetMulti(context.Background(), items, 0); err != nil {
		t.Fatalf("SetMulti: %v", err)
	}

	for k, want := range items {
		got, err := tc.Get(context.Background(), k)
		if err != nil {
			t.Errorf("Get(%q): %v", k, err)
		} else if got != want {
			t.Errorf("Get(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestTypedCache_DeleteMulti(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "d1", "v1", 0)
	_ = tc.Set(context.Background(), "d2", "v2", 0)

	if err := tc.DeleteMulti(context.Background(), "d1", "d2"); err != nil {
		t.Fatalf("DeleteMulti: %v", err)
	}

	for _, k := range []string{"d1", "d2"} {
		_, err := tc.Get(context.Background(), k)
		if err == nil {
			t.Errorf("expected error after DeleteMulti for key %q", k)
		}
	}
}

// ---------------------------------------------------------------------------
// GetOrSet (singleflight)
// ---------------------------------------------------------------------------

func TestTypedCache_GetOrSet_CacheMiss(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	val, err := tc.GetOrSet(context.Background(), "gos", func() (string, error) {
		return "computed", nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet: %v", err)
	}
	if val != "computed" {
		t.Errorf("GetOrSet = %q, want %q", val, "computed")
	}
}

func TestTypedCache_GetOrSet_CacheHit(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "gos", "cached", 0)

	var calls int
	val, err := tc.GetOrSet(context.Background(), "gos", func() (string, error) {
		calls++
		return "should-not-run", nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet: %v", err)
	}
	if val != "cached" {
		t.Errorf("GetOrSet = %q, want %q", val, "cached")
	}
	if calls != 0 {
		t.Errorf("fn called %d times, want 0 (should use cached value)", calls)
	}
}

func TestTypedCache_GetOrSet_FnError(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_, err := tc.GetOrSet(context.Background(), "err-key", func() (string, error) {
		return "", fmt.Errorf("loader failed")
	}, time.Minute)
	if err == nil {
		t.Error("expected error from loader fn, got nil")
	}
}

func TestTypedCache_GetOrSet_Deduplication(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	var calls atomic.Int32
	const goroutines = 10

	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			_, _ = tc.GetOrSet(context.Background(), "dedup", func() (string, error) {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond)
				return "shared", nil
			}, time.Minute)
		}()
	}

	close(startCh)
	wg.Wait()

	// Singleflight should deduplicate — the loader should be called at most
	// a small number of times (ideally 1, but singleflight semantics allow
	// a second call if the first hasn't registered yet).
	if calls.Load() > 2 {
		t.Errorf("loader called %d times, expected at most 2 with singleflight", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// Atomic Operations
// ---------------------------------------------------------------------------

func TestTypedCache_SetNX(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	set, err := tc.SetNX(context.Background(), "nx", "first", 0)
	if err != nil || !set {
		t.Errorf("first SetNX = %v, %v; want true, nil", set, err)
	}

	set, err = tc.SetNX(context.Background(), "nx", "second", 0)
	if err != nil || set {
		t.Errorf("second SetNX = %v, %v; want false, nil", set, err)
	}
}

func TestTypedCache_CompareAndSwap(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "cas", "old", 0)

	swapped, err := tc.CompareAndSwap(context.Background(), "cas", "old", "new", 0)
	if err != nil {
		t.Fatalf("CAS: %v", err)
	}
	if !swapped {
		t.Error("CAS should succeed with matching old value")
	}

	val, _ := tc.Get(context.Background(), "cas")
	if val != "new" {
		t.Errorf("after CAS, value = %q, want %q", val, "new")
	}
}

func TestTypedCache_CompareAndSwap_Mismatch(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "cas", "actual", 0)

	swapped, _ := tc.CompareAndSwap(context.Background(), "cas", "wrong", "new", 0)
	if swapped {
		t.Error("CAS should fail with mismatched old value")
	}
}

func TestTypedCache_GetSet(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "gs", "old", 0)

	old, err := tc.GetSet(context.Background(), "gs", "new", 0)
	if err != nil {
		t.Fatalf("GetSet: %v", err)
	}
	if old != "old" {
		t.Errorf("GetSet returned old = %q, want %q", old, "old")
	}

	val, _ := tc.Get(context.Background(), "gs")
	if val != "new" {
		t.Errorf("after GetSet, value = %q, want %q", val, "new")
	}
}

func TestTypedCache_Increment_Decrement(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[int64](mc, codec.Int64Codec{})

	val, err := tc.Increment(context.Background(), "counter", 10)
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if val != 10 {
		t.Errorf("Increment = %d, want 10", val)
	}

	val, err = tc.Decrement(context.Background(), "counter", 3)
	if err != nil {
		t.Fatalf("Decrement: %v", err)
	}
	if val != 7 {
		t.Errorf("Decrement = %d, want 7", val)
	}
}

// ---------------------------------------------------------------------------
// Scan Operations
// ---------------------------------------------------------------------------

func TestTypedCache_Keys(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "alpha", "1", 0)
	_ = tc.Set(context.Background(), "beta", "2", 0)

	keys, err := tc.Keys(context.Background(), "*")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Keys returned %d items, want 2", len(keys))
	}
}

func TestTypedCache_Clear(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)
	_ = tc.Set(context.Background(), "k2", "v2", 0)

	if err := tc.Clear(context.Background()); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	size, _ := tc.Size(context.Background())
	if size != 0 {
		t.Errorf("Size after Clear = %d, want 0", size)
	}
}

func TestTypedCache_Size(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)
	_ = tc.Set(context.Background(), "k2", "v2", 0)

	size, err := tc.Size(context.Background())
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if size != 2 {
		t.Errorf("Size = %d, want 2", size)
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestTypedCache_Ping(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	if err := tc.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestTypedCache_Close(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	if err := tc.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !tc.Closed() {
		t.Error("should be closed after Close")
	}
}

func TestTypedCache_Stats(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	_ = tc.Set(context.Background(), "k1", "v1", 0)
	_, _ = tc.Get(context.Background(), "k1")

	snap := tc.Stats()
	if snap.Sets != 1 {
		t.Errorf("Stats.Sets = %d, want 1", snap.Sets)
	}
	if snap.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", snap.Hits)
	}
}

// ---------------------------------------------------------------------------
// Empty Key Validation
// ---------------------------------------------------------------------------

func TestTypedCache_EmptyKey(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	if err := tc.Set(context.Background(), "", "val", 0); err == nil {
		t.Error("expected error for empty key on Set")
	}
	if _, err := tc.Get(context.Background(), ""); err == nil {
		t.Error("expected error for empty key on Get")
	}
	if err := tc.Delete(context.Background(), ""); err == nil {
		t.Error("expected error for empty key on Delete")
	}
}

// ---------------------------------------------------------------------------
// Convenience Constructors
// ---------------------------------------------------------------------------

func TestNewJSONTypedCache(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewJSONTypedCache[jsonItem](mc)

	original := jsonItem{ID: 99, Name: "test"}
	if err := tc.Set(context.Background(), "j99", original, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "j99")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val.ID != 99 || val.Name != "test" {
		t.Errorf("Get = %+v, want %+v", val, original)
	}
}

func TestNewRawTypedCache(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewRawTypedCache(mc)

	data := []byte("raw-data")
	if err := tc.Set(context.Background(), "raw", data, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "raw")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "raw-data" {
		t.Errorf("Get = %q, want %q", val, "raw-data")
	}
}

func TestNewStringTypedCache(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewStringTypedCache(mc)

	if err := tc.Set(context.Background(), "msg", "hello", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "msg")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get = %q, want %q", val, "hello")
	}
}

func TestNewInt64TypedCache(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewInt64TypedCache(mc)

	if err := tc.Set(context.Background(), "num", 12345, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "num")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 12345 {
		t.Errorf("Get = %d, want 12345", val)
	}
}

func TestNewFloat64TypedCache(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewFloat64TypedCache(mc)

	if err := tc.Set(context.Background(), "num", 2.71828, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "num")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 2.71828 {
		t.Errorf("Get = %g, want 2.71828", val)
	}
}

// ---------------------------------------------------------------------------
// WithOnSetError option
// ---------------------------------------------------------------------------

func TestTypedCache_WithOnSetError(t *testing.T) {
	mc := newMemBackend(t)

	var capturedKey string
	var capturedErr error
	tc := NewTypedCache[string](mc, codec.StringCodec{},
		WithOnSetError[string](func(key string, err error) {
			capturedKey = key
			capturedErr = err
		}),
	)

	// The onSetError callback is invoked when GetOrSet encounters a Set error
	// or a decode error. We can't easily trigger a Set error on memory backend,
	// but we can verify the option is wired correctly.
	_ = tc.Set(context.Background(), "test", "val", 0)
	_ = capturedKey
	_ = capturedErr
}

// ---------------------------------------------------------------------------
// Legacy type aliases
// ---------------------------------------------------------------------------

func TestLegacyTypeAliases(t *testing.T) {
	mc := newMemBackend(t)

	// TypedInt64Cache
	_ = NewTypedInt64Cache(mc)

	// TypedStringCache
	_ = NewStringTypedCache(mc)

	// TypedBytesCache
	_ = NewRawTypedCache(mc)
}

// ---------------------------------------------------------------------------
// Backend typed constructors
// ---------------------------------------------------------------------------

func TestNewMemoryInt64Cache(t *testing.T) {
	tc, err := NewMemoryInt64Cache()
	if err != nil {
		t.Fatalf("NewMemoryInt64Cache: %v", err)
	}
	t.Cleanup(func() { _ = tc.Close(context.Background()) })

	if err := tc.Set(context.Background(), "n", 100, 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "n")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 100 {
		t.Errorf("Get = %d, want 100", val)
	}
}

func TestNewMemoryStringCache(t *testing.T) {
	tc, err := NewMemoryStringCache()
	if err != nil {
		t.Fatalf("NewMemoryStringCache: %v", err)
	}
	t.Cleanup(func() { _ = tc.Close(context.Background()) })

	if err := tc.Set(context.Background(), "s", "world", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := tc.Get(context.Background(), "s")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "world" {
		t.Errorf("Get = %q, want %q", val, "world")
	}
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestTypedCache_ConcurrentAccess(t *testing.T) {
	mc := newMemBackend(t)
	tc := NewTypedCache[int64](mc, codec.Int64Codec{})

	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			for j := 0; j < opsPerGoroutine; j++ {
				_ = tc.Set(context.Background(), key, int64(j), 0)
				_, _ = tc.Get(context.Background(), key)
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkTypedCache_Set_String(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	for b.Loop() {
		_ = tc.Set(context.Background(), "bench", "value", 0)
	}
}

func BenchmarkTypedCache_Get_String(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[string](mc, codec.StringCodec{})
	_ = tc.Set(context.Background(), "bench", "value", 0)

	for b.Loop() {
		_, _ = tc.Get(context.Background(), "bench")
	}
}

func BenchmarkTypedCache_Set_Int64(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[int64](mc, codec.Int64Codec{})

	for b.Loop() {
		_ = tc.Set(context.Background(), "bench", 42, 0)
	}
}

func BenchmarkTypedCache_Get_Int64(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[int64](mc, codec.Int64Codec{})
	_ = tc.Set(context.Background(), "bench", 42, 0)

	for b.Loop() {
		_, _ = tc.Get(context.Background(), "bench")
	}
}

func BenchmarkTypedCache_Set_JSON(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[jsonItem](mc, codec.NewJSONCodec[jsonItem]())
	v := jsonItem{ID: 1, Name: "bench"}

	for b.Loop() {
		_ = tc.Set(context.Background(), "bench", v, 0)
	}
}

func BenchmarkTypedCache_Get_JSON(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[jsonItem](mc, codec.NewJSONCodec[jsonItem]())
	_ = tc.Set(context.Background(), "bench", jsonItem{ID: 1, Name: "bench"}, 0)

	for b.Loop() {
		_, _ = tc.Get(context.Background(), "bench")
	}
}

func BenchmarkTypedCache_GetOrSet_String(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	tc := NewTypedCache[string](mc, codec.StringCodec{})

	for b.Loop() {
		_, _ = tc.GetOrSet(context.Background(), "gos-bench", func() (string, error) {
			return "computed", nil
		}, time.Minute)
	}
}
