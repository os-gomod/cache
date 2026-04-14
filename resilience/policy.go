package resilience

import (
	"fmt"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// RetryConfig controls the retry behavior of a Policy. When MaxAttempts is
// zero, no retries are performed. Otherwise the executor will call the
// operation up to MaxAttempts times with exponential backoff between attempts.
type RetryConfig struct {
	MaxAttempts  int              // 0 = no retry; 1 = single attempt (no retry); N = up to N total attempts
	InitialDelay time.Duration    // delay before the first retry
	MaxDelay     time.Duration    // cap on the backoff delay
	Multiplier   float64          // exponential backoff multiplier (≥ 1.0)
	Jitter       bool             // add ±25% random jitter to each delay
	RetryableErr func(error) bool // nil = use errors.Retryable()
}

// Validate returns an error if the RetryConfig contains inconsistent values.
func (r RetryConfig) Validate() error {
	if r.MaxAttempts < 0 {
		return fmt.Errorf("resilience: RetryConfig.MaxAttempts (%d) must be >= 0", r.MaxAttempts)
	}
	if r.MaxAttempts > 1 {
		if r.Multiplier < 1.0 {
			return fmt.Errorf(
				"resilience: RetryConfig.Multiplier (%.2f) must be >= 1.0 when MaxAttempts > 1",
				r.Multiplier,
			)
		}
		if r.InitialDelay < 0 {
			return fmt.Errorf(
				"resilience: RetryConfig.InitialDelay (%v) must be >= 0",
				r.InitialDelay,
			)
		}
		if r.MaxDelay < 0 {
			return fmt.Errorf("resilience: RetryConfig.MaxDelay (%v) must be >= 0", r.MaxDelay)
		}
	}
	return nil
}

// isRetryable determines whether an error should trigger a retry. It uses the
// custom RetryableErr function if provided, otherwise falls back to the
// package-level errors.Retryable classifier.
func (r RetryConfig) isRetryable(err error) bool {
	if r.RetryableErr != nil {
		return r.RetryableErr(err)
	}
	return _errors.Retryable(err)
}

// Policy is the unified resilience configuration. A nil CircuitBreaker or nil
// Limiter means that component is disabled. A zero Timeout means no per-op
// timeout is applied (the caller's context deadline still applies).
type Policy struct {
	CircuitBreaker *CircuitBreaker // nil = disabled
	Limiter        *Limiter        // nil = disabled
	Retry          RetryConfig
	Timeout        time.Duration // 0 = no per-op timeout
}

// Validate checks the entire Policy for consistency.
func (p Policy) Validate() error {
	if err := p.Retry.Validate(); err != nil {
		return err
	}
	if p.Timeout < 0 {
		return fmt.Errorf("resilience: Policy.Timeout (%v) must be >= 0", p.Timeout)
	}
	return nil
}

// DefaultPolicy returns a sensible production policy: 3 retry attempts with
// exponential backoff (starting at 100 ms, multiplier 2.0) and jitter enabled.
// No circuit breaker or rate limiter is configured by default.
func DefaultPolicy() Policy {
	return Policy{
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     5 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		},
	}
}

// NoRetryPolicy returns a policy with no retry, no circuit breaker, and no
// rate limiter. Operations are executed exactly once.
func NoRetryPolicy() Policy {
	return Policy{}
}

// HighAvailabilityPolicy returns a policy tuned for high availability:
// circuit breaker with threshold 5, 3 retries with jitter, and a 30-second
// per-op timeout.
func HighAvailabilityPolicy() Policy {
	return Policy{
		CircuitBreaker: NewCircuitBreaker(5, 30*time.Second),
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 200 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		},
		Timeout: 30 * time.Second,
	}
}
