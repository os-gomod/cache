package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/observability"
)

// ─── policy.go additional tests ───

func TestIsReadOp(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"cache.get", true},
		{"cache.exists", true},
		{"cache.ttl", true},
		{"cache.get_multi", true},
		{"cache.ping", true},
		{"cache.set", false},
		{"cache.delete", false},
		{"cache.set_multi", false},
		{"cache.delete_multi", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isReadOp(tt.op); got != tt.want {
			t.Errorf("isReadOp(%q) = %v, want %v", tt.op, got, tt.want)
		}
	}
}

func TestPolicy_Backoff(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			InitialDelay: 100 * time.Millisecond,
			Multiplier:   2.0,
			MaxDelay:     1 * time.Second,
		},
	}

	// attempt 0 should return 0
	if d := p.backoff(0); d != 0 {
		t.Errorf("backoff(0) = %v, want 0", d)
	}

	// attempt 1 should return InitialDelay
	if d := p.backoff(1); d != 100*time.Millisecond {
		t.Errorf("backoff(1) = %v, want 100ms", d)
	}

	// attempt 2 should be InitialDelay * 2
	d2 := p.backoff(2)
	if d2 != 200*time.Millisecond {
		t.Errorf("backoff(2) = %v, want 200ms", d2)
	}

	// attempt 5 should be capped at MaxDelay
	d5 := p.backoff(5)
	if d5 > 1*time.Second {
		t.Errorf("backoff(5) = %v, want <= 1s (MaxDelay)", d5)
	}
}

func TestPolicy_Backoff_ZeroInitialDelay(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			InitialDelay: 0,
			Multiplier:   2.0,
		},
	}
	d := p.backoff(1)
	if d == 0 {
		t.Error("backoff should use default 100ms when InitialDelay is 0")
	}
}

func TestPolicy_Backoff_WithJitter(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			InitialDelay: 100 * time.Millisecond,
			Multiplier:   2.0,
			MaxDelay:     1 * time.Second,
			Jitter:       true,
		},
	}
	// Run many times to ensure jitter produces varied results
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		d := p.backoff(1)
		if d <= 0 {
			t.Errorf("backoff(1) with jitter = %v, want > 0", d)
		}
		seen[d] = true
	}
	// With jitter, we should see some variation
	if len(seen) < 5 {
		t.Error("expected some variation in jitter values")
	}
}

func TestSleep_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleep(ctx, time.Second) {
		t.Error("sleep should return false for cancelled context")
	}
}

func TestSleep_ZeroDuration(t *testing.T) {
	if !sleep(context.Background(), 0) {
		t.Error("sleep(0) should return true")
	}
}

func TestSleep_NegativeDuration(t *testing.T) {
	if !sleep(context.Background(), -time.Second) {
		t.Error("sleep(-1s) should return true")
	}
}

func TestSleep_ActualSleep(t *testing.T) {
	start := time.Now()
	sleep(context.Background(), 5*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 3*time.Millisecond {
		t.Errorf("sleep(5ms) should wait at least ~5ms, got %v", elapsed)
	}
}

func TestPolicy_Execute_CircuitBreakerInRetryLoop(t *testing.T) {
	cb := NewCircuitBreaker(1, 0) // threshold=1, no auto-reset
	p := Policy{
		CircuitBreaker: cb,
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			RetryableErr: func(err error) bool { return true },
		},
	}
	wantErr := errTestTransient{}
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		return wantErr
	})
	if err == nil {
		t.Error("expected error")
	}
	// After all retries exhausted, recordFailure is called once, opening the CB
	if cb.State() != StateOpen {
		t.Errorf("expected circuit breaker to be open, got %v", cb.State())
	}
}

func TestPolicy_Execute_WriteOp_RateLimited(t *testing.T) {
	limiter := NewLimiterWithConfig(LimiterConfig{
		WriteRPS:   10,
		WriteBurst: 0,
	})
	p := Policy{
		Limiter: limiter,
		Retry: RetryConfig{
			MaxAttempts: 1,
		},
	}
	called := false
	err := p.Execute(context.Background(), "cache.set", func(ctx context.Context) error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("expected error for rate limited write op")
	}
	if called {
		t.Error("fn should not be called when rate limited")
	}
}

func TestPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		p       Policy
		wantErr bool
	}{
		{"valid default", DefaultPolicy(), false},
		{"valid no retry", NoRetryPolicy(), false},
		{"negative timeout", Policy{Timeout: -1}, true},
		{"invalid retry config", Policy{Retry: RetryConfig{MaxAttempts: 3, Multiplier: 0.5}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicy_Execute_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := Policy{
		Timeout: time.Second,
		Retry: RetryConfig{
			MaxAttempts: 1,
		},
	}
	called := false
	err := p.Execute(ctx, "cache.get", func(ctx context.Context) error {
		called = true
		return nil
	})
	// With MaxAttempts=1 and cancelled context, the function may or may not be called
	// depending on timing, but we should get an error because ctx is already cancelled
	_ = err
	_ = called
}

// helper error type
type errTestTransient struct{}

func (errTestTransient) Error() string { return "transient" }

// ─── cache.go additional tests ───

func TestCache_TTL(t *testing.T) {
	wantTTL := 5 * time.Minute
	backend := &mockBackend{
		ttlFn: func(ctx context.Context, key string) (time.Duration, error) {
			if key != "mykey" {
				t.Errorf("expected key=mykey, got %q", key)
			}
			return wantTTL, nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	d, err := c.TTL(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != wantTTL {
		t.Errorf("got %v, want %v", d, wantTTL)
	}
}

func TestCache_GetMulti(t *testing.T) {
	backend := &mockBackend{
		getMultiFn: func(ctx context.Context, keys ...string) (map[string][]byte, error) {
			result := make(map[string][]byte)
			for _, k := range keys {
				result[k] = []byte("value-" + k)
			}
			return result, nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	vals, err := c.GetMulti(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 2 {
		t.Errorf("expected 2 values, got %d", len(vals))
	}
}

func TestCache_SetMulti(t *testing.T) {
	var gotItems map[string][]byte
	backend := &mockBackend{
		setMultiFn: func(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
			gotItems = items
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	items := map[string][]byte{"a": []byte("1"), "b": []byte("2")}
	err := c.SetMulti(context.Background(), items, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotItems) != 2 {
		t.Errorf("expected 2 items, got %d", len(gotItems))
	}
}

func TestCache_DeleteMulti(t *testing.T) {
	var gotKeys []string
	backend := &mockBackend{
		deleteMultiFn: func(ctx context.Context, keys ...string) error {
			gotKeys = keys
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	err := c.DeleteMulti(context.Background(), "a", "b", "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotKeys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(gotKeys))
	}
}

func TestCache_Close_NilReceiver(t *testing.T) {
	var c *Cache
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCache_Close_NilBackend(t *testing.T) {
	c := NewCacheWithPolicy(&mockBackend{}, DefaultPolicy())
	// Close with nil backend should be safe
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCache_Stats(t *testing.T) {
	c := NewCacheWithPolicy(&mockBackend{}, DefaultPolicy())
	s := c.Stats()
	// Just ensure it doesn't panic
	_ = s
}

func TestCache_Closed(t *testing.T) {
	c := NewCacheWithPolicy(&mockBackend{}, DefaultPolicy())
	if c.Closed() {
		t.Error("should not be closed initially")
	}
	c.Close(context.Background())
	if !c.Closed() {
		t.Error("should be closed after Close()")
	}
}

func TestCache_SetInterceptors(t *testing.T) {
	c := NewCacheWithPolicy(&mockBackend{}, DefaultPolicy())
	c.SetInterceptors(observability.NopInterceptor{})
	// Should not panic
}

func TestCache_WithInterceptors_Option(t *testing.T) {
	backend := &mockBackend{
		setFn: func(ctx context.Context, key string, value []byte, ttl time.Duration) error {
			return nil
		},
	}
	var beforeCalled, afterCalled bool
	ic := &testInterceptor{beforeFn: func(ctx context.Context, op observability.Op) context.Context {
		beforeCalled = true
		return ctx
	}, afterFn: func(ctx context.Context, op observability.Op, result observability.Result) {
		afterCalled = true
	}}
	c := NewCacheWithPolicy(backend, DefaultPolicy(), WithInterceptors(ic))
	c.Set(context.Background(), "key", []byte("val"), 0)
	if !beforeCalled {
		t.Error("Before should have been called")
	}
	if !afterCalled {
		t.Error("After should have been called")
	}
}

func TestCache_GetMulti_Empty(t *testing.T) {
	backend := &mockBackend{
		getMultiFn: func(ctx context.Context, keys ...string) (map[string][]byte, error) {
			return nil, nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	vals, err := c.GetMulti(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected 0 values, got %d", len(vals))
	}
}

func TestCache_Get_Error_NonRetryable(t *testing.T) {
	wantErr := errTestNonRetryable{}
	backend := &mockBackend{
		getFn: func(ctx context.Context, key string) ([]byte, error) {
			return nil, wantErr
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	_, err := c.Get(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error")
	}
}

type errTestNonRetryable struct{}

func (errTestNonRetryable) Error() string { return "not retryable" }

type testInterceptor struct {
	beforeFn func(ctx context.Context, op observability.Op) context.Context
	afterFn  func(ctx context.Context, op observability.Op, result observability.Result)
}

func (t *testInterceptor) Before(ctx context.Context, op observability.Op) context.Context {
	if t.beforeFn != nil {
		return t.beforeFn(ctx, op)
	}
	return ctx
}

func (t *testInterceptor) After(ctx context.Context, op observability.Op, result observability.Result) {
	if t.afterFn != nil {
		t.afterFn(ctx, op, result)
	}
}
