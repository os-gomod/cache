package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

func TestRetryConfig_Validate_Valid(t *testing.T) {
	rc := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
	if err := rc.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRetryConfig_Validate_NegativeMaxAttempts(t *testing.T) {
	rc := RetryConfig{
		MaxAttempts: -1,
	}
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for negative MaxAttempts")
	}
}

func TestRetryConfig_Validate_MultiplierTooLow(t *testing.T) {
	rc := RetryConfig{
		MaxAttempts: 3,
		Multiplier:  0.5,
	}
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for multiplier < 1.0 when MaxAttempts > 1")
	}
}

func TestPolicy_Execute_Success(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	}
	called := false
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !called {
		t.Fatal("expected fn to be called")
	}
}

func TestPolicy_Execute_RetrySucceeds(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErr: func(err error) bool { return true },
		},
	}
	attempts := 0
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestPolicy_Execute_AllFail(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErr: func(err error) bool { return true },
		},
	}
	attempts := 0
	wantErr := errors.New("persistent error")
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		attempts++
		return wantErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestPolicy_Execute_NonRetryableError(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErr: func(err error) bool { return false },
		},
	}
	attempts := 0
	wantErr := errors.New("not retryable")
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		attempts++
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestPolicy_Execute_Timeout(t *testing.T) {
	p := Policy{
		Retry: RetryConfig{
			MaxAttempts:  10,
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   1.0,
			RetryableErr: func(err error) bool { return true },
		},
		Timeout: 20 * time.Millisecond,
	}
	attempts := 0
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		attempts++
		return errors.New("always fails")
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// Should have been cut short by timeout, not reaching all 10 attempts.
	if attempts >= 10 {
		t.Fatalf("expected fewer attempts due to timeout, got %d", attempts)
	}
}

func TestPolicy_Execute_CircuitBreakerOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 0) // threshold=1, resetTimeout=0 (never auto-reset)
	// Trip the breaker
	cb.Failure()

	p := Policy{
		CircuitBreaker: cb,
		Retry: RetryConfig{
			MaxAttempts: 1,
		},
	}
	called := false
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		called = true
		return nil
	})
	if !_errors.IsCircuitOpen(err) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
	if called {
		t.Fatal("fn should not be called when circuit is open")
	}
}

func TestPolicy_Execute_RateLimited(t *testing.T) {
	limiter := NewLimiterWithConfig(LimiterConfig{
		ReadRPS:   0, // zero rate means no limiting by tokenBucket
		ReadBurst: 0,
	})
	// Actually, rate=0 allows all through tokenBucket. We need to test the Limiter path.
	// Let's use a positive rate with burst=0 so it always blocks.
	limiter = NewLimiterWithConfig(LimiterConfig{
		ReadRPS:   10,
		ReadBurst: 0,
	})

	p := Policy{
		Limiter: limiter,
		Retry: RetryConfig{
			MaxAttempts: 1,
		},
	}
	called := false
	err := p.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		called = true
		return nil
	})
	if !_errors.IsRateLimited(err) {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}
	if called {
		t.Fatal("fn should not be called when rate limited")
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", p.Retry.MaxAttempts)
	}
	if p.Retry.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", p.Retry.Multiplier)
	}
	if p.Retry.Jitter != true {
		t.Error("expected Jitter=true")
	}
	if p.Retry.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay=100ms, got %v", p.Retry.InitialDelay)
	}
	if p.Retry.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay=5s, got %v", p.Retry.MaxDelay)
	}
}

func TestNoRetryPolicy(t *testing.T) {
	p := NoRetryPolicy()
	if p.Retry.MaxAttempts != 0 {
		t.Errorf("expected MaxAttempts=0, got %d", p.Retry.MaxAttempts)
	}
	if p.CircuitBreaker != nil {
		t.Error("expected no circuit breaker")
	}
	if p.Limiter != nil {
		t.Error("expected no limiter")
	}
}

func TestHighAvailabilityPolicy(t *testing.T) {
	p := HighAvailabilityPolicy()
	if p.CircuitBreaker == nil {
		t.Fatal("expected circuit breaker")
	}
	if p.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", p.Retry.MaxAttempts)
	}
	if p.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", p.Timeout)
	}
}
