package layered

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/memory"
)

func newTestLayered(t *testing.T) (*Store, func()) {
	t.Helper()

	l1, err := memory.New(
		memory.WithMaxEntries(1000),
		memory.WithCleanupInterval(0),
		memory.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("Failed to create L1: %v", err)
	}

	l2, err := memory.New(
		memory.WithMaxEntries(1000),
		memory.WithCleanupInterval(0),
		memory.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("Failed to create L2: %v", err)
	}

	store, err := New(
		WithL1(l1),
		WithL2(l2),
		WithPromoteOnHit(true),
		WithWriteBack(false),
	)
	if err != nil {
		t.Fatalf("Failed to create layered store: %v", err)
	}

	cleanup := func() {
		store.Close(context.Background())
	}
	return store, cleanup
}

func TestStore_New(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	if store.Name() != "layered" {
		t.Errorf("Name() = %q, want %q", store.Name(), "layered")
	}
	if store.Closed() {
		t.Error("newly created store should not be closed")
	}
}

func TestStore_NewWithoutL1(t *testing.T) {
	l2, _ := memory.New(memory.WithCleanupInterval(0))
	_, err := New(WithL2(l2))
	if err == nil {
		t.Error("New() without L1 should return error")
	}
}

func TestStore_NewWithoutL2(t *testing.T) {
	l1, _ := memory.New(memory.WithCleanupInterval(0))
	_, err := New(WithL1(l1))
	if err == nil {
		t.Error("New() without L2 should return error")
	}
}

func TestStore_GetSet(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	// Set through layered
	err := store.Set(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get through layered
	val, err := store.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("Get() = %q, want %q", string(val), "value1")
	}

	// Non-existent key
	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get() of nonexistent key should return error")
	}
}

func TestStore_L1Hit(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	// Set - writes to both L1 and L2
	store.Set(ctx, "key1", []byte("value1"), time.Minute)

	// First get - L1 hit (value was written there by Set)
	val, err := store.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("Get() = %q, want %q", string(val), "value1")
	}

	snap := store.Stats()
	if snap.Hits < 1 {
		t.Errorf("Stats().Hits = %d, want >= 1", snap.Hits)
	}
}

func TestStore_L2Promotion(t *testing.T) {
	l1, _ := memory.New(memory.WithCleanupInterval(0))
	l2, _ := memory.New(memory.WithCleanupInterval(0))

	// Set value only in L2
	l2.Set(context.Background(), "key1", []byte("l2value"), time.Minute)

	store, err := New(
		WithL1(l1),
		WithL2(l2),
		WithPromoteOnHit(true),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close(context.Background())

	ctx := context.Background()

	// Get - should find in L2 and promote to L1
	val, err := store.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "l2value" {
		t.Errorf("Get() = %q, want %q", string(val), "l2value")
	}

	// Verify it's now in L1
	l1val, err := l1.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("L1 Get() error = %v", err)
	}
	if string(l1val) != "l2value" {
		t.Errorf("L1 value = %q, want %q after promotion", string(l1val), "l2value")
	}
}

func TestStore_Delete(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value1"), time.Minute)
	store.Delete(ctx, "key1")

	_, err := store.Get(ctx, "key1")
	if err == nil {
		t.Error("Get() after delete should return error")
	}
}

func TestStore_Exists(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	exists, err := store.Exists(ctx, "key1")
	if err != nil || exists {
		t.Error("Exists() of nonexistent key should return false")
	}

	store.Set(ctx, "key1", []byte("value1"), time.Minute)
	exists, err = store.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Error("Exists() of existing key should return true")
	}
}

func TestStore_GetMulti(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

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
}

func TestStore_SetMulti(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

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
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)

	store.DeleteMulti(ctx, "k1", "k2")

	_, err := store.Get(ctx, "k1")
	if err == nil {
		t.Error("k1 should be deleted")
	}
}

func TestStore_Increment(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	val, err := store.Increment(ctx, "counter", 5)
	if err != nil || val != 5 {
		t.Errorf("Increment() = %d, want 5", val)
	}

	val, err = store.Increment(ctx, "counter", 3)
	if err != nil || val != 8 {
		t.Errorf("Increment() = %d, want 8", val)
	}
}

func TestStore_GetSetMethod(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("old"), time.Minute)

	oldVal, err := store.GetSet(ctx, "key1", []byte("new"), time.Minute)
	if err != nil {
		t.Fatalf("GetSet() error = %v", err)
	}
	if string(oldVal) != "old" {
		t.Errorf("GetSet() old = %q, want %q", string(oldVal), "old")
	}

	val, err := store.Get(ctx, "key1")
	if err != nil || string(val) != "new" {
		t.Error("GetSet() should have updated the value")
	}
}

func TestStore_SetNX(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	set, err := store.SetNX(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil || !set {
		t.Error("SetNX() on new key should return true")
	}

	set, err = store.SetNX(ctx, "key1", []byte("value2"), time.Minute)
	if err != nil || set {
		t.Error("SetNX() on existing key should return false")
	}
}

func TestStore_Keys(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Set(ctx, "k2", []byte("v2"), time.Minute)

	keys, err := store.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}
}

func TestStore_Clear(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Clear(ctx)

	size, err := store.Size(ctx)
	if err != nil || size != 0 {
		t.Errorf("Size() after Clear = %d, want 0", size)
	}
}

func TestStore_Size(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

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

func TestStore_Ping(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestStore_CloseIdempotent(t *testing.T) {
	store, cleanup := newTestLayered(t)
	cleanup() // first close

	if err := store.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if !store.Closed() {
		t.Error("Closed() should return true after Close()")
	}
}

func TestStore_Stats(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "k1", []byte("v1"), time.Minute)
	store.Get(ctx, "k1")
	store.Get(ctx, "nonexistent")

	snap := store.Stats()
	if snap.Sets < 1 {
		t.Errorf("Stats().Sets = %d, want >= 1", snap.Sets)
	}
	if snap.StartTime.IsZero() {
		t.Error("Stats().StartTime should not be zero")
	}
}

func TestStore_CompareAndSwap(t *testing.T) {
	store, cleanup := newTestLayered(t)
	defer cleanup()

	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value1"), time.Minute)

	swapped, err := store.CompareAndSwap(
		ctx,
		"key1",
		[]byte("value1"),
		[]byte("value2"),
		time.Minute,
	)
	if err != nil || !swapped {
		t.Error("CompareAndSwap() with matching value should succeed")
	}

	swapped, err = store.CompareAndSwap(ctx, "key1", []byte("wrong"), []byte("value3"), time.Minute)
	if err != nil || swapped {
		t.Error("CompareAndSwap() with non-matching value should fail")
	}
}
