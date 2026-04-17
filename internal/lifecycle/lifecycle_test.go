package lifecycle

import (
	"testing"

	"github.com/os-gomod/cache/errors"
)

func TestGuard_InitialState(t *testing.T) {
	var g Guard
	if g.IsClosed() {
		t.Error("new Guard should not be closed")
	}
	if err := g.CheckClosed("test.op"); err != nil {
		t.Errorf("CheckClosed on open guard: %v", err)
	}
}

func TestGuard_Close(t *testing.T) {
	var g Guard
	alreadyClosed := g.Close()
	if alreadyClosed {
		t.Error("first Close should return false (was not already closed)")
	}
	if !g.IsClosed() {
		t.Error("guard should be closed after Close")
	}
	if err := g.CheckClosed("test.op"); err == nil {
		t.Error("CheckClosed on closed guard should return error")
	}
}

func TestGuard_DoubleClose(t *testing.T) {
	var g Guard
	g.Close()
	alreadyClosed := g.Close()
	if !alreadyClosed {
		t.Error("second Close should return true (was already closed)")
	}
}

func TestGuard_CheckClosed_Error(t *testing.T) {
	var g Guard
	g.Close()
	err := g.CheckClosed("myop")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.IsCacheClosed(err) {
		t.Errorf("expected CacheClosed error, got %v", err)
	}
}
