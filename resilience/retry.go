package resilience

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
)

// Execute runs fn under the resilience policy, applying rate limiting,
// circuit breaking, timeout, and retry logic as configured.
func (p Policy) Execute(ctx context.Context, op string, fn func(context.Context) error) error {
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	if p.Limiter != nil && !p.allowOp(ctx, op) {
		return cacheerrors.ErrRateLimited
	}
	if p.CircuitBreaker != nil && !p.CircuitBreaker.Allow() {
		return cacheerrors.ErrCircuitOpen
	}
	return p.retryLoop(ctx, fn)
}

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
			if p.CircuitBreaker != nil && !p.CircuitBreaker.Allow() {
				return cacheerrors.ErrCircuitOpen
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
	p.recordFailure()
	return lastErr
}

func (p Policy) recordSuccess() {
	if p.CircuitBreaker != nil {
		p.CircuitBreaker.Success()
	}
}

func (p Policy) recordFailure() {
	if p.CircuitBreaker != nil {
		p.CircuitBreaker.Failure()
	}
}

func (p Policy) allowOp(ctx context.Context, op string) bool {
	if p.Limiter == nil {
		return true
	}
	if isReadOp(op) {
		return p.Limiter.AllowRead(ctx)
	}
	return p.Limiter.AllowWrite(ctx)
}

func isReadOp(op string) bool {
	switch op {
	case "cache.get", "cache.exists", "cache.ttl",
		"cache.get_multi", "cache.ping":
		return true
	default:
		return false
	}
}

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
