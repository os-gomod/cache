// Package testing provides contract tests and test utilities for verifying
// cache backend implementations. All backends (memory, redis, layer, resilience)
// should pass RunBackendContractSuite to ensure behavioral consistency.
package testing

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/stats"
)

// Backend is an alias for the canonical backend interface.
type Backend = backendiface.Backend

// RunBackendContractSuite runs a comprehensive set of tests against any Backend
// implementation. The factory function should return a fresh, ready-to-use backend
// for each subtest. The suite covers basic CRUD, TTL, batch operations, concurrent
// access, and closed-cache behavior.
func RunBackendContractSuite(t *testing.T, factory func() Backend) {
	t.Helper()
	t.Run("CRUD", func(t *testing.T) { testCRUD(t, factory) })
	t.Run("Batch", func(t *testing.T) { testBatch(t, factory) })
	t.Run("Lifecycle", func(t *testing.T) { testLifecycle(t, factory) })
	t.Run("Concurrency", func(t *testing.T) { testConcurrency(t, factory) })
}

func testCRUD(t *testing.T, factory func() Backend) {
	t.Helper()
	t.Run("Get_Miss", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		_, err := b.Get(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error on miss, got nil")
		}
	})
	t.Run("Set_Get_Roundtrip", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		if err := b.Set(context.Background(), "k1", []byte("v1"), 0); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		val, err := b.Get(context.Background(), "k1")
		if err != nil {
			t.Fatalf("Get after Set failed: %v", err)
		}
		if string(val) != "v1" {
			t.Errorf("Get = %q, want %q", val, "v1")
		}
	})
	t.Run("Delete_Removes", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		_ = b.Set(context.Background(), "k1", []byte("v1"), 0)
		if err := b.Delete(context.Background(), "k1"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err := b.Get(context.Background(), "k1")
		if err == nil {
			t.Error("expected error after Delete, got nil")
		}
	})
	t.Run("TTL_Accuracy_Within_50ms", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		ttl := 10 * time.Second
		if err := b.Set(context.Background(), "ttl-acc", []byte("v"), ttl); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		got, err := b.TTL(context.Background(), "ttl-acc")
		if err != nil {
			t.Fatalf("TTL failed: %v", err)
		}
		delta := ttl - got
		if delta < 0 {
			delta = -delta
		}
		if delta > 50*time.Millisecond {
			t.Errorf("TTL accuracy: got %v, want within 50ms of %v (delta=%v)", got, ttl, delta)
		}
	})
}

func testBatch(t *testing.T, factory func() Backend) {
	t.Helper()
	t.Run("GetMulti_PartialHit", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		_ = b.Set(context.Background(), "mh1", []byte("a"), 0)
		result, err := b.GetMulti(context.Background(), "mh1", "mh2")
		if err != nil {
			t.Fatalf("GetMulti failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("GetMulti returned %d items, want 1", len(result))
		}
		if string(result["mh1"]) != "a" {
			t.Errorf("GetMulti[mh1] = %q, want %q", result["mh1"], "a")
		}
		if _, ok := result["mh2"]; ok {
			t.Error("GetMulti[mh2] should not be present on miss")
		}
	})
	t.Run("SetMulti_AllPresent", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		items := map[string][]byte{
			"sm1": []byte("x"),
			"sm2": []byte("y"),
		}
		if err := b.SetMulti(context.Background(), items, 0); err != nil {
			t.Fatalf("SetMulti failed: %v", err)
		}
		for k, want := range items {
			got, err := b.Get(context.Background(), k)
			if err != nil {
				t.Errorf("Get(%q) failed: %v", k, err)
			} else if !bytes.Equal(got, want) {
				t.Errorf("Get(%q) = %q, want %q", k, got, want)
			}
		}
	})
	t.Run("DeleteMulti_AllRemoved", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		_ = b.Set(context.Background(), "dm1", []byte("a"), 0)
		_ = b.Set(context.Background(), "dm2", []byte("b"), 0)
		if err := b.DeleteMulti(context.Background(), "dm1", "dm2"); err != nil {
			t.Fatalf("DeleteMulti failed: %v", err)
		}
		for _, k := range []string{"dm1", "dm2"} {
			_, err := b.Get(context.Background(), k)
			if err == nil {
				t.Errorf("after DeleteMulti: Get(%q) expected error, got nil", k)
			}
		}
	})
}

func testLifecycle(t *testing.T, factory func() Backend) {
	t.Helper()
	t.Run("Closed_After_Close", func(t *testing.T) {
		b := factory()
		if b.Closed() {
			t.Error("new backend should not be closed")
		}
		if err := b.Close(context.Background()); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
		if !b.Closed() {
			t.Error("backend should be closed after Close")
		}
	})
	t.Run("Ping_While_Open", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		if err := b.Ping(context.Background()); err != nil {
			t.Errorf("Ping on open backend failed: %v", err)
		}
	})
	t.Run("Name_Returns_NonEmpty", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		name := b.Name()
		if name == "" {
			t.Error("Name() returned empty string")
		}
	})
	t.Run("Stats_HasFields", func(_ *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()
		snap := b.Stats()
		_ = snap.Hits
		_ = snap.Misses
		_ = snap.Sets
		_ = snap.Gets
		_ = snap.HitRate
		_ = snap.Uptime
	})
}

func testConcurrency(t *testing.T, factory func() Backend) {
	t.Helper()
	b := factory()
	defer func() { _ = b.Close(context.Background()) }()
	_ = b.Set(context.Background(), "race-key", []byte("initial"), 0)
	const goroutines = 100
	const opsPerGoroutine = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if j%2 == 0 {
					_, _ = b.Get(context.Background(), "race-key")
				} else {
					_ = b.Set(context.Background(), "race-key",
						[]byte("val-from-"+string(rune('A'+id%26))), 0)
				}
			}
		}(i)
	}
	wg.Wait()
}

// Stats is an alias for stats.Snapshot to avoid an extra import in consumers.
type Stats = stats.Snapshot
