package memory

import (
	"context"
	"testing"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/cachectx"
)

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGet_Miss(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_, err := c.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get() on missing key should return error")
	}
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %T: %v", err, err)
	}
}

func TestGet_Hit(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	val, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "value" {
		t.Errorf("Get() = %q, want %q", string(val), "value")
	}
}

func TestGet_ExpiredKey(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Set with a very short TTL.
	if err := c.Set(ctx, "shortlived", []byte("data"), 10*time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	// Wait for expiry.
	time.Sleep(50 * time.Millisecond)

	_, err = c.Get(ctx, "shortlived")
	if err == nil {
		t.Fatal("Get() on expired key should return error")
	}
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// Set
// ---------------------------------------------------------------------------

func TestSet_Overwrite(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v1"), 0)
	_ = c.Set(ctx, "k", []byte("v2"), 0)

	val, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "v2" {
		t.Errorf("Get() = %q, want %q after overwrite", string(val), "v2")
	}
}

func TestSet_NilValue(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// Setting nil value should succeed (no panic).
	if err := c.Set(ctx, "nilkey", nil, 0); err != nil {
		t.Fatalf("Set(nil) error = %v", err)
	}

	val, err := c.Get(ctx, "nilkey")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if val != nil {
		t.Errorf("Get() = %v, want nil", val)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete_Existing(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "delme", []byte("val"), 0)
	if err := c.Delete(ctx, "delme"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err := c.Get(ctx, "delme")
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("after Delete, expected NotFound, got %v", err)
	}
}

func TestDelete_NonExisting(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// Deleting a non-existing key should not return an error.
	if err := c.Delete(ctx, "ghost"); err != nil {
		t.Errorf("Delete(ghost) error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Exists
// ---------------------------------------------------------------------------

func TestExists_True(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "here", []byte("yes"), 0)
	ok, err := c.Exists(ctx, "here")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !ok {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_False(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	ok, err := c.Exists(ctx, "nope")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if ok {
		t.Error("Exists() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// TTL / Expire / Persist
// ---------------------------------------------------------------------------

func TestTTL_AfterSet(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	ttl := 5 * time.Minute
	_ = c.Set(ctx, "k", []byte("v"), ttl)

	remaining, err := c.TTL(ctx, "k")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if remaining <= 0 {
		t.Errorf("TTL() = %v, want > 0", remaining)
	}
}

func TestTTL_NotFound(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_, err := c.TTL(ctx, "missing")
	if err == nil {
		t.Fatal("TTL() on missing key should return error")
	}
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %T: %v", err, err)
	}
}

func TestExpire_Existing(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 10*time.Minute)

	newTTL := 1 * time.Hour
	if err := c.Expire(ctx, "k", newTTL); err != nil {
		t.Fatalf("Expire() error = %v", err)
	}

	remaining, err := c.TTL(ctx, "k")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	// Remaining should be close to the new TTL.
	if remaining <= 50*time.Minute {
		t.Errorf("TTL after Expire = %v, want close to %v", remaining, newTTL)
	}
}

func TestExpire_NotFound(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	err := c.Expire(ctx, "ghost", 5*time.Minute)
	if err == nil {
		t.Fatal("Expire() on missing key should return error")
	}
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %T: %v", err, err)
	}
}

func TestExpire_NegativeTTL(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 10*time.Minute)

	err := c.Expire(ctx, "k", -1*time.Second)
	if err == nil {
		t.Fatal("Expire() with negative TTL should return error")
	}
}

func TestPersist_Existing(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 10*time.Minute)

	if err := c.Persist(ctx, "k"); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	remaining, err := c.TTL(ctx, "k")
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	// Persistent entries have TTL = 0.
	if remaining != 0 {
		t.Errorf("TTL after Persist = %v, want 0", remaining)
	}
}

// ---------------------------------------------------------------------------
// Keys
// ---------------------------------------------------------------------------

func TestKeys_ReturnsAll(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k1", []byte("v1"), 0)
	_ = c.Set(ctx, "k2", []byte("v2"), 0)
	_ = c.Set(ctx, "k3", []byte("v3"), 0)

	keys, err := c.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("Keys() returned %d keys, want 3", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, want := range []string{"k1", "k2", "k3"} {
		if !keySet[want] {
			t.Errorf("Keys() missing %q", want)
		}
	}
}

func TestKeys_ExcludesExpired(t *testing.T) {
	c, err := New(WithMaxEntries(100), WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	_ = c.Set(ctx, "permanent", []byte("yes"), 0)
	_ = c.Set(ctx, " fleeting", []byte("no"), 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	keys, err := c.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	for _, k := range keys {
		if k == " fleeting" {
			t.Error("Keys() should not include expired key")
		}
	}
	// "permanent" should still be present.
	found := false
	for _, k := range keys {
		if k == "permanent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Keys() should include non-expired key 'permanent'")
	}
}

// ---------------------------------------------------------------------------
// Size / Clear
// ---------------------------------------------------------------------------

func TestSize_AfterOperations(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	size, err := c.Size(ctx)
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}
	if size != 0 {
		t.Errorf("initial Size() = %d, want 0", size)
	}

	_ = c.Set(ctx, "k1", []byte("v1"), 0)
	_ = c.Set(ctx, "k2", []byte("v2"), 0)

	size, _ = c.Size(ctx)
	if size != 2 {
		t.Errorf("Size() after 2 sets = %d, want 2", size)
	}

	_ = c.Delete(ctx, "k1")

	size, _ = c.Size(ctx)
	if size != 1 {
		t.Errorf("Size() after 1 delete = %d, want 1", size)
	}
}

func TestClear_RemovesAll(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), 0)
	_ = c.Set(ctx, "b", []byte("2"), 0)
	_ = c.Set(ctx, "c", []byte("3"), 0)

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	keys, _ := c.Keys(ctx, "*")
	if len(keys) != 0 {
		t.Errorf("Keys() after Clear = %v, want empty", keys)
	}

	size, _ := c.Size(ctx)
	if size != 0 {
		t.Errorf("Size() after Clear = %d, want 0", size)
	}
}

// ---------------------------------------------------------------------------
// BypassCache
// ---------------------------------------------------------------------------

func TestBypassCache(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set(context.Background(), "k", []byte("v"), 0)

	// With bypass context, Get should miss even though the key exists.
	bypassCtx := cachectx.NoCache(context.Background())
	_, err := c.Get(bypassCtx, "k")
	if err == nil {
		t.Fatal("Get with bypass context should return error (miss)")
	}
	if !cacheerrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %T: %v", err, err)
	}

	// Exists with bypass should also return false.
	ok, err := c.Exists(bypassCtx, "k")
	if err != nil {
		t.Fatalf("Exists(bypass) error = %v", err)
	}
	if ok {
		t.Error("Exists with bypass should return false")
	}

	// Normal context should still work.
	val, err := c.Get(context.Background(), "k")
	if err != nil {
		t.Fatalf("Get(normal) error = %v", err)
	}
	if string(val) != "v" {
		t.Errorf("Get(normal) = %q, want %q", string(val), "v")
	}
}
