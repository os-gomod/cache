package singlefght

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewGroup(t *testing.T) {
	g := NewGroup()
	if g == nil {
		t.Fatal("expected non-nil group")
	}
}

func TestDo_Success(t *testing.T) {
	g := NewGroup()
	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		return []byte("result"), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "result" {
		t.Errorf("got %q, want %q", string(val), "result")
	}
}

func TestDo_SharedResult(t *testing.T) {
	g := NewGroup()

	callCount := 0
	fn := func() ([]byte, error) {
		callCount++
		time.Sleep(50 * time.Millisecond) // slow enough for concurrency
		return []byte("shared"), nil
	}

	var wg sync.WaitGroup
	results := make([]string, 10)
	errs := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err := g.Do(context.Background(), "same-key", fn)
			results[idx] = string(val)
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
	for i, r := range results {
		if r != "shared" {
			t.Errorf("goroutine %d: got %q, want shared", i, r)
		}
	}
	// The fn should have been called once due to singleflight deduplication.
	if callCount != 1 {
		t.Errorf("expected fn called once, got %d", callCount)
	}
}

func TestDo_DifferentKeys(t *testing.T) {
	g := NewGroup()

	var mu sync.Mutex
	calls := make(map[string]int)
	fn := func(key string) func() ([]byte, error) {
		return func() ([]byte, error) {
			mu.Lock()
			calls[key]++
			mu.Unlock()
			return []byte(key), nil
		}
	}

	v1, err1 := g.Do(context.Background(), "key1", fn("key1"))
	v2, err2 := g.Do(context.Background(), "key2", fn("key2"))

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if string(v1) != "key1" {
		t.Errorf("got %q, want key1", string(v1))
	}
	if string(v2) != "key2" {
		t.Errorf("got %q, want key2", string(v2))
	}

	mu.Lock()
	defer mu.Unlock()
	if calls["key1"] != 1 || calls["key2"] != 1 {
		t.Errorf("expected each key called once, got %v", calls)
	}
}

func TestDo_CancelledContext(t *testing.T) {
	g := NewGroup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	val, err := g.Do(ctx, "key", func() ([]byte, error) {
		return []byte("should not run"), nil
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if val != nil {
		t.Errorf("expected nil value, got %q", string(val))
	}
}

func TestDo_Error(t *testing.T) {
	g := NewGroup()
	wantErr := errors.New("fn error")
	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		return nil, wantErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestDo_NilResult(t *testing.T) {
	g := NewGroup()
	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestForget(t *testing.T) {
	g := NewGroup()

	// First call populates the cache.
	val, err := g.Do(context.Background(), "key", func() ([]byte, error) {
		return []byte("first"), nil
	})
	if err != nil || string(val) != "first" {
		t.Fatalf("unexpected result: val=%q, err=%v", val, err)
	}

	// Forget the key.
	g.Forget("key")

	// Second call should run the function again.
	callCount := 0
	val, err = g.Do(context.Background(), "key", func() ([]byte, error) {
		callCount++
		return []byte("second"), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected fn to be called again after Forget, got callCount=%d", callCount)
	}
	if string(val) != "second" {
		t.Errorf("got %q, want second", string(val))
	}
}

func TestStats(t *testing.T) {
	g := NewGroup()
	s := g.Stats()
	if s.Inflight != 0 {
		t.Errorf("expected 0 inflight, got %d", s.Inflight)
	}
}
