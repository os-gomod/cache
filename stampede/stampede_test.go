package stampede

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
	goredis "github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Detector tests
// ---------------------------------------------------------------------------

func TestDetector_CacheMiss(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	var calls atomic.Int64
	val, err := d.Do(context.Background(), "key1", nil, nil,
		func(ctx context.Context) ([]byte, error) {
			calls.Add(1)
			return []byte("result"), nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "result" {
		t.Errorf("val = %q, want %q", val, "result")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

func TestDetector_CacheHit_NoEarlyRefresh(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	// Entry far from soft expiry — ShouldEarlyRefresh returns false.
	entry := eviction.NewEntry("key1", []byte("cached"), 10*time.Minute, 0)

	var calls atomic.Int64
	val, err := d.Do(context.Background(), "key1", entry.Value, entry,
		func(ctx context.Context) ([]byte, error) {
			calls.Add(1)
			return []byte("fresh"), nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return the cached value without calling fn.
	if string(val) != "cached" {
		t.Errorf("val = %q, want %q", val, "cached")
	}
	if calls.Load() != 0 {
		t.Errorf("calls = %d, want 0 (no refresh needed)", calls.Load())
	}
}

func TestDetector_EarlyRefresh_PastSoftExpiry(t *testing.T) {
	d := NewDetector(1.0, nil)
	// Don't use defer d.Close() here — we call it explicitly to wait for the
	// background goroutine.

	// Entry past soft expiry — ShouldEarlyRefresh returns true.
	entry := eviction.NewEntry("key1", []byte("stale"), 5*time.Second, 0)
	// Force soft expiry into the past.
	entry.SoftExpiresAt = time.Now().Add(-1 * time.Second).UnixNano()
	// Hard expiry still in the future.
	entry.ExpiresAt = time.Now().Add(10 * time.Second).UnixNano()

	var refreshed atomic.Bool
	var onRefreshCalled atomic.Bool
	val, err := d.Do(context.Background(), "key1", entry.Value, entry,
		func(ctx context.Context) ([]byte, error) {
			refreshed.Store(true)
			return []byte("fresh"), nil
		},
		func(b []byte) {
			onRefreshCalled.Store(true)
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Background refresh: Do returns stale value immediately.
	if string(val) != "stale" {
		t.Errorf("val = %q, want %q (stale returned immediately)", val, "stale")
	}

	// Wait for background goroutine to complete.
	d.Close()

	if !refreshed.Load() {
		t.Error("expected fn to be called for early refresh")
	}
	if !onRefreshCalled.Load() {
		t.Error("expected onRefresh to be called after successful refresh")
	}
}

func TestDetector_EarlyRefresh_FailedRefresh(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	// Entry past soft expiry.
	entry := eviction.NewEntry("key1", []byte("stale"), 5*time.Second, 0)
	entry.SoftExpiresAt = time.Now().Add(-1 * time.Second).UnixNano()
	entry.ExpiresAt = time.Now().Add(10 * time.Second).UnixNano()

	var onRefreshCalled atomic.Bool
	val, err := d.Do(context.Background(), "key1", entry.Value, entry,
		func(ctx context.Context) ([]byte, error) {
			return nil, context.DeadlineExceeded
		},
		func(b []byte) {
			onRefreshCalled.Store(true)
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return stale value even when refresh fails.
	if string(val) != "stale" {
		t.Errorf("val = %q, want %q", val, "stale")
	}

	d.Close()

	// onRefresh should NOT be called for a failed refresh.
	if onRefreshCalled.Load() {
		t.Error("onRefresh should not be called on refresh failure")
	}
}

func TestDetector_Deduplication(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	var calls atomic.Int64
	var wg sync.WaitGroup
	const n = 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := d.Do(context.Background(), "hotkey", nil, nil,
				func(ctx context.Context) ([]byte, error) {
					calls.Add(1)
					time.Sleep(50 * time.Millisecond)
					return []byte("result"), nil
				},
				nil,
			)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(val) != "result" {
				t.Errorf("val = %q, want %q", val, "result")
			}
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("fn called %d times, want 1 (deduplication)", calls.Load())
	}
}

func TestDetector_EarlyRefresh_OnlyOneRefreshes(t *testing.T) {
	d := NewDetector(1.0, nil)
	// Don't use defer d.Close() — we call it explicitly.

	// Entry past soft expiry.
	entry := eviction.NewEntry("key1", []byte("stale"), 5*time.Second, 0)
	entry.SoftExpiresAt = time.Now().Add(-1 * time.Second).UnixNano()
	entry.ExpiresAt = time.Now().Add(10 * time.Second).UnixNano()

	var refreshCalls atomic.Int64
	var wg sync.WaitGroup
	const n = 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := d.Do(context.Background(), "key1", entry.Value, entry,
				func(ctx context.Context) ([]byte, error) {
					refreshCalls.Add(1)
					time.Sleep(50 * time.Millisecond)
					return []byte("fresh"), nil
				},
				func(b []byte) {},
			)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			// All callers should get the stale value (background refresh).
			if string(val) != "stale" {
				t.Errorf("val = %q, want %q", val, "stale")
			}
		}()
	}
	wg.Wait()

	// Wait for the background goroutine to finish.
	d.Close()

	// Only one goroutine should have performed the refresh.
	if refreshCalls.Load() != 1 {
		t.Errorf("refresh called %d times, want 1", refreshCalls.Load())
	}
}

func TestDetector_ContextCancellation(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := d.Do(ctx, "key1", nil, nil,
		func(ctx context.Context) ([]byte, error) {
			return []byte("result"), nil
		},
		nil,
	)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestDetector_BetaValidation(t *testing.T) {
	d := NewDetector(0, nil) // beta <= 0 should be replaced with DefaultBeta.
	if d.beta != DefaultBeta {
		t.Errorf("beta = %v, want %v", d.beta, DefaultBeta)
	}
}

func TestDetector_NilChain(t *testing.T) {
	d := NewDetector(1.0, nil)
	if d.metrics == nil {
		t.Error("metrics should be set to NopChain, not nil")
	}
}

// ---------------------------------------------------------------------------
// DistributedLock tests (using miniredis)
// ---------------------------------------------------------------------------

func setupMiniredis(t *testing.T) (goredis.UniversalClient, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	client := goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs: []string{mr.Addr()},
	})
	return client, mr
}

func TestDistributedLock_AcquireAndRelease(t *testing.T) {
	client, mr := setupMiniredis(t)
	defer mr.Close()

	token := GenerateToken()
	lock, acquired, err := AcquireLock(
		context.Background(),
		client,
		"lock:key1",
		token,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}

	// Verify the key exists in Redis.
	val, err := client.Get(context.Background(), "lock:key1").Result()
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	if val != token {
		t.Errorf("lock value = %q, want %q", val, token)
	}

	// Release the lock.
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release error: %v", err)
	}

	// Verify the key is gone.
	_, err = client.Get(context.Background(), "lock:key1").Result()
	if err == nil {
		t.Error("expected key to be deleted after release")
	}
}

func TestDistributedLock_AcquireFails_WhenHeld(t *testing.T) {
	client, mr := setupMiniredis(t)
	defer mr.Close()

	token1 := GenerateToken()
	lock1, acquired, err := AcquireLock(
		context.Background(),
		client,
		"lock:key1",
		token1,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected first lock to be acquired")
	}
	defer func() { _ = lock1.Release(context.Background()) }()

	// Second acquisition with a different token should fail.
	token2 := GenerateToken()
	_, acquired2, err := AcquireLock(
		context.Background(),
		client,
		"lock:key1",
		token2,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if acquired2 {
		t.Error("expected second lock acquisition to fail")
	}
}

func TestDistributedLock_Release_Idempotent(t *testing.T) {
	client, mr := setupMiniredis(t)
	defer mr.Close()

	token := GenerateToken()
	lock, acquired, err := AcquireLock(
		context.Background(),
		client,
		"lock:idem",
		token,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}

	// Release multiple times — should not panic or error.
	for i := 0; i < 3; i++ {
		if err := lock.Release(context.Background()); err != nil {
			t.Fatalf("Release %d error: %v", i+1, err)
		}
	}
}

func TestDistributedLock_Renewal(t *testing.T) {
	client, mr := setupMiniredis(t)
	defer mr.Close()

	token := GenerateToken()
	ttl := 2 * time.Second
	lock, acquired, err := AcquireLock(context.Background(), client, "lock:renew", token, ttl)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}
	defer func() { _ = lock.Release(context.Background()) }()

	// Wait for at least one renewal cycle (60% of TTL = 1.2s).
	time.Sleep(1500 * time.Millisecond)

	// Verify the lock key still exists and its TTL has been extended.
	exists, err := client.Exists(context.Background(), "lock:renew").Result()
	if err != nil {
		t.Fatalf("EXISTS error: %v", err)
	}
	if exists != 1 {
		t.Fatal("expected lock key to still exist after renewal")
	}

	// Check TTL is still close to the original (not near zero).
	keyTTL, err := client.TTL(context.Background(), "lock:renew").Result()
	if err != nil {
		t.Fatalf("TTL error: %v", err)
	}
	if keyTTL < ttl/2 {
		t.Errorf("TTL = %v, expected >= %v (renewal should have extended it)", keyTTL, ttl/2)
	}
}

func TestDistributedLock_GenerateToken(t *testing.T) {
	t1 := GenerateToken()
	t2 := GenerateToken()
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
	if len(t1) != 32 { // 16 bytes → 32 hex chars
		t.Errorf("token length = %d, want 32", len(t1))
	}
}

func BenchmarkDetector_Get(b *testing.B) {
	d := NewDetector(1.0, observability.NopChain())
	defer d.Close()

	entry := eviction.NewEntry("bench", []byte("value"), 10*time.Minute, 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Do(ctx, "bench", entry.Value, entry,
			func(ctx context.Context) ([]byte, error) {
				return []byte("fresh"), nil
			},
			nil,
		)
	}
}

func BenchmarkDetector_Miss(b *testing.B) {
	d := NewDetector(1.0, observability.NopChain())
	defer d.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Do(ctx, "key", nil, nil,
			func(ctx context.Context) ([]byte, error) {
				return []byte("result"), nil
			},
			nil,
		)
	}
}

func BenchmarkDistributedLock_Acquire(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	client := goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs: []string{mr.Addr()},
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := GenerateToken()
		lock, ok, _ := AcquireLock(ctx, client, "bench:lock", token, 5*time.Second)
		if ok && lock != nil {
			_ = lock.Release(ctx)
		}
	}
}
