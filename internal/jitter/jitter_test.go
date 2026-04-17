package jitter

import (
	"testing"
	"time"
)

func TestAddJitter_ZeroDuration(t *testing.T) {
	got := AddJitter(0, 0.25)
	if got != 0 {
		t.Errorf("expected 0 for zero duration, got %v", got)
	}
}

func TestAddJitter_ZeroFactor(t *testing.T) {
	got := AddJitter(100*time.Millisecond, 0)
	if got != 100*time.Millisecond {
		t.Errorf("expected 100ms for zero factor, got %v", got)
	}
}

func TestAddJitter_NegativeFactor(t *testing.T) {
	got := AddJitter(100*time.Millisecond, -1)
	if got != 100*time.Millisecond {
		t.Errorf("expected 100ms for negative factor, got %v", got)
	}
}

func TestAddJitter_WithinRange(t *testing.T) {
	base := 100 * time.Millisecond
	factor := 0.25
	// Result should be within [base*0.75, base*1.25].
	for i := 0; i < 100; i++ {
		got := AddJitter(base, factor)
		minDur := time.Duration(float64(base) * 0.75)
		maxDur := time.Duration(float64(base) * 1.25)
		if got < minDur || got > maxDur {
			t.Errorf("iteration %d: got %v, expected between %v and %v", i, got, minDur, maxDur)
		}
	}
}

func TestAddJitter_FactorCappedAtHalf(t *testing.T) {
	base := 100 * time.Millisecond
	factor := 1.0 // should be capped to 0.5
	for i := 0; i < 100; i++ {
		got := AddJitter(base, factor)
		minDur := time.Duration(float64(base) * 0.5)
		maxDur := time.Duration(float64(base) * 1.5)
		if got < minDur || got > maxDur {
			t.Errorf("iteration %d: got %v, expected between %v and %v", i, got, minDur, maxDur)
		}
	}
}
