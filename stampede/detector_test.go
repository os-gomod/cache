package stampede

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/memory/eviction"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(0.5, nil)
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.beta != 0.5 {
		t.Errorf("expected beta=0.5, got %f", d.beta)
	}
}

func TestNewDetector_ZeroBeta(t *testing.T) {
	d := NewDetector(0, nil)
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.beta != DefaultBeta {
		t.Errorf("expected beta=%f (DefaultBeta), got %f", DefaultBeta, d.beta)
	}
}

func TestNewDetector_NegativeBeta(t *testing.T) {
	d := NewDetector(-1, nil)
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.beta != DefaultBeta {
		t.Errorf("expected beta=%f (DefaultBeta), got %f", DefaultBeta, d.beta)
	}
}

func TestDo_NilEntry_CallsFn(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	called := false
	fn := func(ctx context.Context) ([]byte, error) {
		called = true
		return []byte("result"), nil
	}

	val, err := d.Do(context.Background(), "key", nil, nil, fn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected fn to be called")
	}
	if string(val) != "result" {
		t.Errorf("got %q, want %q", string(val), "result")
	}
}

func TestDo_NilCurrent_CallsFn(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), time.Hour, 0)

	called := false
	fn := func(ctx context.Context) ([]byte, error) {
		called = true
		return []byte("new-val"), nil
	}

	// current is nil but entry is not nil; code checks: current == nil || entry == nil
	val, err := d.Do(context.Background(), "key", nil, entry, fn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected fn to be called when current is nil")
	}
	if string(val) != "new-val" {
		t.Errorf("got %q, want %q", string(val), "new-val")
	}
}

func TestDo_NoEarlyRefresh_ReturnsCurrent(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	// Create an entry with a very long TTL so it won't need early refresh.
	entry := eviction.NewEntry("key", []byte("current-value"), 10*time.Minute, 0)
	current := []byte("current-value")

	called := false
	fn := func(ctx context.Context) ([]byte, error) {
		called = true
		return []byte("new-value"), nil
	}

	val, err := d.Do(context.Background(), "key", current, entry, fn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("fn should not be called when no early refresh needed")
	}
	if string(val) != "current-value" {
		t.Errorf("got %q, want current-value", string(val))
	}
}

func TestDo_EarlyRefresh_UpdatesValue(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	// Create an entry with a very short TTL so it will need early refresh.
	// TTL = 1ms, soft TTL = 0.85ms. After sleeping >1ms, ShouldEarlyRefresh will be true.
	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)

	// Wait for soft expiry.
	time.Sleep(2 * time.Millisecond)

	current := []byte("old")

	var refreshedValue []byte
	var mu sync.Mutex
	onRefresh := func(val []byte) {
		mu.Lock()
		refreshedValue = val
		mu.Unlock()
	}

	fn := func(ctx context.Context) ([]byte, error) {
		return []byte("refreshed"), nil
	}

	val, err := d.Do(context.Background(), "key", current, entry, fn, onRefresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The Do method returns the current value immediately and triggers async refresh.
	if string(val) != "old" {
		t.Errorf("got %q, want 'old' (current value returned immediately)", string(val))
	}

	// Wait for the async refresh to complete.
	d.Close() // Close waits for all inflight goroutines.

	mu.Lock()
	rv := refreshedValue
	mu.Unlock()
	if string(rv) != "refreshed" {
		t.Errorf("expected onRefresh to be called with 'refreshed', got %q", string(rv))
	}
}

func TestClose_Idempotent(t *testing.T) {
	d := NewDetector(1.0, nil)
	// Close multiple times should not panic.
	d.Close()
	d.Close()
	d.Close()
}
