package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// GetMulti
// ---------------------------------------------------------------------------

func TestGetMulti_MixedHitsAndMisses(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), 0)
	_ = c.Set(ctx, "b", []byte("2"), 0)

	result, err := c.GetMulti(ctx, "a", "b", "missing")
	if err != nil {
		t.Fatalf("GetMulti() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("GetMulti() returned %d entries, want 2", len(result))
	}
	if string(result["a"]) != "1" {
		t.Errorf("result[a] = %q, want %q", string(result["a"]), "1")
	}
	if string(result["b"]) != "2" {
		t.Errorf("result[b] = %q, want %q", string(result["b"]), "2")
	}
	if _, ok := result["missing"]; ok {
		t.Error("result should not contain 'missing'")
	}
}

// ---------------------------------------------------------------------------
// SetMulti / DeleteMulti
// ---------------------------------------------------------------------------

func TestSetMultiple(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	items := map[string][]byte{
		"x": []byte("10"),
		"y": []byte("20"),
		"z": []byte("30"),
	}
	if err := c.SetMulti(ctx, items, 0); err != nil {
		t.Fatalf("SetMulti() error = %v", err)
	}

	for k, expected := range items {
		val, err := c.Get(ctx, k)
		if err != nil {
			t.Errorf("Get(%q) error = %v", k, err)
			continue
		}
		if string(val) != string(expected) {
			t.Errorf("Get(%q) = %q, want %q", k, string(val), string(expected))
		}
	}
}

func TestDeleteMulti(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "d1", []byte("v1"), 0)
	_ = c.Set(ctx, "d2", []byte("v2"), 0)
	_ = c.Set(ctx, "d3", []byte("v3"), 0)

	if err := c.DeleteMulti(ctx, "d1", "d3"); err != nil {
		t.Fatalf("DeleteMulti() error = %v", err)
	}

	// d1 and d3 should be gone.
	_, err1 := c.Get(ctx, "d1")
	_, err3 := c.Get(ctx, "d3")
	if err1 == nil {
		t.Error("d1 should be deleted")
	}
	if err3 == nil {
		t.Error("d3 should be deleted")
	}

	// d2 should still exist.
	val, err := c.Get(ctx, "d2")
	if err != nil {
		t.Fatalf("Get(d2) error = %v", err)
	}
	if string(val) != "v2" {
		t.Errorf("Get(d2) = %q, want %q", string(val), "v2")
	}
}

// ---------------------------------------------------------------------------
// GetOrSet
// ---------------------------------------------------------------------------

func TestGetOrSet_Miss_CallsFn(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	var called atomic.Int32
	fn := func() ([]byte, error) {
		called.Add(1)
		return []byte("computed"), nil
	}

	val, err := c.GetOrSet(ctx, "newkey", fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val) != "computed" {
		t.Errorf("GetOrSet() = %q, want %q", string(val), "computed")
	}
	if called.Load() != 1 {
		t.Errorf("fn called %d times, want 1", called.Load())
	}

	// Value should now be cached.
	cached, err := c.Get(ctx, "newkey")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(cached) != "computed" {
		t.Errorf("cached value = %q, want %q", string(cached), "computed")
	}
}

func TestGetOrSet_Hit_ReturnsCached(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "cached", []byte("original"), 0)

	var called atomic.Int32
	fn := func() ([]byte, error) {
		called.Add(1)
		return []byte("should-not-see"), nil
	}

	val, err := c.GetOrSet(ctx, "cached", fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if string(val) != "original" {
		t.Errorf("GetOrSet() = %q, want %q", string(val), "original")
	}
	if called.Load() != 0 {
		t.Errorf("fn should not be called on cache hit, but was called %d times", called.Load())
	}
}

// ---------------------------------------------------------------------------
// GetSet
// ---------------------------------------------------------------------------

func TestGetSet_ReturnsOldAndSetsNew(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "gs", []byte("old"), 0)

	old, err := c.GetSet(ctx, "gs", []byte("new"), 0)
	if err != nil {
		t.Fatalf("GetSet() error = %v", err)
	}
	if string(old) != "old" {
		t.Errorf("GetSet() old = %q, want %q", string(old), "old")
	}

	val, _ := c.Get(ctx, "gs")
	if string(val) != "new" {
		t.Errorf("after GetSet, value = %q, want %q", string(val), "new")
	}
}

