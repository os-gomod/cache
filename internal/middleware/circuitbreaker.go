package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed indicates the circuit is closed and requests flow normally.
	CircuitClosed CircuitState = iota
	// CircuitOpen indicates the circuit is open and requests are rejected.
	CircuitOpen
	// CircuitHalfOpen indicates the circuit is half-open, allowing a probe request.
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds the configuration for the circuit breaker middleware.
type CircuitBreakerConfig struct {
	// Threshold is the number of consecutive failures before the circuit opens.
	Threshold int

	// Timeout is the duration the circuit stays open before transitioning to
	// half-open state.
	Timeout time.Duration

	// OnStateChange is an optional callback invoked when the circuit state changes.
	OnStateChange func(from, to CircuitState)
}

// circuitBreaker maintains the state for a single circuit breaker instance.
type circuitBreaker struct {
	mu            sync.RWMutex
	state         CircuitState
	failures      int
	lastFailure   time.Time
	threshold     int
	timeout       time.Duration
	onStateChange func(from, to CircuitState)
}

func newCircuitBreaker(cfg CircuitBreakerConfig) *circuitBreaker {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &circuitBreaker{
		state:         CircuitClosed,
		threshold:     cfg.Threshold,
		timeout:       cfg.Timeout,
		onStateChange: cfg.OnStateChange,
	}
}

// allow checks if a request should be allowed through the circuit.
func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.transition(CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

// recordSuccess records a successful request and closes the circuit.
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.transition(CircuitClosed)
	}
	cb.failures = 0
}

// recordFailure records a failed request and potentially opens the circuit.
func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.transition(CircuitOpen)
		return
	}

	if cb.failures >= cb.threshold {
		cb.transition(CircuitOpen)
	}
}

// transition changes the circuit state, invoking the callback if set.
func (cb *circuitBreaker) transition(to CircuitState) {
	from := cb.state
	cb.state = to
	if cb.onStateChange != nil && from != to {
		cb.onStateChange(from, to)
	}
}

// State returns the current circuit state.
func (cb *circuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// CircuitBreakerMiddleware returns a Middleware that implements the circuit
// breaker pattern. After Threshold consecutive failures, the circuit opens
// and rejects requests for the configured Timeout duration. After the timeout,
// the circuit transitions to half-open and allows a single probe request.
func CircuitBreakerMiddleware(cfg CircuitBreakerConfig) Middleware {
	cb := newCircuitBreaker(cfg)

	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			if !cb.allow() {
				return fmt.Errorf(
					"circuit breaker is open for operation %s after %d consecutive failures",
					op.Name,
					cb.threshold,
				)
			}

			err := next(ctx, op)
			if err != nil {
				cb.recordFailure()
			} else {
				cb.recordSuccess()
			}
			return err
		}
	}
}
