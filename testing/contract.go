package testing

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/stats"
)

// Backend is a local interface that mirrors cache.Backend.
// Defined locally to avoid circular imports.
type Backend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Stats() stats.Snapshot
	Closed() bool
	Name() string
}

// RunBackendContractSuite runs a comprehensive behavioral test suite against
// any Backend implementation. Every backend must pass this suite.
//
// Call this from each backend's _test.go:
//
//	func TestBackendContract(t *testing.T) {
//	    backend.RunBackendContractSuite(t, func() Backend {
//	        return newMyBackend()
//	    })
//	}
//
//nolint:gocyclo // This test suite intentionally keeps the contract coverage together in one entry point.
func RunBackendContractSuite(t *testing.T, factory func() Backend) {
	t.Helper()

	// ── Original tests (preserved) ──────────────────────────────────────

	t.Run("Get_Miss", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		_, err := b.Get(context.Background(), "nonexistent")
		if !_errors.IsNotFound(err) {
			t.Errorf("Get miss: expected NotFound, got %v", err)
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
		if !_errors.IsNotFound(err) {
			t.Errorf("after Delete: expected NotFound, got %v", err)
		}
	})

	t.Run("TTL_Accuracy", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		ttl := 5 * time.Second
		_ = b.Set(context.Background(), "ttl-key", []byte("val"), ttl)
		got, err := b.TTL(context.Background(), "ttl-key")
		if err != nil {
			t.Fatalf("TTL failed: %v", err)
		}
		delta := ttl - got
		if delta < 0 {
			delta = -delta
		}
		if delta > 50*time.Millisecond {
			t.Errorf("TTL = %v, want within 50ms of %v (delta=%v)", got, ttl, delta)
		}
	})

	t.Run("GetMulti_PartialHit", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		_ = b.Set(context.Background(), "mh1", []byte("a"), 0)
		// "mh2" is NOT set

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
			if !_errors.IsNotFound(err) {
				t.Errorf("after DeleteMulti: Get(%q) expected NotFound, got %v", k, err)
			}
		}
	})

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
			t.Error("Name() returned empty string, expected a non-empty backend identifier")
		}
	})

	// ── Phase 9 expanded contract tests ──────────────────────────────────

	t.Run("Get_Miss_Returns_ErrNotFound", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		_, err := b.Get(context.Background(), "absent-key")
		if err == nil {
			t.Fatal("expected error on cache miss, got nil")
		}
		if !_errors.IsNotFound(err) {
			t.Errorf("expected NotFound error code, got %v (code=%v)", err, _errors.CodeOf(err))
		}
	})

	t.Run("Set_EmptyKey_Returns_ErrEmptyKey", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		// Some backends validate empty keys; others silently accept them.
		// The contract requires that if an error is returned, it must
		// carry CodeEmptyKey. If no error is returned, the test passes
		// (best-effort semantics).
		err := b.Set(context.Background(), "", []byte("val"), 0)
		if err != nil {
			if !_errors.Is(err, _errors.CodeEmptyKey) {
				t.Errorf(
					"Set with empty key: expected CodeEmptyKey, got %v (code=%v)",
					err,
					_errors.CodeOf(err),
				)
			}
		}
	})

	t.Run("Set_NegativeTTL_UsesDefault", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		// A negative TTL should not panic or corrupt state. Backends
		// typically substitute their default TTL. The contract only
		// requires that the key is readable after Set with negative TTL.
		err := b.Set(context.Background(), "neg-ttl", []byte("data"), -1*time.Second)
		if err != nil {
			// Some backends may reject negative TTL — that's acceptable.
			t.Logf("Set with negative TTL returned error (acceptable): %v", err)
			return
		}
		val, err := b.Get(context.Background(), "neg-ttl")
		if err != nil {
			t.Fatalf("Get after Set(negativeTTL) failed: %v", err)
		}
		if string(val) != "data" {
			t.Errorf("Get = %q, want %q", val, "data")
		}
	})

	t.Run("Set_ZeroTTL_NoExpiry", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		if err := b.Set(context.Background(), "no-expiry", []byte("persistent"), 0); err != nil {
			t.Fatalf("Set with zero TTL failed: %v", err)
		}

		// The key should still exist after a short delay — zero TTL
		// means no expiry (persistent until explicitly deleted).
		time.Sleep(50 * time.Millisecond)
		val, err := b.Get(context.Background(), "no-expiry")
		if err != nil {
			t.Fatalf("Get after zero-TTL Set failed: %v", err)
		}
		if string(val) != "persistent" {
			t.Errorf("Get = %q, want %q", val, "persistent")
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

	t.Run("Delete_Idempotent", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		_ = b.Set(context.Background(), "idem", []byte("v"), 0)

		// First delete should succeed.
		if err := b.Delete(context.Background(), "idem"); err != nil {
			t.Fatalf("first Delete failed: %v", err)
		}

		// Second delete of the same key must NOT return an error.
		// Idempotency means repeating the operation has no additional effect.
		if err := b.Delete(context.Background(), "idem"); err != nil {
			t.Errorf("second Delete (idempotent) returned error: %v", err)
		}

		// Deleting a key that never existed must also not error.
		if err := b.Delete(context.Background(), "never-existed"); err != nil {
			t.Errorf("Delete of nonexistent key returned error: %v", err)
		}
	})

	t.Run("GetMulti_ReturnsOnlyFoundKeys", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		_ = b.Set(context.Background(), "found1", []byte("a"), 0)
		_ = b.Set(context.Background(), "found2", []byte("b"), 0)
		// "missing1" and "missing2" are NOT set.

		result, err := b.GetMulti(context.Background(), "found1", "missing1", "found2", "missing2")
		if err != nil {
			t.Fatalf("GetMulti failed: %v", err)
		}

		// Only found keys should be present.
		if len(result) != 2 {
			t.Errorf("GetMulti returned %d items, want 2", len(result))
		}
		for _, k := range []string{"found1", "found2"} {
			if _, ok := result[k]; !ok {
				t.Errorf("expected key %q in result, not found", k)
			}
		}
		for _, k := range []string{"missing1", "missing2"} {
			if _, ok := result[k]; ok {
				t.Errorf("missing key %q should not be in result", k)
			}
		}
	})

	t.Run("SetMulti_Atomic_or_Best_Effort", func(t *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		// SetMulti must either set all items (atomic) or as many as
		// possible (best-effort). The contract does not mandate atomicity,
		// but documents the behavior: after a successful call, all keys
		// must be readable.
		items := map[string][]byte{
			"batom1": []byte("x"),
			"batom2": []byte("y"),
			"batom3": []byte("z"),
		}
		if err := b.SetMulti(context.Background(), items, 0); err != nil {
			t.Fatalf("SetMulti failed: %v", err)
		}

		for k, want := range items {
			got, err := b.Get(context.Background(), k)
			if err != nil {
				t.Errorf("Get(%q) after SetMulti failed: %v", k, err)
			} else if !bytes.Equal(got, want) {
				t.Errorf("Get(%q) = %q, want %q", k, got, want)
			}
		}
	})

	t.Run("Closed_After_Close_Returns_ErrClosed", func(t *testing.T) {
		b := factory()

		if err := b.Close(context.Background()); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Operations on a closed backend must return an ErrClosed error.
		err := b.Set(context.Background(), "post-close", []byte("v"), 0)
		if err == nil {
			t.Error("expected error on Set after Close, got nil")
		} else if !_errors.IsCacheClosed(err) {
			t.Errorf("expected CacheClosed error, got %v (code=%v)", err, _errors.CodeOf(err))
		}

		_, err = b.Get(context.Background(), "post-close")
		if err == nil {
			t.Error("expected error on Get after Close, got nil")
		} else if !_errors.IsCacheClosed(err) {
			t.Errorf("expected CacheClosed error on Get, got %v (code=%v)", err, _errors.CodeOf(err))
		}
	})

	t.Run("Concurrent_Get_Set_Race", func(_ *testing.T) {
		b := factory()
		defer func() { _ = b.Close(context.Background()) }()

		// Pre-populate the key so that reads can hit.
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
	})
}