func TestGetSet_Miss_ReturnsNilAndSets(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	old, err := c.GetSet(ctx, "new_gs", []byte("fresh"), 0)
	if err != nil {
		t.Fatalf("GetSet() error = %v", err)
	}
	if old != nil {
		t.Errorf("GetSet() old = %v, want nil on miss", old)
	}

	val, _ := c.Get(ctx, "new_gs")
	if string(val) != "fresh" {
		t.Errorf("after GetSet, value = %q, want %q", string(val), "fresh")
	}
}

// ---------------------------------------------------------------------------
// CompareAndSwap
// ---------------------------------------------------------------------------

func TestCompareAndSwap_Match(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	_ = c.Set(ctx, "cas", []byte("old"), 0)

	swapped, err := c.CompareAndSwap(ctx, "cas", []byte("old"), []byte("new"), 0)
	if err != nil {
		t.Fatalf("CompareAndSwap() error = %v", err)
	}
	if !swapped {
		t.Error("CompareAndSwap() = false, want true")
	}

	val, _ := c.Get(ctx, "cas")
	if string(val) != "new" {
		t.Errorf("after CAS, value = %q, want %q", string(val), "new")
	}
}

func TestCompareAndSwap_Mismatch(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "cas2", []byte("actual"), 0)

	swapped, err := c.CompareAndSwap(ctx, "cas2", []byte("wrong"), []byte("new"), 0)
	if err != nil {
		t.Fatalf("CompareAndSwap() error = %v", err)
	}
	if swapped {
		t.Error("CompareAndSwap() = true, want false on mismatch")
	}

	// Original value should be unchanged.
	val, _ := c.Get(ctx, "cas2")
	if string(val) != "actual" {
		t.Errorf("after failed CAS, value = %q, want %q", string(val), "actual")
	}
}

func TestCompareAndSwap_NotFound(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	swapped, err := c.CompareAndSwap(ctx, "nope", []byte("old"), []byte("new"), 0)
	if err != nil {
		t.Fatalf("CompareAndSwap() error = %v", err)
	}
	if swapped {
		t.Error("CompareAndSwap() on missing key = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Increment / Decrement
// ---------------------------------------------------------------------------

func TestIncrement_NewKey(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	val, err := c.Increment(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if val != 5 {
		t.Errorf("Increment(new, 5) = %d, want 5", val)
	}
}

func TestIncrement_ExistingKey(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "counter", []byte("10"), 0)

	val, err := c.Increment(ctx, "counter", 7)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if val != 17 {
		t.Errorf("Increment(10, 7) = %d, want 17", val)
	}
}

func TestDecrement(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "counter", []byte("100"), 0)

	val, err := c.Decrement(ctx, "counter", 30)
	if err != nil {
		t.Fatalf("Decrement() error = %v", err)
	}
	if val != 70 {
		t.Errorf("Decrement(100, 30) = %d, want 70", val)
	}
}

// ---------------------------------------------------------------------------
// SetNX
// ---------------------------------------------------------------------------

func TestSetNX_NewKey(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	set, err := c.SetNX(ctx, "nx_key", []byte("first"), 0)
	if err != nil {
		t.Fatalf("SetNX() error = %v", err)
	}
	if !set {
		t.Error("SetNX() = false, want true for new key")
	}

	val, _ := c.Get(ctx, "nx_key")
	if string(val) != "first" {
		t.Errorf("after SetNX, value = %q, want %q", string(val), "first")
	}
}

func TestSetNX_ExistingKey(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	_ = c.Set(ctx, "nx_exist", []byte("original"), 0)

	set, err := c.SetNX(ctx, "nx_exist", []byte("should-not-set"), 0)
	if err != nil {
		t.Fatalf("SetNX() error = %v", err)
	}
	if set {
		t.Error("SetNX() = true, want false for existing key")
	}

	val, _ := c.Get(ctx, "nx_exist")
	if string(val) != "original" {
		t.Errorf("value unchanged = %q, want %q", string(val), "original")
	}
}

// ---------------------------------------------------------------------------
// Concurrency sanity check
// ---------------------------------------------------------------------------

func TestConcurrentIncrement(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	const goroutines = 10
	const increments = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				_, _ = c.Increment(ctx, "shared_counter", 1)
			}
		}()
	}
	wg.Wait()

	val, err := c.Increment(ctx, "shared_counter", 0)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	expected := int64(goroutines * increments)
	if val != expected {
		t.Errorf("concurrent increment result = %d, want %d", val, expected)
	}
}
