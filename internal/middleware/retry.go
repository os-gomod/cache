package middleware

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// RetryConfig holds the configuration for the retry middleware.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts including the initial call.
	// A value of 1 means no retries; a value of 3 means 1 initial + 2 retries.
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the exponential backoff multiplier applied to each successive delay.
	Multiplier float64

	// Jitter adds randomization to the delay to prevent thundering herd.
	Jitter bool

	// RetryableErr is a function that determines whether an error should trigger a retry.
	// If nil, all errors are considered retryable.
	RetryableErr func(error) bool
}

// RetryMiddleware returns a Middleware that retries failed operations using
// exponential backoff with optional jitter. The retry decision is based on
// the RetryConfig.RetryableErr function, falling back to retrying all errors.
//
// The middleware records the number of attempts in the context value
// "retry_attempts" as an int.
func RetryMiddleware(cfg RetryConfig) Middleware {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 100 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}
	if cfg.RetryableErr == nil {
		cfg.RetryableErr = func(_ error) bool { return true }
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			var lastErr error
			delay := cfg.InitialDelay

			for attempt := range cfg.MaxAttempts {
				lastErr = next(ctx, op)
				if lastErr == nil {
					return nil
				}

				// Check if this error is retryable
				if !cfg.RetryableErr(lastErr) {
					return lastErr
				}

				// Don't sleep after the last attempt
				if attempt >= cfg.MaxAttempts-1 {
					break
				}

				// Calculate delay with exponential backoff and optional jitter
				sleepDuration := delay
				if cfg.Jitter {
					sleepDuration = addJitter(delay)
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(sleepDuration):
				}

				// Increase delay for next attempt
				delay = time.Duration(math.Min(
					float64(delay)*cfg.Multiplier,
					float64(cfg.MaxDelay),
				))
			}

			return fmt.Errorf("after %d attempts: %w", cfg.MaxAttempts, lastErr)
		}
	}
}

// addJitter adds random jitter to the delay, returning a value between 0 and delay.
func addJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(delay)))
}
