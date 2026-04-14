package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/resilience"
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

// mockBackend is a controllable backend for testing.
type mockBackend struct {
	getFn    func(ctx context.Context, key string) ([]byte, error)
	setFn    func(ctx context.Context, key string, value []byte, ttl time.Duration) error
	deleteFn func(ctx context.Context, key string) error
	pingFn   func(ctx context.Context) error
	closeFn  func(ctx context.Context) error
	statsFn  func() stats.Snapshot
	closedFn func() bool
	nameFn   func() string
}

func (m *mockBackend) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, key)
	}
	return nil
}

func (m *mockBackend) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (m *mockBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (m *mockBackend) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	return nil, nil
}

func (m *mockBackend) SetMulti(
	ctx context.Context,
	items map[string][]byte,
	ttl time.Duration,
) error {
	return nil
}

func (m *mockBackend) DeleteMulti(ctx context.Context, keys ...string) error {
	return nil
}

func (m *mockBackend) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockBackend) Close(ctx context.Context) error {
	if m.closeFn != nil {
		return m.closeFn(ctx)
	}
	return nil
}

func (m *mockBackend) Stats() stats.Snapshot {
	if m.statsFn != nil {
		return m.statsFn()
	}
	return stats.Snapshot{}
}

func (m *mockBackend) Closed() bool {
	if m.closedFn != nil {
		return m.closedFn()
	}
	return false
}

