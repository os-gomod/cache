package resilience

import (
	"sync/atomic"
	"time"
)

// State represents the current state of a circuit breaker.
type State int32

const (
	// StateClosed is the normal operating state where requests are allowed.
	StateClosed State = iota
	// StateOpen is the tripped state where requests are rejected.
	StateOpen
	// StateHalfOpen allows a probe request to test recovery.
	StateHalfOpen
)

// String returns a human-readable representation of the circuit breaker state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type circuitState struct {
	failures    int64
	_           [56]byte
	state       int32
	lastFailure int64
	_           [52]byte
}

// CircuitBreaker implements a three-state circuit breaker pattern to prevent
// cascading failures when a downstream dependency is unhealthy.
type CircuitBreaker struct {
	st            atomic.Pointer[circuitState]
	failureThresh int64
	resetTimeout  time.Duration
	probeInFlight atomic.Bool
}

// NewCircuitBreaker creates a new CircuitBreaker with the given failure threshold
// and reset timeout. A threshold of 1 or less is normalized to 1.
func NewCircuitBreaker(threshold int64, resetTimeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	if resetTimeout < 0 {
		resetTimeout = 0
	}
	cb := &CircuitBreaker{failureThresh: threshold, resetTimeout: resetTimeout}
	cb.st.Store(&circuitState{state: int32(StateClosed)})
	return cb
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	if cb == nil {
		return StateClosed
	}
	return State(atomic.LoadInt32(&cb.st.Load().state))
}

// Allow reports whether a request should be permitted under the current state.
func (cb *CircuitBreaker) Allow() bool {
	if cb == nil {
		return true
	}
	st := cb.st.Load()
	switch State(atomic.LoadInt32(&st.state)) {
	case StateClosed:
		return true
	case StateOpen:
		if cb.resetTimeout == 0 {
			return false
		}
		elapsed := time.Duration(time.Now().UnixNano() - atomic.LoadInt64(&st.lastFailure))
		if elapsed < cb.resetTimeout {
			return false
		}
		if !atomic.CompareAndSwapInt32(&st.state, int32(StateOpen), int32(StateHalfOpen)) {
			return false
		}
		if !cb.probeInFlight.CompareAndSwap(false, true) {
			return false
		}
		return true
	case StateHalfOpen:
		return cb.probeInFlight.CompareAndSwap(false, true)
	default:
		return false
	}
}

// Success records a successful operation, resetting the failure counter and closing the circuit.
func (cb *CircuitBreaker) Success() {
	if cb == nil {
		return
	}
	cb.probeInFlight.Store(false)
	st := cb.st.Load()
	atomic.StoreInt64(&st.failures, 0)
	atomic.StoreInt32(&st.state, int32(StateClosed))
}

// Failure records a failed operation, incrementing the failure counter and
// potentially opening the circuit.
func (cb *CircuitBreaker) Failure() {
	if cb == nil {
		return
	}
	cb.probeInFlight.Store(false)
	st := cb.st.Load()
	atomic.StoreInt64(&st.lastFailure, time.Now().UnixNano())
	failures := atomic.AddInt64(&st.failures, 1)
	prev := State(atomic.LoadInt32(&st.state))
	if prev == StateHalfOpen || failures >= cb.failureThresh {
		atomic.CompareAndSwapInt32(&st.state, int32(prev), int32(StateOpen))
	}
}

// Reset manually resets the circuit breaker to the closed state and clears the failure counter.
func (cb *CircuitBreaker) Reset() {
	if cb == nil {
		return
	}
	cb.probeInFlight.Store(false)
	st := cb.st.Load()
	atomic.StoreInt64(&st.failures, 0)
	atomic.StoreInt32(&st.state, int32(StateClosed))
}
