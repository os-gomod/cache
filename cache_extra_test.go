package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/os-gomod/cache"
	"github.com/os-gomod/cache/memory"
)

func TestMemoryWithContext(t *testing.T) {
	c, err := cache.MemoryWithContext(context.Background(),
		memory.WithMaxEntries(100),
		memory.WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("MemoryWithContext() error: %v", err)
	}
	defer c.Close(context.Background())
	if c == nil {
		t.Fatal("MemoryWithContext() returned nil")
	}
}

func TestMemory_Ping(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestMemory_Exists(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	ok, err := c.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if ok {
		t.Error("Exists should return false for nonexistent key")
	}

	c.Set(ctx, "exists-key", []byte("val"), 0)
	ok, err = c.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !ok {
		t.Error("Exists should return true for existing key")
	}
}

func TestMemory_GetSet(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Set with TTL
	err = c.Set(ctx, "ttl-key", []byte("val"), time.Minute)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err := c.Get(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(val) != "val" {
		t.Errorf("Get = %q, want %q", val, "val")
	}
}

func TestMemory_GetMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	c.Set(ctx, "a", []byte("1"), 0)
	c.Set(ctx, "b", []byte("2"), 0)

	vals, err := c.GetMulti(ctx, "a", "b", "c")
	if err != nil {
		t.Fatalf("GetMulti() error: %v", err)
	}
	if len(vals) != 2 {
		t.Errorf("expected 2 values, got %d", len(vals))
	}
}

func TestMemory_DeleteMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	c.Set(ctx, "a", []byte("1"), 0)
	c.Set(ctx, "b", []byte("2"), 0)

	err = c.DeleteMulti(ctx, "a", "b")
	if err != nil {
		t.Fatalf("DeleteMulti() error: %v", err)
	}
}

func TestMemory_SetMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	err = c.SetMulti(ctx, map[string][]byte{"x": []byte("1"), "y": []byte("2")}, 0)
	if err != nil {
		t.Fatalf("SetMulti() error: %v", err)
	}
}

func TestMemory_TTL(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	c.Set(ctx, "ttl-key", []byte("val"), time.Minute)
	d, err := c.TTL(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("TTL() error: %v", err)
	}
	if d <= 0 || d > time.Minute {
		t.Errorf("TTL = %v, want between 0 and 1m", d)
	}
}

func TestMemory_KeysAndClear(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	c.Set(ctx, "a", []byte("1"), 0)
	c.Set(ctx, "b", []byte("2"), 0)

	keys, err := c.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() error: %v", err)
	}
	if len(keys) < 2 {
		t.Errorf("expected at least 2 keys, got %d", len(keys))
	}

	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	keys, err = c.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after clear, got %d", len(keys))
	}
}

func TestMemory_Size(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	c.Set(ctx, "a", []byte("1"), 0)
	c.Set(ctx, "b", []byte("2"), 0)

	size, err := c.Size(ctx)
	if err != nil {
		t.Fatalf("Size() error: %v", err)
	}
	if size != 2 {
		t.Errorf("Size = %d, want 2", size)
	}
}

func TestMemory_AtomicOps(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Increment
	val, err := c.Increment(ctx, "counter", 1)
	if err != nil {
		t.Fatalf("Increment() error: %v", err)
	}
	if val != 1 {
		t.Errorf("Increment = %d, want 1", val)
	}

	// Decrement
	val, err = c.Decrement(ctx, "counter", 1)
	if err != nil {
		t.Fatalf("Decrement() error: %v", err)
	}
	if val != 0 {
		t.Errorf("Decrement = %d, want 0", val)
	}

	// SetNX
	ok, err := c.SetNX(ctx, "nx-key", []byte("val"), 0)
	if err != nil {
		t.Fatalf("SetNX() error: %v", err)
	}
	if !ok {
		t.Error("SetNX should return true for new key")
	}

	// SetNX again should fail
	ok, err = c.SetNX(ctx, "nx-key", []byte("val2"), 0)
	if err != nil {
		t.Fatalf("SetNX() error: %v", err)
	}
	if ok {
		t.Error("SetNX should return false for existing key")
	}

	// CompareAndSwap
	ok, err = c.CompareAndSwap(ctx, "nx-key", []byte("val"), []byte("new-val"), 0)
	if err != nil {
		t.Fatalf("CAS() error: %v", err)
	}
	if !ok {
		t.Error("CAS should succeed for matching value")
	}

	ok, err = c.CompareAndSwap(ctx, "nx-key", []byte("wrong"), []byte("newer"), 0)
	if err != nil {
		t.Fatalf("CAS() error: %v", err)
	}
	if ok {
		t.Error("CAS should fail for non-matching value")
	}

	// GetSet
	oldVal, err := c.GetSet(ctx, "nx-key", []byte("replaced"), 0)
	if err != nil {
		t.Fatalf("GetSet() error: %v", err)
	}
	if string(oldVal) != "new-val" {
		t.Errorf("GetSet old = %q, want %q", oldVal, "new-val")
	}
}

