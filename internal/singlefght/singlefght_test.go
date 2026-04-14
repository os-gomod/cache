package singlefght

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_BasicDo(t *testing.T) {
	g := NewGroup()
	val, err := g.Do(context.Background(), "key1", func() ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "hello" {
		t.Errorf("got %q, want %q", val, "hello")
	}
}

func TestGroup_ContextCancellation(t *testing.T) {
	g := NewGroup()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := g.Do(ctx, "slow-key", func() ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("done"), nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGroup_ConcurrentSameKey(t *testing.T) {
	g := NewGroup()
	var calls atomic.Int32

	startCh := make(chan struct{})
	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			val, err := g.Do(context.Background(), "same-key", func() ([]byte, error) {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond)
				return []byte("shared"), nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(val) != "shared" {
				t.Errorf("got %q, want %q", val, "shared")
			}
		}()
	}

	close(startCh)
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("fn called %d times, want 1 (should be deduplicated)", calls.Load())
	}
}

func TestGroup_DifferentKeys(t *testing.T) {
	g := NewGroup()
	var calls atomic.Int32

	val1, err := g.Do(context.Background(), "key-a", func() ([]byte, error) {
		calls.Add(1)
		return []byte("a"), nil
	})
	if err != nil || string(val1) != "a" {
		t.Fatalf("key-a: got %q, err %v", val1, err)
	}

	val2, err := g.Do(context.Background(), "key-b", func() ([]byte, error) {
		calls.Add(1)
		return []byte("b"), nil
	})
	if err != nil || string(val2) != "b" {
		t.Fatalf("key-b: got %q, err %v", val2, err)
	}

	if calls.Load() != 2 {
		t.Errorf("expected 2 calls for different keys, got %d", calls.Load())
	}
}

func TestGroup_FnError(t *testing.T) {
	g := NewGroup()
	_, err := g.Do(context.Background(), "err-key", func() ([]byte, error) {
		return nil, errors.New("fn failed")
	})
	if err == nil {
		t.Error("expected error from fn")
	}
}

func TestGroup_AlreadyCancelledContext(t *testing.T) {
	g := NewGroup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := g.Do(ctx, "key", func() ([]byte, error) {
		return []byte("should not run"), nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got %v", err)
	}
}

func TestGroup_Stats(t *testing.T) {
	g := NewGroup()
	s := g.Stats()
	if s.Inflight != 0 {
		t.Errorf("initial Inflight = %d, want 0", s.Inflight)
	}
	if s.Deduplicated != 0 {
		t.Errorf("initial Deduplicated = %d, want 0", s.Deduplicated)
	}
}

func TestGroup_Forget(t *testing.T) {
	g := NewGroup()
	var calls atomic.Int32

	_, _ = g.Do(context.Background(), "key", func() ([]byte, error) {
		calls.Add(1)
		return []byte("first"), nil
	})

	g.Forget("key")

	_, _ = g.Do(context.Background(), "key", func() ([]byte, error) {
		calls.Add(1)
		return []byte("second"), nil
	})

	if calls.Load() != 2 {
		t.Errorf("expected 2 calls after Forget, got %d", calls.Load())
	}
}

func TestGroup_NilResult(t *testing.T) {
	g := NewGroup()
	val, err := g.Do(context.Background(), "nil-key", func() ([]byte, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}
