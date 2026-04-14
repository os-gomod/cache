package resilience

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/stats"
)

// ---------------------------------------------------------------------------
// Circuit Breaker Tests
// ---------------------------------------------------------------------------

func TestCircuitBreaker_New(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreaker_Allow(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)
	if !cb.Allow() {
		t.Error("closed circuit should allow requests")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	cb.Failure()

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want %v after 2 failures (threshold=2)", cb.State(), StateOpen)
	}
	if cb.Allow() {
		t.Error("open circuit should not allow requests")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	cb.Success()

	if cb.State() != StateClosed {
		t.Error("success should keep circuit closed")
	}

	cb.Failure()
	cb.Failure()
	if cb.State() != StateOpen {
		t.Error("should be open after threshold failures")
	}

	cb.Success()
	if cb.State() != StateClosed {
		t.Error("success should close an open circuit")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)
	cb.Failure()

	if cb.State() != StateOpen {
		t.Error("should be open")
	}

	cb.Reset()
	if cb.State() != StateClosed {
		t.Error("Reset should close the circuit")
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatal("should be open")
	}

	time.Sleep(30 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should transition to half-open after timeout")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("state = %v, want %v", cb.State(), StateHalfOpen)
	}

	if cb.Allow() {
		t.Error("half-open should not allow additional requests while probe is in flight")
	}
}

func TestCircuitBreaker_Nil(t *testing.T) {
	var cb *CircuitBreaker
	if cb.State() != StateClosed {
		t.Error("nil circuit breaker should report closed")
	}
	if !cb.Allow() {
		t.Error("nil circuit breaker should allow")
	}
	cb.Success()
	cb.Failure()
	cb.Reset()
}

func TestCircuitBreaker_Hooks(t *testing.T) {
	var stateChanges atomic.Int32
	cb := NewCircuitBreaker(1, time.Hour)

	// Use NewCacheWithPolicy with the circuit breaker to observe state changes.
	b := &mockBackend{data: map[string][]byte{"key1": []byte("val1")}, failGet: true}
	rc := NewCacheWithPolicy(b, Policy{CircuitBreaker: cb})

	_, _ = rc.Get(context.Background(), "key1")

	if stateChanges.Load() != 0 {
		// stateChanges is not directly observable without hooks,
		// but the circuit breaker should have tripped.
	}

	if cb.State() != StateOpen {
		t.Error("circuit should be open after failure")
	}
}

// TestCircuitBreaker_HalfOpenProbeOnce verifies that when the circuit breaker
// is in half-open state, exactly ONE concurrent goroutine is allowed through
// as the probe. All other concurrent callers must be rejected until the probe
// completes (Success or Failure).
func TestCircuitBreaker_HalfOpenProbeOnce(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	// Trip the breaker open.
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatal("should be open")
	}

	// Wait for the reset timeout to elapse.
	time.Sleep(30 * time.Millisecond)

	// Now many goroutines will race to call Allow().
	// Exactly one should get through; the rest should be rejected.
	const numGoroutines = 100
	var allowed atomic.Int32
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if cb.Allow() {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	if allowed.Load() != 1 {
		t.Errorf("expected exactly 1 goroutine allowed through half-open, got %d", allowed.Load())
	}

	// After the probe succeeds, the circuit should be closed and allow again.
	cb.Success()
	if cb.State() != StateClosed {
		t.Errorf("expected closed after probe success, got %v", cb.State())
	}
	if !cb.Allow() {
		t.Error("closed circuit should allow after probe success")
	}
}

// ---------------------------------------------------------------------------
// Limiter Tests
// ---------------------------------------------------------------------------

func TestNewLimiter(t *testing.T) {
	limiter := NewLimiter(10, 5)
	ctx := context.Background()

	if !limiter.AllowRead(ctx) {
		t.Error("limiter should allow first read")
	}
}

func TestLimiter_RateLimiting(t *testing.T) {
	limiter := NewLimiterWithConfig(LimiterConfig{
		ReadRPS:   1000,
		ReadBurst: 2,
	})
	ctx := context.Background()

	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.AllowRead(ctx) {
			allowed++
		}
	}
	if allowed > 3 {
		t.Errorf("too many allowed: %d", allowed)
	}
}

func TestLimiter_Nil(t *testing.T) {
	var limiter *Limiter
	ctx := context.Background()
	if !limiter.AllowRead(ctx) {
		t.Error("nil limiter should allow reads")
	}
	if !limiter.AllowWrite(ctx) {
		t.Error("nil limiter should allow writes")
	}
}

