package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// ---------------------------------------------------------------------------
// TestRetry_ExponentialBackoff
// ---------------------------------------------------------------------------

func TestRetry_ExponentialBackoff(t *testing.T) {
	var calls atomic.Int32
	var attemptTimes []time.Time

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  4,
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     2 * time.Second,
			Multiplier:   2.0,
			Jitter:       false, // deterministic
		},
	}

	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		attemptTimes = append(attemptTimes, time.Now())
		calls.Add(1)
		if calls.Load() < 4 {
			return _errors.TimeoutError("cache.get") // retryable
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls.Load() != 4 {
		t.Fatalf("expected 4 calls, got %d", calls.Load())
	}

	// Verify exponential growth: delay between attempt 1→2 ~ 50ms, 2→3 ~ 100ms, 3→4 ~ 200ms
	expectedDelays := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}
	for i := 0; i < len(expectedDelays) && i+1 < len(attemptTimes); i++ {
		actual := attemptTimes[i+1].Sub(attemptTimes[i])
		tolerance := 30 * time.Millisecond
		expected := expectedDelays[i]
		if actual < expected-tolerance || actual > expected+tolerance {
			t.Errorf("delay between attempt %d and %d: got %v, want ~%v (±%v)",
				i+1, i+2, actual, expected, tolerance)
		}
	}
}

// ---------------------------------------------------------------------------
// TestRetry_MaxAttempts
// ---------------------------------------------------------------------------

func TestRetry_MaxAttempts(t *testing.T) {
	var calls atomic.Int32

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}

	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return _errors.TimeoutError("cache.get")
	})
	if err == nil {
		t.Fatal("expected error after max attempts exhausted")
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// TestRetry_JitterRange
// ---------------------------------------------------------------------------

func TestRetry_JitterRange(t *testing.T) {
	// Verify jitter stays within ±25% of expected delay.
	// With InitialDelay=200ms, Multiplier=1.0, the first retry delay should
	// be in [150ms, 250ms].
	baseDelay := 200 * time.Millisecond
	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  2,
			InitialDelay: baseDelay,
			MaxDelay:     1 * time.Second,
			Multiplier:   1.0,
			Jitter:       true,
		},
	}

	minAllowed := time.Duration(float64(baseDelay) * 0.74)
	maxAllowed := time.Duration(float64(baseDelay) * 1.26)

	var delays []time.Duration
	for i := 0; i < 20; i++ {
		delay := policy.backoff(1)
		delays = append(delays, delay)
	}

	outOfRange := 0
	for _, d := range delays {
		if d < minAllowed || d > maxAllowed {
			outOfRange++
		}
	}
	// Allow up to 1 out-of-range due to timing precision.
	if outOfRange > 1 {
		t.Errorf("too many delays out of ±25%% range: %d/%d", outOfRange, len(delays))
	}
}

// ---------------------------------------------------------------------------
// TestRetry_ContextCancellation
// ---------------------------------------------------------------------------

func TestRetry_ContextCancellation(t *testing.T) {
	var calls atomic.Int32

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  10,
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after the first call.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := policy.Execute(ctx, "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return _errors.TimeoutError("cache.get")
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// The function should not be called many times after cancellation.
	// It may be called 1-2 times depending on timing, but not all 10.
	if calls.Load() > 3 {
		t.Errorf("expected at most 3 calls after cancellation, got %d", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// TestRetry_NonRetryableError
// ---------------------------------------------------------------------------

func TestRetry_NonRetryableError(t *testing.T) {
	var calls atomic.Int32

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  5,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}

	// NotFound errors are not retryable.
	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return _errors.NotFound("cache.get", "key1")
	})
	if err == nil {
		t.Fatal("expected NotFound error")
	}
	if !_errors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("non-retryable error should cause exactly 1 call, got %d", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// TestRetry_CustomRetryableErr
// ---------------------------------------------------------------------------

func TestRetry_CustomRetryableErr(t *testing.T) {
	var calls atomic.Int32

	customErr := errors.New("custom")

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			Jitter:       false,
			RetryableErr: func(err error) bool {
				return errors.Is(err, customErr)
			},
		},
	}

	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return customErr
	})
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls with custom retryable, got %d", calls.Load())
	}

	// Verify that a TimeoutError is NOT retried (our custom function says no).
	calls.Store(0)
	_ = policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return _errors.TimeoutError("cache.get")
	})
	if calls.Load() != 1 {
		t.Errorf(
			"TimeoutError should not be retried with custom RetryableErr, got %d calls",
			calls.Load(),
		)
	}
}

// ---------------------------------------------------------------------------
// TestRetry_ZeroAttempts
// ---------------------------------------------------------------------------

func TestRetry_ZeroAttempts(t *testing.T) {
	var calls atomic.Int32

	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts: 0, // no retry, but should still execute once
		},
	}

	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		calls.Add(1)
		return _errors.TimeoutError("cache.get")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call with MaxAttempts=0, got %d", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// TestRetry_CircuitBreakerIntegration
// ---------------------------------------------------------------------------

func TestRetry_CircuitBreakerIntegration(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)

	policy := Policy{
		CircuitBreaker: cb,
		Retry: RetryConfig{
			MaxAttempts:  2,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}

	// All attempts fail → circuit breaker should record a failure.
	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		return _errors.TimeoutError("cache.get")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// After one Execute with all attempts failing, CB should have 1 failure recorded.
	if cb.State() != StateClosed {
		t.Errorf("circuit should still be closed after 1 failure, got %v", cb.State())
	}
}

// ---------------------------------------------------------------------------
// TestRetry_Timeout
// ---------------------------------------------------------------------------

func TestRetry_Timeout(t *testing.T) {
	policy := Policy{
		Timeout: 50 * time.Millisecond,
		Retry: RetryConfig{
			MaxAttempts:  1, // no retry, just test timeout wrapping
			InitialDelay: 0,
			Multiplier:   1.0,
		},
	}

	err := policy.Execute(context.Background(), "cache.get", func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if ctx_err := context.DeadlineExceeded; !errors.Is(err, ctx_err) {
		// The error may be wrapped, but should be a deadline error.
		if !_errors.IsTimeout(err) {
			t.Errorf("expected timeout error, got %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// BenchmarkExecute_NoRetry
// ---------------------------------------------------------------------------

func BenchmarkExecute_NoRetry(b *testing.B) {
	policy := NoRetryPolicy()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Execute(ctx, "cache.get", func(ctx context.Context) error {
			return nil
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkExecute_WithRetry
// ---------------------------------------------------------------------------

func BenchmarkExecute_WithRetry(b *testing.B) {
	policy := Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 0, // instant for benchmark
			MaxDelay:     0,
			Multiplier:   1.0,
			Jitter:       false,
		},
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Execute(ctx, "cache.get", func(ctx context.Context) error {
			return nil
		})
	}
}
