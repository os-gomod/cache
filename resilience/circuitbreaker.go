package resilience

import (
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

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

// ---------------------------------------------------------------------------
// Circuit Breaker
// ---------------------------------------------------------------------------

// circuitState holds the mutable state of a circuit breaker, stored behind an
// atomic pointer so that all fields are updated atomically together.
type circuitState struct {
	failures    int64
	_           [56]byte //nolint:unused // cache-line padding
	state       int32
	lastFailure int64
	_           [52]byte //nolint:unused // cache-line padding
}

// CircuitBreaker implements the circuit-breaker pattern. When the number of
// consecutive failures exceeds the threshold the breaker opens, rejecting all
// calls until the reset timeout elapses. It then transitions to half-open,
// allowing exactly one probe call. If the probe succeeds the breaker closes;
// if it fails the breaker re-opens.
//
// State transitions are observable via an external observability.Interceptor
// wired at construction time through NewCacheWithPolicy.
type CircuitBreaker struct {
	st            atomic.Pointer[circuitState]
	failureThresh int64
	resetTimeout  time.Duration
	probeInFlight atomic.Bool // ensures exactly one probe in half-open
}

// NewCircuitBreaker creates a circuit breaker that opens after threshold
// consecutive failures and remains open for at least resetTimeout before
// transitioning to half-open.
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

// State returns the current state of the circuit breaker. A nil receiver
// reports StateClosed.
func (cb *CircuitBreaker) State() State {
	if cb == nil {
		return StateClosed
	}
	return State(atomic.LoadInt32(&cb.st.Load().state))
}

// Allow returns true if a request should be permitted. In half-open state it
// allows exactly one concurrent probe request — all others are rejected until
// the probe completes via Success() or Failure().
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
		// Transition to half-open: CAS ensures only one goroutine wins.
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

// Success records a successful operation. In half-open state this closes the
// circuit. The probe-in-flight flag is always cleared.
func (cb *CircuitBreaker) Success() {
	if cb == nil {
		return
	}
	cb.probeInFlight.Store(false)
	st := cb.st.Load()
	atomic.StoreInt64(&st.failures, 0)
	atomic.StoreInt32(&st.state, int32(StateClosed))
}

// Failure records a failed operation. In half-open state this immediately
// re-opens the circuit. The probe-in-flight flag is always cleared.
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

// Reset forcefully closes the circuit and resets all counters.
func (cb *CircuitBreaker) Reset() {
	if cb == nil {
		return
	}
	cb.probeInFlight.Store(false)
	st := cb.st.Load()
	atomic.StoreInt64(&st.failures, 0)
	atomic.StoreInt32(&st.state, int32(StateClosed))
}
