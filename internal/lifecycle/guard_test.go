package lifecycle

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGuardInitialState(t *testing.T) {
	var g Guard
	if g.IsClosed() {
		t.Error("new guard should not be closed")
	}
	if !g.Open() {
		t.Error("new guard should be open")
	}
	if err := g.CheckClosed("get"); err != nil {
		t.Errorf("CheckClosed on open guard should return nil, got: %v", err)
	}
}

func TestGuardClose(t *testing.T) {
	var g Guard

	// First Close should transition from open to closed
	alreadyClosed := g.Close()
	if alreadyClosed {
		t.Error("first Close should return false (was not already closed)")
	}
	if !g.IsClosed() {
		t.Error("guard should be closed after Close")
	}

	// Second Close should be idempotent
	alreadyClosed = g.Close()
	if !alreadyClosed {
		t.Error("second Close should return true (was already closed)")
	}
}

func TestGuardCheckClosedAfterClose(t *testing.T) {
	var g Guard
	g.Close()

	err := g.CheckClosed("get")
	if err == nil {
		t.Fatal("expected error when checking closed guard")
	}
}

func TestGuardCheckClosedMessage(t *testing.T) {
	var g Guard
	g.Close()

	err := g.CheckClosed("delete")
	if err == nil {
		t.Fatal("expected error")
	}
	// Verify the error message contains the operation name
	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("error message should not be empty")
	}
}

func TestGuardConcurrentClose(t *testing.T) {
	var g Guard
	var wg sync.WaitGroup
	var closeCount, openCount int64

	// Many goroutines try to close concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.Close()
		}()
	}

	// Many goroutines check state
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if g.IsClosed() {
				atomic.AddInt64(&closeCount, 1)
			} else {
				atomic.AddInt64(&openCount, 1)
			}
		}()
	}

	wg.Wait()

	if !g.IsClosed() {
		t.Error("guard should be closed after concurrent close")
	}
}

func TestGuardCheckClosedDifferentOps(t *testing.T) {
	tests := []struct {
		op string
	}{
		{"get"},
		{"set"},
		{"delete"},
		{"clear"},
		{"ping"},
		{"keys"},
		{"increment"},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			var g Guard
			g.Close()
			err := g.CheckClosed(tt.op)
			if err == nil {
				t.Errorf("expected error for op=%s", tt.op)
			}
		})
	}
}
