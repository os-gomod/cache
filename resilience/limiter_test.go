package resilience

import (
	"context"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(10, 5)
	if l == nil {
		t.Fatal("expected non-nil limiter")
	}
}

func TestNewLimiterWithConfig(t *testing.T) {
	l := NewLimiterWithConfig(LimiterConfig{
		ReadRPS:    100,
		ReadBurst:  10,
		WriteRPS:   50,
		WriteBurst: 5,
	})
	if l == nil {
		t.Fatal("expected non-nil limiter")
	}
	// Verify read burst is consumed but write burst is still available.
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if !l.AllowRead(ctx) {
			t.Fatalf("read allowed failed at iteration %d", i)
		}
	}
	// Read burst should be exhausted now (rate hasn't had time to refill).
	if l.AllowRead(ctx) {
		t.Fatal("expected read to be exhausted")
	}
	// Write burst should still have tokens.
	if !l.AllowWrite(ctx) {
		t.Fatal("expected write to succeed within burst")
	}
}

func TestAllowRead_WithinBurst(t *testing.T) {
	l := NewLimiter(10, 5)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if !l.AllowRead(ctx) {
			t.Fatalf("expected read %d to succeed within burst", i+1)
		}
	}
}

func TestAllowWrite_WithinBurst(t *testing.T) {
	l := NewLimiter(10, 5)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if !l.AllowWrite(ctx) {
			t.Fatalf("expected write %d to succeed within burst", i+1)
		}
	}
}

func TestAllowRead_Exhausted(t *testing.T) {
	l := NewLimiter(0.001, 2) // very low rate, burst of 2
	ctx := context.Background()
	if !l.AllowRead(ctx) {
		t.Fatal("first read should succeed")
	}
	if !l.AllowRead(ctx) {
		t.Fatal("second read should succeed")
	}
	// Third read should fail since burst is exhausted and rate is very low.
	if l.AllowRead(ctx) {
		t.Fatal("expected read to be exhausted")
	}
}

func TestAllowWrite_Exhausted(t *testing.T) {
	l := NewLimiter(0.001, 2) // very low rate, burst of 2
	ctx := context.Background()
	if !l.AllowWrite(ctx) {
		t.Fatal("first write should succeed")
	}
	if !l.AllowWrite(ctx) {
		t.Fatal("second write should succeed")
	}
	if l.AllowWrite(ctx) {
		t.Fatal("expected write to be exhausted")
	}
}

func TestAllowRead_NilLimiter(t *testing.T) {
	var l *Limiter
	if !l.AllowRead(context.Background()) {
		t.Fatal("nil limiter should allow all reads")
	}
}

func TestAllowWrite_NilLimiter(t *testing.T) {
	var l *Limiter
	if !l.AllowWrite(context.Background()) {
		t.Fatal("nil limiter should allow all writes")
	}
}

func TestAllowRead_CancelledContext(t *testing.T) {
	l := NewLimiter(10, 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if l.AllowRead(ctx) {
		t.Fatal("cancelled context should not allow read")
	}
}

func TestAllowRead_ZeroRate(t *testing.T) {
	l := NewLimiter(0, 5)
	ctx := context.Background()
	// With rate=0, tokenBucket.allow returns true for all calls.
	for i := 0; i < 100; i++ {
		if !l.AllowRead(ctx) {
			t.Fatalf("zero rate should allow all reads, failed at %d", i)
		}
	}
}

func TestAllowRead_Refill(t *testing.T) {
	l := NewLimiter(1000, 1) // high rate, burst of 1
	ctx := context.Background()
	if !l.AllowRead(ctx) {
		t.Fatal("first read should succeed")
	}
	if l.AllowRead(ctx) {
		t.Fatal("second read should fail immediately")
	}
	// Wait enough for a token to refill: 1/1000s = 1ms
	time.Sleep(2 * time.Millisecond)
	if !l.AllowRead(ctx) {
		t.Fatal("read should succeed after refill")
	}
}