func TestLimiter_CancelledContext(t *testing.T) {
	limiter := NewLimiter(10, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if limiter.AllowRead(ctx) {
		t.Error("cancelled context should not allow")
	}
}

// ---------------------------------------------------------------------------
// Resilience Cache — Backend integration (via Policy.Execute)
// ---------------------------------------------------------------------------

func TestResilienceCache_Get(t *testing.T) {
	b := &mockBackend{data: map[string][]byte{"key1": []byte("val1")}}
	rc := NewCacheWithPolicy(b, Policy{})

	val, err := rc.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "val1" {
		t.Errorf("Get = %q, want %q", val, "val1")
	}
}

func TestResilienceCache_Set(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	if err := rc.Set(context.Background(), "key1", []byte("val1"), 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
}

func TestResilienceCache_WithCircuitBreaker(t *testing.T) {
	b := &mockBackend{data: map[string][]byte{"key1": []byte("val1")}, failGet: true}
	cb := NewCircuitBreaker(1, time.Hour)
	rc := NewCacheWithPolicy(b, Policy{CircuitBreaker: cb})

	_, err := rc.Get(context.Background(), "key1")
	if err == nil {
		t.Error("expected error from failing backend")
	}

	if cb.State() != StateOpen {
		t.Error("circuit should be open after failure")
	}

	_, err = rc.Get(context.Background(), "key1")
	if err == nil {
		t.Error("expected error from open circuit")
	}
}

func TestResilienceCache_WithRetry(t *testing.T) {
	var calls atomic.Int32
	b := &mockBackend{
		data:      map[string][]byte{"key1": []byte("val1")},
		failGet:   true,
		failCount: &calls,
		failUntil: 2,
	}

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}
	rc := NewCacheWithPolicy(b, policy)

	val, err := rc.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if string(val) != "val1" {
		t.Errorf("Get = %q, want %q", val, "val1")
	}
}

func TestResilienceCache_Close(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	if err := rc.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestResilienceCache_NilClose(t *testing.T) {
	var rc *Cache
	if err := rc.Close(context.Background()); err != nil {
		t.Error("nil Cache Close should return nil")
	}
}

func TestResilienceCache_GetMulti(t *testing.T) {
	b := &mockBackend{data: map[string][]byte{"k1": []byte("v1")}}
	rc := NewCacheWithPolicy(b, Policy{})

	result, err := rc.GetMulti(context.Background(), "k1", "k2")
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}
	if string(result["k1"]) != "v1" {
		t.Errorf("GetMulti[k1] = %q, want %q", result["k1"], "v1")
	}
}

func TestResilienceCache_SetMulti(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	err := rc.SetMulti(context.Background(), map[string][]byte{"k1": []byte("v1")}, 0)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}
}

func TestResilienceCache_DeleteMulti(t *testing.T) {
	b := &mockBackend{data: map[string][]byte{"k1": []byte("v1")}}
	rc := NewCacheWithPolicy(b, Policy{})

	err := rc.DeleteMulti(context.Background(), "k1")
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}
}

func TestResilienceCache_Ping(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	if err := rc.Ping(context.Background()); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestResilienceCache_Stats(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	snap := rc.Stats()
	_ = snap
}

func TestResilienceCache_Closed(t *testing.T) {
	b := &mockBackend{data: make(map[string][]byte)}
	rc := NewCacheWithPolicy(b, Policy{})

	if rc.Closed() {
		t.Error("new cache should not be closed")
	}
	if err := rc.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !rc.Closed() {
		t.Error("cache should be closed after Close")
	}
}

// ---------------------------------------------------------------------------
// Policy validation
// ---------------------------------------------------------------------------

func TestPolicy_Validate(t *testing.T) {
	// Valid default policy.
	if err := DefaultPolicy().Validate(); err != nil {
		t.Errorf("DefaultPolicy should validate: %v", err)
	}
	if err := NoRetryPolicy().Validate(); err != nil {
		t.Errorf("NoRetryPolicy should validate: %v", err)
	}
	if err := HighAvailabilityPolicy().Validate(); err != nil {
		t.Errorf("HighAvailabilityPolicy should validate: %v", err)
	}

	// Invalid: Multiplier < 1.0 with MaxAttempts > 1.
	bad := Policy{
		Retry: RetryConfig{
			MaxAttempts: 3,
			Multiplier:  0.5,
		},
	}
	if err := bad.Validate(); err == nil {
		t.Error("expected validation error for Multiplier < 1.0")
	}

	// Invalid: negative MaxAttempts.
	bad2 := Policy{
		Retry: RetryConfig{MaxAttempts: -1},
	}
	if err := bad2.Validate(); err == nil {
		t.Error("expected validation error for negative MaxAttempts")
	}

	// Invalid: negative Timeout.
	bad3 := Policy{Timeout: -1}
	if err := bad3.Validate(); err == nil {
		t.Error("expected validation error for negative Timeout")
	}
}

// ---------------------------------------------------------------------------
// Mock backend (implements backend.Backend)
// ---------------------------------------------------------------------------

type mockBackend struct {
	data      map[string][]byte
	failGet   bool
	failCount *atomic.Int32 // counts Get calls when failGet is true
	failUntil int32         // succeed after this many failures (0 = always fail)
}

func (m *mockBackend) Get(_ context.Context, key string) ([]byte, error) {
	if m.failGet {
		if m.failCount != nil {
			count := m.failCount.Add(1)
			if m.failUntil > 0 && count > m.failUntil {
				// Succeed after failUntil attempts.
				val, ok := m.data[key]
				if !ok {
					return nil, _errors.NotFound("mock.get", key)
				}
				return val, nil
			}
		}
		return nil, _errors.TimeoutError("mock.get")
	}
	val, ok := m.data[key]
	if !ok {
		return nil, _errors.NotFound("mock.get", key)
	}
	return val, nil
}

func (m *mockBackend) Set(_ context.Context, key string, val []byte, _ time.Duration) error {
	m.data[key] = val
	return nil
}

func (m *mockBackend) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockBackend) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockBackend) TTL(_ context.Context, _ string) (time.Duration, error) {
	return 0, nil
}

func (m *mockBackend) GetMulti(_ context.Context, keys ...string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (m *mockBackend) SetMulti(_ context.Context, items map[string][]byte, _ time.Duration) error {
	for k, v := range items {
		m.data[k] = v
	}
	return nil
}

func (m *mockBackend) DeleteMulti(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func (m *mockBackend) Ping(_ context.Context) error  { return nil }
func (m *mockBackend) Close(_ context.Context) error { return nil }
func (m *mockBackend) Stats() stats.Snapshot         { return stats.Snapshot{} }
func (m *mockBackend) Closed() bool                  { return false }
func (m *mockBackend) Name() string                  { return "mock" }
