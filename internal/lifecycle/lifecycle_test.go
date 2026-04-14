package lifecycle

import (
	"sync"
	"testing"
)

func TestGuard_ZeroValueIsOpen(t *testing.T) {
	var g Guard
	if g.IsClosed() {
		t.Error("zero-value Guard should not be closed")
	}
	if err := g.CheckClosed("test"); err != nil {
		t.Errorf("CheckClosed on open guard should return nil, got %v", err)
	}
}

func TestGuard_Close(t *testing.T) {
	var g Guard
	if alreadyClosed := g.Close(); alreadyClosed {
		t.Error("first Close should return false")
	}
	if !g.IsClosed() {
		t.Error("Guard should be closed after Close")
	}
	if err := g.CheckClosed("test"); err == nil {
		t.Error("CheckClosed on closed guard should return error")
	}
}

func TestGuard_DoubleClose(t *testing.T) {
	var g Guard
	g.Close()
	if alreadyClosed := g.Close(); !alreadyClosed {
		t.Error("second Close should return true (already closed)")
	}
}

func TestGuard_ConcurrentClose(t *testing.T) {
	var g Guard
	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.Close()
		}()
	}
	wg.Wait()

	if !g.IsClosed() {
		t.Error("Guard should be closed after concurrent Close calls")
	}
}
