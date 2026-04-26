package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// RateLimiterConfig holds the configuration for the token bucket rate limiter middleware.
type RateLimiterConfig struct {
	// Rate is the number of tokens added per second.
	Rate float64

	// Burst is the maximum number of tokens that can be accumulated.
	Burst int
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu        sync.Mutex
	tokens    float64
	maxTokens float64
	rate      float64
	lastTime  time.Time
}

func newTokenBucket(cfg RateLimiterConfig) *tokenBucket {
	burst := float64(cfg.Burst)
	if burst <= 0 {
		burst = 1
	}
	rate := cfg.Rate
	if rate <= 0 {
		rate = 1.0
	}
	return &tokenBucket{
		tokens:    burst,
		maxTokens: burst,
		rate:      rate,
		lastTime:  time.Now(),
	}
}

// allow tries to consume one token. Returns true if a token was available.
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// Add tokens based on elapsed time
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiterMiddleware returns a Middleware that limits the rate of cache
// operations using a token bucket algorithm. When no tokens are available,
// the operation is rejected with an error.
func RateLimiterMiddleware(cfg RateLimiterConfig) Middleware {
	tb := newTokenBucket(cfg)

	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			if !tb.allow() {
				return fmt.Errorf("rate limit exceeded for operation %s", op.Name)
			}
			return next(ctx, op)
		}
	}
}