func TestMemory_Stats(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	s := c.Stats()
	_ = s // Just verify it doesn't panic
}

func TestMemory_Closed(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	if c.Closed() {
		t.Error("should not be closed initially")
	}
	c.Close(context.Background())
	if !c.Closed() {
		t.Error("should be closed after Close()")
	}
}

// ─── Typed tests ───

func TestTypedCache_NewJSON(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	type User struct {
		Name string `json:"name"`
	}

	tc := cache.NewJSONTypedCache[User](c)
	if tc == nil {
		t.Fatal("NewJSONTypedCache returned nil")
	}
	if tc.Name() != "typed:memory" {
		t.Errorf("Name() = %q, want %q", tc.Name(), "typed:memory")
	}
	if tc.Backend() == nil {
		t.Error("Backend() should not be nil")
	}
	if tc.Codec() == nil {
		t.Error("Codec() should not be nil")
	}

	// Set/Get
	err = tc.Set(ctx, "user:1", User{Name: "Alice"}, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	u, err := tc.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if u.Name != "Alice" {
		t.Errorf("got %q, want Alice", u.Name)
	}

	// Exists
	ok, err := tc.Exists(ctx, "user:1")
	if err != nil || !ok {
		t.Error("Exists should return true")
	}

	// Delete
	err = tc.Delete(ctx, "user:1")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err = tc.Get(ctx, "user:1")
	if err == nil {
		t.Error("expected error after delete")
	}

	// Ping
	if err := tc.Ping(ctx); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestTypedCache_EmptyKey(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())

	tc := cache.NewJSONTypedCache[struct{}](c)
	_, err = tc.Get(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty key")
	}
	err = tc.Set(context.Background(), "", struct{}{}, 0)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestTypedCache_RawCodec(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewRawTypedCache(c)
	err = tc.Set(ctx, "key", []byte("raw-value"), 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err := tc.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(val) != "raw-value" {
		t.Errorf("got %q, want raw-value", val)
	}
}

func TestTypedCache_StringCodec(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	err = tc.Set(ctx, "key", "hello", 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err := tc.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("got %q, want hello", val)
	}
}

func TestTypedCache_Int64Codec(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewInt64TypedCache(c)
	err = tc.Set(ctx, "counter", 42, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err := tc.Get(ctx, "counter")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != 42 {
		t.Errorf("got %d, want 42", val)
	}
}

func TestTypedCache_Float64Codec(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewFloat64TypedCache(c)
	err = tc.Set(ctx, "pi", 3.14, 0)
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err := tc.Get(ctx, "pi")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != 3.14 {
		t.Errorf("got %f, want 3.14", val)
	}
}

func TestTypedCache_Close(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}

	tc := cache.NewJSONTypedCache[string](c)
	if err := tc.Close(context.Background()); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !tc.Closed() {
		t.Error("should be closed")
	}
}

func TestTypedCache_Stats(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())

	tc := cache.NewJSONTypedCache[string](c)
	s := tc.Stats()
	_ = s // Just verify it doesn't panic
}

func TestTypedCache_GetOrSet(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	called := false
	val, err := tc.GetOrSet(ctx, "gorset-key", func() (string, error) {
		called = true
		return "computed", nil
	}, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error: %v", err)
	}
	if !called {
		t.Error("fn should be called on miss")
	}
	if val != "computed" {
		t.Errorf("got %q, want computed", val)
	}

	// Second call should hit cache
	called = false
	val, err = tc.GetOrSet(ctx, "gorset-key", func() (string, error) {
		called = true
		return "recomputed", nil
	}, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error: %v", err)
	}
	if called {
		t.Error("fn should not be called on hit")
	}
	if val != "computed" {
		t.Errorf("got %q, want computed", val)
	}
}

func TestTypedCache_GetOrSet_Error(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())

	tc := cache.NewStringTypedCache(c)
	_, err = tc.GetOrSet(context.Background(), "gorset-err", func() (string, error) {
		return "", context.DeadlineExceeded
	}, 0)
	if err == nil {
		t.Error("expected error from fn")
	}
}

func TestNewMemoryInt64Cache(t *testing.T) {
	tc, err := cache.NewMemoryInt64Cache(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("NewMemoryInt64Cache() error: %v", err)
	}
	defer tc.Close(context.Background())
	ctx := context.Background()

	tc.Set(ctx, "counter", 100, 0)
	val, err := tc.Get(ctx, "counter")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != 100 {
		t.Errorf("got %d, want 100", val)
	}
}

func TestNewMemoryStringCache(t *testing.T) {
	tc, err := cache.NewMemoryStringCache(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("NewMemoryStringCache() error: %v", err)
	}
	defer tc.Close(context.Background())
	ctx := context.Background()

	tc.Set(ctx, "greeting", "hello", 0)
	val, err := tc.Get(ctx, "greeting")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("got %q, want hello", val)
	}
}

