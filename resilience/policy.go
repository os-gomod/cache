package resilience

import (
	"fmt"
	"time"

	cacheerrors "github.com/os-gomod/cache/errors"
)

// RetryConfig configures the retry behavior for a resilience policy.
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool
	RetryableErr func(error) bool
}

// Validate checks the retry configuration for correctness.
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

func (r RetryConfig) isRetryable(err error) bool {
	if r.RetryableErr != nil {
		return r.RetryableErr(err)
	}
	return cacheerrors.Retryable(err)
}

// Policy composes circuit breaking, rate limiting, retry, and timeout
// into a single resilience policy for protecting cache operations.
type Policy struct {
	CircuitBreaker *CircuitBreaker
	Limiter        *Limiter
	Retry          RetryConfig
	Timeout        time.Duration
}

// Validate checks the policy configuration for correctness.
func (p Policy) Validate() error {
	if err := p.Retry.Validate(); err != nil {
		return err
	}
	if p.Timeout < 0 {
		return fmt.Errorf("resilience: Policy.Timeout (%v) must be >= 0", p.Timeout)
	}
	return nil
}

// DefaultPolicy returns a Policy with retry enabled (3 attempts, exponential backoff, jitter).
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

// NoRetryPolicy returns a Policy with no retry, circuit breaking, or rate limiting.
func NoRetryPolicy() Policy {
	return Policy{}
}

// HighAvailabilityPolicy returns a Policy tuned for high availability with
// circuit breaking and aggressive retry with a 30-second timeout.
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
