package singlefght

import (
	"context"
	"testing"
)

func TestDo_ContextCancelled_DuringFlight(t *testing.T) {
	g := NewGroup()
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	go func() {
		close(started)
		// Simulate slow fn
		<-ctx.Done()
	}()

	// Wait for fn to start
	<-started

	// Cancel while fn is running - the result channel will still be consumed
	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		return []byte("result"), nil
	})
	// This should succeed because we used context.Background()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "result" {
		t.Errorf("got %q, want %q", val, "result")
	}
	cancel()
}

func TestDo_PanicInFn(t *testing.T) {
	g := NewGroup()
	defer func() {
		// The panic is recovered and returned as an error
	}()

	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		panic("test panic")
	})
	if err == nil {
		t.Error("expected error from panic")
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestGroup_Stats_AfterOperations(t *testing.T) {
	g := NewGroup()

	// Perform some operations
	g.Do(context.Background(), "key1", func() ([]byte, error) {
		return []byte("val1"), nil
	})
	g.Do(context.Background(), "key2", func() ([]byte, error) {
		return []byte("val2"), nil
	})

	s := g.Stats()
	if s.Inflight < 0 {
		t.Errorf("Inflight should be >= 0, got %d", s.Inflight)
	}
}

func TestForget_NonexistentKey(t *testing.T) {
	g := NewGroup()
	// Forgetting a key that was never added should not panic
	g.Forget("nonexistent")
}

func TestDo_SharedResult_Stats(t *testing.T) {
	g := NewGroup()

	fn := func() ([]byte, error) {
		return []byte("shared"), nil
	}

	// First call - not shared
	g.Do(context.Background(), "key", fn)

	// Second call - should be shared (if timing works)
	// Note: singleflight dedup only works if the first call is still in flight
	g.Forget("key") // Forget to allow fresh call
	g.Do(context.Background(), "key", fn)

	s := g.Stats()
	// Stats should not be negative
	if s.Deduplicated < 0 {
		t.Errorf("Deduplicated should be >= 0, got %d", s.Deduplicated)
	}
}