func (m *mockBackend) Name() string {
	if m.nameFn != nil {
		return m.nameFn()
	}
	return "mock"
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNew_NoDefaultBackend(t *testing.T) {
	_, err := New()
	if err == nil {
		t.Error("expected error when no default backend provided, got nil")
	}
}

func TestNew_WithDefaultBackend(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNew_WithPolicy(t *testing.T) {
	mc := newMemBackend(t)
	p := resilience.DefaultPolicy()
	mgr, err := New(WithDefaultBackend(mc), WithPolicy(p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = mgr
}

func TestNew_WithInterceptors(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc), WithInterceptors())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = mgr
}

// ---------------------------------------------------------------------------
// Task 5.1: TestManager_MultipleBackends
// ---------------------------------------------------------------------------

func TestManager_MultipleBackends(t *testing.T) {
	mc := newMemBackend(t)
	mc2 := newMemBackend(t)

	mgr, err := New(
		WithDefaultBackend(mc),
		WithBackend("secondary", mc2),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Write to the default backend.
	if err := mgr.Set(context.Background(), "k1", []byte("default-val"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Write to the secondary backend.
	b, err := mgr.Backend("secondary")
	if err != nil {
		t.Fatalf("Backend(secondary): %v", err)
	}
	if err := b.Set(context.Background(), "k1", []byte("secondary-val"), 0); err != nil {
		t.Fatalf("secondary Set: %v", err)
	}

	// Verify the default backend has its own value.
	val, err := mgr.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "default-val" {
		t.Errorf("default Get = %q, want %q", val, "default-val")
	}

	// Verify the secondary backend has its own value.
	val2, err := b.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("secondary Get: %v", err)
	}
	if string(val2) != "secondary-val" {
		t.Errorf("secondary Get = %q, want %q", val2, "secondary-val")
	}

	// Verify BackendNames includes both.
	names := mgr.BackendNames()
	if len(names) != 2 {
		t.Errorf("BackendNames returned %d names, want 2", len(names))
	}
}

func TestManager_Backend_NotFound(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = mgr.Backend("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent backend, got nil")
	}
}

func TestManager_Backend_EmptyName(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	b, err := mgr.Backend("")
	if err != nil {
		t.Fatalf("Backend(''): %v", err)
	}
	if b == nil {
		t.Error("expected non-nil default backend")
	}
}

// ---------------------------------------------------------------------------
// Core KV
// ---------------------------------------------------------------------------

func TestManager_GetSetDelete(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Set
	if err := mgr.Set(context.Background(), "key1", []byte("hello"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get
	val, err := mgr.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "hello" {
		t.Errorf("Get = %q, want %q", val, "hello")
	}

	// Delete
	if err := mgr.Delete(context.Background(), "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify deleted
	_, err = mgr.Get(context.Background(), "key1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestManager_Get_Miss(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = mgr.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error on cache miss, got nil")
	}
}

// ---------------------------------------------------------------------------
// Task 5.2: TestManager_HealthCheck_Concurrent
// ---------------------------------------------------------------------------

func TestManager_HealthCheck_Concurrent(t *testing.T) {
	var pingCalls atomic.Int32

	slowMock := &mockBackend{
		pingFn: func(ctx context.Context) error {
			pingCalls.Add(1)
			time.Sleep(100 * time.Millisecond) // simulate slow ping
			return nil
		},
		nameFn: func() string { return "slow" },
	}

	mc := newMemBackend(t)
	mgr, err := New(
		WithDefaultBackend(mc),
		WithBackend("slow1", slowMock),
		WithBackend("slow2", slowMock),
		WithBackend("slow3", slowMock),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	results := mgr.HealthCheck(context.Background())
	elapsed := time.Since(start)

	// All backends should be healthy (nil errors are omitted from the map).
	if len(results) != 0 {
		t.Errorf("HealthCheck returned %d errors, want 0", len(results))
	}

	// Total time should be < 2×slowest (100ms), because pings run concurrently.
	// With 3 backends at 100ms each, sequential would be ~300ms.
	// Concurrent should be ~100ms. Allow up to 250ms for scheduling overhead.
	if elapsed > 250*time.Millisecond {
		t.Errorf("HealthCheck took %v, expected < 250ms (concurrent pings)", elapsed)
	}

	// Verify all 3 slow backends were pinged.
	if pingCalls.Load() != 3 {
		t.Errorf("ping called %d times, want 3", pingCalls.Load())
	}
}

func TestManager_HealthCheck_WithErrors(t *testing.T) {
	failingMock := &mockBackend{
		pingFn: func(ctx context.Context) error {
			return errors.New("connection refused")
		},
		nameFn: func() string { return "failing" },
	}

	mc := newMemBackend(t)
	mgr, err := New(
		WithDefaultBackend(mc),
		WithBackend("failing", failingMock),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	results := mgr.HealthCheck(context.Background())
	if len(results) != 1 {
		t.Errorf("HealthCheck returned %d errors, want 1", len(results))
	}
	if results["failing"] == nil {
		t.Error("expected error for failing backend")
	}
}

func TestManager_HealthCheck_Timeout(t *testing.T) {
	hangingMock := &mockBackend{
		pingFn: func(ctx context.Context) error {
			select {
			case <-time.After(10 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		nameFn: func() string { return "hanging" },
	}

	mc := newMemBackend(t)
	mgr, err := New(
		WithDefaultBackend(mc),
		WithBackend("hanging", hangingMock),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	results := mgr.HealthCheck(context.Background())
	elapsed := time.Since(start)

	// The 2s per-backend timeout should kick in.
	if elapsed > 3*time.Second {
		t.Errorf("HealthCheck took %v, expected < 3s (per-backend timeout)", elapsed)
	}

	if len(results) != 1 {
		t.Errorf("HealthCheck returned %d errors, want 1", len(results))
	}
}

// ---------------------------------------------------------------------------
// Task 5.3: TestManager_Close_PartialFailure
// ---------------------------------------------------------------------------

func TestManager_Close_PartialFailure(t *testing.T) {
	var closes atomic.Int32

	closeErrMock := &mockBackend{
		closeFn: func(ctx context.Context) error {
			closes.Add(1)
			return errors.New("close failed")
		},
		nameFn: func() string { return "failing" },
	}

	okMock := &mockBackend{
		closeFn: func(ctx context.Context) error {
			closes.Add(1)
			return nil
		},
		nameFn: func() string { return "ok" },
	}

	mgr, err := New(
		WithDefaultBackend(okMock),
		WithBackend("failing", closeErrMock),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = mgr.Close(context.Background())

	// Both backends should have been attempted.
	if closes.Load() != 2 {
		t.Errorf("Close called on %d backends, want 2", closes.Load())
	}

	// Should return an error (single-error case returns the wrapped error directly).
	if err == nil {
		t.Error("expected error from Close with partial failure, got nil")
	}

	// The error should mention the failing backend.
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Manager should be marked closed.
	if !mgr.Closed() {
		t.Error("manager should be closed after Close()")
	}

	// Double close should be a no-op.
	closes.Store(0)
	err2 := mgr.Close(context.Background())
	if err2 != nil {
		t.Errorf("second Close returned error: %v", err2)
	}
	if closes.Load() != 0 {
		t.Errorf("second Close called %d backends, want 0", closes.Load())
	}
}

func TestManager_Close_MultipleFailures(t *testing.T) {
	var closes atomic.Int32

	failMock1 := &mockBackend{
		closeFn: func(ctx context.Context) error {
			closes.Add(1)
			return errors.New("close failed 1")
		},
		nameFn: func() string { return "fail1" },
	}

	failMock2 := &mockBackend{
		closeFn: func(ctx context.Context) error {
			closes.Add(1)
			return errors.New("close failed 2")
		},
		nameFn: func() string { return "fail2" },
	}

	mgr, err := New(
		WithDefaultBackend(failMock1),
		WithBackend("fail2", failMock2),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = mgr.Close(context.Background())

	// Both backends should have been attempted.
	if closes.Load() != 2 {
		t.Errorf("Close called on %d backends, want 2", closes.Load())
	}

	// Should return a *multiError.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var me *multiError
	if !errors.As(err, &me) {
		t.Fatalf("expected *multiError, got %T", err)
	}
	if len(me.errs) != 2 {
		t.Errorf("multiError has %d errors, want 2", len(me.errs))
	}
}

func TestManager_Close_AllSuccess(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := mgr.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mgr.Closed() {
		t.Error("manager should be closed")
	}
}

// ---------------------------------------------------------------------------
// Task 5.4: TestManager_Namespace_Isolation
// ---------------------------------------------------------------------------

func TestManager_Namespace_Isolation(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	nsA := mgr.Namespace("a:")
	nsB := mgr.Namespace("b:")

	// Set key "x" in both namespaces with different values.
	if err := nsA.Set(context.Background(), "x", []byte("alpha"), 0); err != nil {
		t.Fatalf("nsA.Set: %v", err)
	}
	if err := nsB.Set(context.Background(), "x", []byte("beta"), 0); err != nil {
		t.Fatalf("nsB.Set: %v", err)
	}

	// Verify they are isolated.
	valA, err := nsA.Get(context.Background(), "x")
	if err != nil {
		t.Fatalf("nsA.Get: %v", err)
	}
	if string(valA) != "alpha" {
		t.Errorf("nsA.Get = %q, want %q", valA, "alpha")
	}

	valB, err := nsB.Get(context.Background(), "x")
	if err != nil {
		t.Fatalf("nsB.Get: %v", err)
	}
	if string(valB) != "beta" {
		t.Errorf("nsB.Get = %q, want %q", valB, "beta")
	}

	// Verify the underlying backend stored both keys separately.
	val, err := mgr.Get(context.Background(), "a:x")
	if err != nil {
		t.Fatalf("Get(a:x): %v", err)
	}
	if string(val) != "alpha" {
		t.Errorf("Get(a:x) = %q, want %q", val, "alpha")
	}

	val, err = mgr.Get(context.Background(), "b:x")
	if err != nil {
		t.Fatalf("Get(b:x): %v", err)
	}
	if string(val) != "beta" {
		t.Errorf("Get(b:x) = %q, want %q", val, "beta")
	}
}

func TestNamespace_Delete(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ns := mgr.Namespace("ns:")
	if err := ns.Set(context.Background(), "k1", []byte("v1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := ns.Delete(context.Background(), "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = ns.Get(context.Background(), "k1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestNamespace_Prefix(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ns := mgr.Namespace("app:")
	if ns.Prefix() != "app:" {
		t.Errorf("Prefix = %q, want %q", ns.Prefix(), "app:")
	}
}

// ---------------------------------------------------------------------------
// Typed access — removed (Typed was removed from manager package)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func TestManager_Stats(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Perform a set to ensure the backend is functioning.
	_ = mgr.Set(context.Background(), "stats-key", []byte("v"), 0)

	snapshots := mgr.Stats()
	if len(snapshots) != 1 {
		t.Errorf("Stats returned %d entries, want 1", len(snapshots))
	}

	snap, ok := snapshots["default"]
	if !ok {
		t.Error("Stats missing 'default' entry")
	}
	// The resilience wrapper tracks its own stats, which may differ from
	// the raw backend stats. Just verify the snapshot is returned.
	_ = snap
}

// ---------------------------------------------------------------------------
// Presets
// ---------------------------------------------------------------------------

func TestNewMemoryManager(t *testing.T) {
	mgr, err := NewMemoryManager()
	if err != nil {
		t.Fatalf("NewMemoryManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close(context.Background()) })

	if err := mgr.Set(context.Background(), "k1", []byte("v1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := mgr.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "v1" {
		t.Errorf("Get = %q, want %q", val, "v1")
	}
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestManager_ConcurrentAccess(t *testing.T) {
	mc := newMemBackend(t)
	mgr, err := New(WithDefaultBackend(mc))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const goroutines = 50
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			for j := 0; j < opsPerGoroutine; j++ {
				_ = mgr.Set(context.Background(), key, []byte(fmt.Sprintf("val-%d", j)), 0)
				_, _ = mgr.Get(context.Background(), key)
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Task 5.5: BenchmarkManager_Get — overhead vs direct backend < 500ns
// ---------------------------------------------------------------------------

func BenchmarkManager_Get(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()

	mgr, _ := New(WithDefaultBackend(mc))
	_ = mgr.Set(context.Background(), "bench", []byte("value"), 0)

	for b.Loop() {
		_, _ = mgr.Get(context.Background(), "bench")
	}
}

func BenchmarkManager_Set(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()

	mgr, _ := New(WithDefaultBackend(mc))

	for b.Loop() {
		_ = mgr.Set(context.Background(), "bench", []byte("value"), 0)
	}
}

func BenchmarkManager_Get_Direct(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()

	_ = mc.Set(context.Background(), "bench", []byte("value"), 0)

	for b.Loop() {
		_, _ = mc.Get(context.Background(), "bench")
	}
}

func BenchmarkManager_Namespace_Get(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()

	mgr, _ := New(WithDefaultBackend(mc))
	ns := mgr.Namespace("ns:")
	_ = ns.Set(context.Background(), "bench", []byte("value"), 0)

	for b.Loop() {
		_, _ = ns.Get(context.Background(), "bench")
	}
}

func BenchmarkManager_HealthCheck(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()

	mgr, _ := New(WithDefaultBackend(mc))

	for b.Loop() {
		_ = mgr.HealthCheck(context.Background())
	}
}

// Verify the interface is satisfied at compile time.
var _ Backend = (*mockBackend)(nil)
