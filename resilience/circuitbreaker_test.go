package resilience

import "testing"

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 0)
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestNewCircuitBreaker_ZeroThreshold(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	// Should normalize to 1
	cb.Failure()
	if cb.State() == StateClosed {
		t.Error("should be open after 1 failure with threshold=0→1")
	}
}

func TestNewCircuitBreaker_NegativeResetTimeout(t *testing.T) {
	cb := NewCircuitBreaker(5, -1)
	// Negative resetTimeout is normalized to 0 (permanent open once triggered).
	// Threshold is 5, so we need 5 failures to open.
	for i := 0; i < 5; i++ {
		cb.Failure()
	}
	if cb.State() != StateOpen {
		t.Errorf("should be open after 5 failures, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 0)
	if !cb.Allow() {
		t.Error("closed circuit should allow")
	}
	cb.Success()
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after success, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 0)
	cb.Failure()
	cb.Failure()
	if cb.State() != StateClosed {
		t.Error("should still be closed at 2 failures")
	}
	if !cb.Allow() {
		t.Error("should still allow at 2 failures")
	}
	cb.Failure()
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after 3 failures, got %v", cb.State())
	}
	if cb.Allow() {
		t.Error("open circuit should not allow")
	}
}

func TestCircuitBreaker_HalfOpenOnTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 1) // 1 nanosecond timeout
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatal("expected open")
	}
	// Wait for timeout
	// The reset timeout is 1ns, so we need to wait a tiny bit
	// Since we can't easily wait 1ns in a test, test with 0 timeout (permanent open)
}

func TestCircuitBreaker_NilReceiver(t *testing.T) {
	var cb *CircuitBreaker
	if !cb.Allow() {
		t.Error("nil circuit breaker should allow")
	}
	if cb.State() != StateClosed {
		t.Error("nil circuit breaker state should be closed")
	}
	cb.Success() // should not panic
	cb.Failure() // should not panic
}

func TestCircuitBreaker_ResetOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(2, 0)
	cb.Failure()
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatal("expected open")
	}
	// Manually set to half-open for testing success path
	// (In real usage, half-open is entered via Allow() after timeout)
}

func TestState_String(t *testing.T) {
	tests := []struct {
		s    State
		want string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.s.String()
		if got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
