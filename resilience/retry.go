package resilience

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// Execute runs fn under the given Policy. The execution order is fixed:
//
//  1. If p.Timeout > 0: wrap ctx with context.WithTimeout
//  2. Check rate limiter (read vs write based on op prefix)
//  3. Check circuit breaker Allow()
//  4. Execute fn(ctx) in retry loop with exponential backoff
//
// Execute never leaks goroutines. All sleep intervals respect ctx cancellation.
func (p Policy) Execute(ctx context.Context, op string, fn func(context.Context) error) error {
	// Step 1: optional per-op timeout.
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}

	// Step 2: rate limiter.
	if p.Limiter != nil && !p.allowOp(ctx, op) {
		return _errors.ErrRateLimited
	}

	// Step 3: circuit breaker guard.
	if p.CircuitBreaker != nil && !p.CircuitBreaker.Allow() {
		return _errors.ErrCircuitOpen
	}

	// Step 4: retry loop.
	return p.retryLoop(ctx, fn)
}

// retryLoop executes fn up to MaxAttempts times with exponential backoff
// between retries. This is extracted from Execute to keep cognitive complexity
// under 15.
func (p Policy) retryLoop(ctx context.Context, fn func(context.Context) error) error {
	maxAttempts := p.Retry.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := p.backoff(attempt)
			if !sleep(ctx, delay) {
				return ctx.Err()
			}
			// Re-check circuit breaker after back-off sleep.
			if p.CircuitBreaker != nil && !p.CircuitBreaker.Allow() {
				return _errors.ErrCircuitOpen
			}
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			p.recordSuccess()
			return nil
		}

		if !p.Retry.isRetryable(lastErr) {
			p.recordFailure()
			return lastErr
		}
	}

	// All attempts exhausted.
	p.recordFailure()
	return lastErr
}

// recordSuccess notifies the circuit breaker of a successful operation.
func (p Policy) recordSuccess() {
	if p.CircuitBreaker != nil {
		p.CircuitBreaker.Success()
	}
}

// recordFailure notifies the circuit breaker of a failed operation.
func (p Policy) recordFailure() {
	if p.CircuitBreaker != nil {
		p.CircuitBreaker.Failure()
	}
}

// allowOp checks the rate limiter, choosing the read or write bucket based on
// the operation prefix. Read operations: "cache.get", "cache.exists",
// "cache.ttl", "cache.get_multi", "cache.ping". All others are writes.
func (p Policy) allowOp(ctx context.Context, op string) bool {
	if p.Limiter == nil {
		return true
	}
	if isReadOp(op) {
		return p.Limiter.AllowRead(ctx)
	}
	return p.Limiter.AllowWrite(ctx)
}

// isReadOp returns true for operations that only read data.
func isReadOp(op string) bool {
	switch op {
	case "cache.get", "cache.exists", "cache.ttl",
		"cache.get_multi", "cache.ping":
		return true
	default:
		return false
	}
}

// backoff computes the delay before the given attempt (1-indexed, so attempt 1
// is the first retry). The formula is:
//
//	delay = min(InitialDelay * Multiplier^(attempt-1), MaxDelay)
//
// If Jitter is enabled, ±25% randomness is added.
func (p Policy) backoff(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	delay := p.Retry.InitialDelay
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	mult := math.Pow(p.Retry.Multiplier, float64(attempt-1))
	delay = time.Duration(float64(delay) * mult)
	if p.Retry.MaxDelay > 0 && delay > p.Retry.MaxDelay {
		delay = p.Retry.MaxDelay
	}
	if p.Retry.Jitter {
		jitterRange := float64(delay) * 0.25
		delay = time.Duration(float64(delay) + (rand.Float64()*2-1)*jitterRange)
		if delay < 0 {
			delay = 0
		}
	}
	return delay
}

// sleep waits for the given duration or until ctx is cancelled, whichever
// comes first. Returns true if the full duration elapsed, false if ctx was
// cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
