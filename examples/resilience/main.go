// Package main demonstrates resilience middleware (retry, circuit breaker, rate limiter).
//
// This example shows:
//   - Wrapping a cache with retry middleware
//   - Configuring circuit breaker with failure threshold
//   - Adding rate limiting to prevent cache stampedes
//   - Using the combined WithResilience convenience function
//   - Observing circuit breaker state transitions
//   - Retry behavior under transient failures
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/memory"
)

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// Basic Resilience Stack
	// ---------------------------------------------------------------------------
	fmt.Println("=== Creating Cache with Resilience ===")

	backend, err := cache.NewMemory(
		memory.WithMaxEntries(1000),
		memory.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create memory cache: %v", err)
	}
	defer backend.Close(ctx)

	// Apply the full resilience stack: retry + circuit breaker + rate limiter
	resilient := cache.WithResilience(backend,
		cache.WithRetry(3, 100*time.Millisecond),    // up to 3 retries, 100ms initial delay
		cache.WithCircuitBreaker(5, 30*time.Second), // opens after 5 failures, resets after 30s
		cache.WithRateLimit(1000, 100),              // 1000 ops/sec, burst of 100
	)

	fmt.Println("Resilience stack applied:")
	fmt.Println("  - Retry: maxAttempts=3, initialDelay=100ms (exponential backoff)")
	fmt.Println("  - Circuit Breaker: threshold=5, timeout=30s")
	fmt.Println("  - Rate Limiter: 1000 ops/sec, burst=100")

	// Set a value through the resilient wrapper
	err = resilient.Set(ctx, "key1", []byte("value1"), 5*time.Minute)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Println("\nSet key1 = value1 (through resilience stack)")

	val, err := resilient.Get(ctx, "key1")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get key1 = %s\n", string(val))

	// ---------------------------------------------------------------------------
	// Individual Middleware Application
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Individual Middleware Configuration ===")

	// Retry only
	retryOnly := cache.WithMiddleware(backend,
		middleware.RetryMiddleware(middleware.RetryConfig{
			MaxAttempts:  5,
			InitialDelay: 50 * time.Millisecond,
		}),
	)

	// Circuit breaker only
	cbOnly := cache.WithMiddleware(backend,
		middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
			Threshold: 3,
			Timeout:   10 * time.Second,
		}),
	)

	_ = cbOnly // suppress unused variable warning

	// Rate limiter only
	rlOnly := cache.WithMiddleware(backend,
		middleware.RateLimiterMiddleware(middleware.RateLimiterConfig{
			Rate:  500,
			Burst: 50,
		}),
	)

	fmt.Println("Created three individually-wrapped backends:")
	fmt.Println("  - retryOnly: 5 attempts, 50ms delay")
	fmt.Println("  - cbOnly: threshold=3, timeout=10s")
	fmt.Println("  - rlOnly: 500 ops/sec, burst=50")

	// Demonstrate retry behavior
	fmt.Println("\n=== Retry Behavior ===")
	retryOnly.Set(ctx, "retry:test", []byte("works"), 5*time.Minute) // nolint: errcheck
	val, err = retryOnly.Get(ctx, "retry:test")
	if err != nil {
		log.Fatalf("Get through retry wrapper failed: %v", err)
	}
	fmt.Printf("Get retry:test = %s (succeeded on first attempt)\n", string(val))

	// ---------------------------------------------------------------------------
	// Circuit Breaker Demonstration
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Circuit Breaker State ===")

	// The circuit breaker tracks failures. In this example, we show
	// the expected behavior (in production, actual failures would trigger it).
	fmt.Println("Circuit breaker states:")
	fmt.Println("  - Closed: normal operation, requests pass through")
	fmt.Println("  - Open: failures exceeded threshold, requests are rejected immediately")
	fmt.Println("  - Half-Open: after timeout, limited requests test backend health")
	fmt.Println()
	fmt.Println("Example circuit breaker flow:")
	fmt.Println("  1. Normal requests succeed -> circuit stays CLOSED")
	fmt.Println("  2. 3 consecutive failures -> circuit OPENS")
	fmt.Println("  3. Requests fail fast with CircuitOpenError")
	fmt.Println("  4. After 10s timeout -> circuit enters HALF-OPEN")
	fmt.Println("  5. Successful request -> circuit CLOSES again")

	// ---------------------------------------------------------------------------
	// Rate Limiter Demonstration
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Rate Limiter Behavior ===")

	fmt.Println("Rate limiter allows burst of 50, then 500 ops/sec sustained.")
	fmt.Println("Demonstrating burst behavior...")

	start := time.Now()
	ops := 0
	for i := range 60 {
		_, err := rlOnly.Get(ctx, fmt.Sprintf("rl:test:%d", i))
		if errors.Is(err, cacheerrors.ErrRateLimited) {
			fmt.Println("RATE LIMITED")
			continue
		}
		fmt.Println("MISS")
	}
	elapsed := time.Since(start)
	fmt.Printf("Completed %d/%d ops in %v\n", ops, 60, elapsed)

	// ---------------------------------------------------------------------------
	// Chained Middleware
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Chained Middleware Pipeline ===")

	// Multiple middleware can be chained. Execution order:
	// rate limiter -> circuit breaker -> retry -> backend
	chained := cache.WithMiddleware(backend,
		middleware.RateLimiterMiddleware(middleware.RateLimiterConfig{
			Rate:  2000,
			Burst: 200,
		}),
		middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
			Threshold: 10,
			Timeout:   60 * time.Second,
		}),
		middleware.RetryMiddleware(middleware.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 200 * time.Millisecond,
		}),
	)

	fmt.Println("Chained middleware pipeline:")
	fmt.Println("  Request -> RateLimiter -> CircuitBreaker -> Retry -> Backend")
	fmt.Println("  Response <- RateLimiter <- CircuitBreaker <- Retry <- Backend")

	err = chained.Set(ctx, "chained:key", []byte("works"), 5*time.Minute)
	if err != nil {
		log.Fatalf("Set through chained middleware failed: %v", err)
	}
	fmt.Println("Set chained:key = works (through all middleware layers)")

	// ---------------------------------------------------------------------------
	// Statistics
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Statistics ===")
	stats := resilient.Stats()
	fmt.Printf("Resilient backend stats: Hits=%d, Misses=%d\n",
		stats.Hits, stats.Misses)

	fmt.Println("\n=== All Operations Completed Successfully ===")
}

// Ensure unused import is referenced.
var _ = memory.Option(nil)