func TestNewMemoryTypedCache(t *testing.T) {
	type Data struct {
		Value int `json:"value"`
	}
	tc, err := cache.NewMemoryTypedCache[Data](
		[]memory.Option{memory.WithCleanupInterval(0)},
	)
	if err != nil {
		t.Fatalf("NewMemoryTypedCache() error: %v", err)
	}
	defer tc.Close(context.Background())
	ctx := context.Background()

	tc.Set(ctx, "data:1", Data{Value: 42}, 0)
	val, err := tc.Get(ctx, "data:1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val.Value != 42 {
		t.Errorf("got %d, want 42", val.Value)
	}
}

func TestTypedCache_WithOnSetError(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	var setErrorKeys []string
	tc := cache.NewJSONTypedCache[string](c, cache.WithOnSetError[string](
		func(key string, err error) {
			setErrorKeys = append(setErrorKeys, key)
		},
	))

	// Normal set should not trigger callback
	tc.Set(ctx, "key1", "val1", 0)
	if len(setErrorKeys) != 0 {
		t.Error("onSetError should not be called for successful set")
	}
}

func TestTypedCache_TTL(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	tc.Set(ctx, "ttl-key", "val", time.Minute)
	d, err := tc.TTL(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("TTL() error: %v", err)
	}
	if d <= 0 || d > time.Minute {
		t.Errorf("TTL = %v, want between 0 and 1m", d)
	}
}

func TestTypedCache_GetMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	tc.Set(ctx, "a", "1", 0)
	tc.Set(ctx, "b", "2", 0)

	vals, err := tc.GetMulti(ctx, "a", "b", "c")
	if err != nil {
		t.Fatalf("GetMulti() error: %v", err)
	}
	if len(vals) != 2 {
		t.Errorf("expected 2 values, got %d", len(vals))
	}
}

func TestTypedCache_SetMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	err = tc.SetMulti(ctx, map[string]string{"x": "1", "y": "2"}, 0)
	if err != nil {
		t.Fatalf("SetMulti() error: %v", err)
	}
}

func TestTypedCache_DeleteMulti(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	tc.Set(ctx, "a", "1", 0)
	tc.Set(ctx, "b", "2", 0)

	err = tc.DeleteMulti(ctx, "a", "b")
	if err != nil {
		t.Fatalf("DeleteMulti() error: %v", err)
	}
}

func TestTypedCache_TypeAliases(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())

	// Type aliases should work
	var _ *cache.TypedInt64Cache = cache.NewInt64TypedCache(c)
	var _ *cache.TypedStringCache = cache.NewStringTypedCache(c)
	var _ *cache.TypedBytesCache = cache.NewRawTypedCache(c)
}

func TestTypedCache_AtomicOps(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewInt64TypedCache(c)

	// Increment
	val, err := tc.Increment(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("Increment() error: %v", err)
	}
	if val != 5 {
		t.Errorf("Increment = %d, want 5", val)
	}

	// Decrement
	val, err = tc.Decrement(ctx, "counter", 2)
	if err != nil {
		t.Fatalf("Decrement() error: %v", err)
	}
	if val != 3 {
		t.Errorf("Decrement = %d, want 3", val)
	}

	// SetNX
	ok, err := tc.SetNX(ctx, "nx-key", 100, 0)
	if err != nil || !ok {
		t.Fatal("SetNX should succeed for new key")
	}

	// CompareAndSwap
	ok, err = tc.CompareAndSwap(ctx, "nx-key", 100, 200, 0)
	if err != nil || !ok {
		t.Fatal("CAS should succeed for matching value")
	}

	// GetSet
	old, err := tc.GetSet(ctx, "nx-key", 300, 0)
	if err != nil {
		t.Fatalf("GetSet() error: %v", err)
	}
	if old != 200 {
		t.Errorf("GetSet old = %d, want 200", old)
	}
}

func TestTypedCache_ScanOps(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	tc := cache.NewStringTypedCache(c)
	tc.Set(ctx, "a", "1", 0)
	tc.Set(ctx, "b", "2", 0)

	keys, err := tc.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() error: %v", err)
	}
	if len(keys) < 2 {
		t.Errorf("expected at least 2 keys, got %d", len(keys))
	}

	size, err := tc.Size(ctx)
	if err != nil {
		t.Fatalf("Size() error: %v", err)
	}
	if size != 2 {
		t.Errorf("Size = %d, want 2", size)
	}

	err = tc.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}
}
