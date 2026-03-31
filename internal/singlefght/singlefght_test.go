// Package singlefght_test provides tests for typed, context-aware singleflight.
package singlefght_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/singlefght"
)

func TestNewGroup(t *testing.T) {
	g := singlefght.NewGroup()
	if g == nil {
		t.Fatal("NewGroup returned nil")
	}
}

func TestGroup_Do_Success(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "test-key"

	executed := false
	result, err := g.Do(ctx, key, func() ([]byte, error) {
		executed = true
		return []byte("success"), nil
	})
	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}
	if string(result) != "success" {
		t.Errorf("Do() result = %s, want success", string(result))
	}
	if !executed {
		t.Error("fn was not executed")
	}
}

func TestGroup_Do_Error(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "error-key"
	expectedErr := errors.New("test error")

	_, err := g.Do(ctx, key, func() ([]byte, error) {
		return nil, expectedErr
	})

	if err != expectedErr {
		t.Errorf("Do() error = %v, want %v", err, expectedErr)
	}
}

func TestGroup_Do_Deduplication(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "dedup-key"

	var executions int
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Create a function that takes time to execute
	fn := func() ([]byte, error) {
		mu.Lock()
		executions++
		count := executions
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		if count%2 == 0 {
			return nil, errors.New("intentional error")
		}
		return []byte("result"), nil
	}

	// Launch multiple concurrent calls
	const numCalls = 10
	results := make(chan []byte, numCalls)
	_errors := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := g.Do(ctx, key, fn)
			if err != nil {
				_errors <- err
			} else {
				results <- result
			}
		}()
	}

	wg.Wait()
	close(results)
	close(_errors)

	// Check that only one execution happened
	if executions != 1 {
		t.Errorf("fn executed %d times, want 1", executions)
	}

	// Check all results are the same
	for result := range results {
		if string(result) != "result" {
			t.Errorf("result = %s, want result", string(result))
		}
	}

	// Check no errors
	if len(_errors) > 0 {
		t.Errorf("got %d errors", len(_errors))
	}
}

func TestGroup_Do_DifferentKeys(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()

	var executions1, executions2 int
	var mu sync.Mutex

	fn1 := func() ([]byte, error) {
		mu.Lock()
		executions1++
		count := executions1
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)

		if count < 0 { // impossible condition
			return nil, errors.New("impossible")
		}
		return []byte("result1"), nil
	}

	fn2 := func() ([]byte, error) {
		mu.Lock()
		executions2++
		count := executions2
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)

		if count < 0 { // impossible condition
			return nil, errors.New("impossible")
		}
		return []byte("result2"), nil
	}

	var wg sync.WaitGroup

	// Launch calls for two different keys
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = g.Do(ctx, "key1", fn1)
		}()
		go func() {
			defer wg.Done()
			_, _ = g.Do(ctx, "key2", fn2)
		}()
	}

	wg.Wait()

	if executions1 != 1 {
		t.Errorf("fn1 executed %d times, want 1", executions1)
	}
	if executions2 != 1 {
		t.Errorf("fn2 executed %d times, want 1", executions2)
	}
}

func TestGroup_Do_ContextCancellation(t *testing.T) {
	g := singlefght.NewGroup()
	key := "cancel-key"

	// Create a function that blocks
	fn := func() ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("result"), nil
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan error, 2)

	// First call starts the operation
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := g.Do(ctx, key, fn)
		results <- err
	}()

	// Give first call time to start
	time.Sleep(10 * time.Millisecond)

	// Second call with cancelled context
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := g.Do(ctx2, key, fn)
		results <- err
	}()

	wg.Wait()
	close(results)

	// Collect errors
	var errs []error
	for err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Second call should have been cancelled
	foundCancel := false
	for _, err := range errs {
		if errors.Is(err, context.Canceled) {
			foundCancel = true
		}
	}
	if !foundCancel {
		t.Error("expected context.Canceled error")
	}
}

func TestGroup_Do_ContextTimeout(t *testing.T) {
	g := singlefght.NewGroup()
	key := "timeout-key"

	// Create a function that takes longer than the timeout
	fn := func() ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("result"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := g.Do(ctx, key, fn)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGroup_Do_ErrorPropagation(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "error-prop-key"
	expectedErr := errors.New("propagation error")

	_, err := g.Do(ctx, key, func() ([]byte, error) {
		return nil, expectedErr
	})

	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestGroup_Forget(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "forget-key"

	var executions int
	var mu sync.Mutex
	fn := func() ([]byte, error) {
		mu.Lock()
		executions++
		mu.Unlock()
		return []byte("result"), nil
	}

	// First call
	_, _ = g.Do(ctx, key, fn)

	// Forget the key
	g.Forget(key)

	// Second call should re-execute
	_, _ = g.Do(ctx, key, fn)

	if executions != 2 {
		t.Errorf("fn executed %d times, want 2", executions)
	}
}

func TestGroup_Forget_WithPendingCalls(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "pending-forget-key"

	var executions int
	var mu sync.Mutex
	fn := func() ([]byte, error) {
		mu.Lock()
		executions++
		count := executions
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)

		if count < 0 { // impossible condition
			return nil, errors.New("impossible")
		}

		return []byte("result"), nil
	}

	var wg sync.WaitGroup

	// Launch a call that will block
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = g.Do(ctx, key, fn)
	}()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Forget the key while call is in flight
	g.Forget(key)

	// Launch another call - this should start a new execution
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = g.Do(ctx, key, fn)
	}()

	wg.Wait()

	// With proper singleflight semantics, the second call should wait for the first
	// but since we called Forget, it should start a new execution
	if executions != 2 {
		t.Errorf("fn executed %d times, want 2", executions)
	}
}

func TestGroup_Do_ConcurrentWithErrors(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "concurrent-error-key"

	var executions int
	var mu sync.Mutex
	fn := func() ([]byte, error) {
		mu.Lock()
		executions++
		mu.Unlock()
		time.Sleep(25 * time.Millisecond)
		return []byte("result"), errors.New("intentional error")
	}

	var wg sync.WaitGroup
	const numCalls = 10
	errs := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Do(ctx, key, fn)
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	// Only one execution
	if executions != 1 {
		t.Errorf("fn executed %d times, want 1", executions)
	}

	// All calls should get the same error
	for err := range errs {
		if err == nil || err.Error() != "intentional error" {
			t.Errorf("error = %v, want intentional error", err)
		}
	}
}

func TestGroup_Do_Panics(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "panic-key"

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	_, _ = g.Do(ctx, key, func() ([]byte, error) {
		panic("test panic")
	})
}

func TestGroup_Do_TypeAssertion(t *testing.T) {
	// This test ensures that the type assertion in Do works correctly
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "type-key"

	result, err := g.Do(ctx, key, func() ([]byte, error) {
		return []byte("correct-type"), nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(result) != "correct-type" {
		t.Errorf("result = %s, want correct-type", string(result))
	}
}

func TestGroup_Do_NilResult(t *testing.T) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "nil-key"

	result, err := g.Do(ctx, key, func() ([]byte, error) {
		return nil, nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func BenchmarkGroup_Do_Success(b *testing.B) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "bench-key"
	fn := func() ([]byte, error) {
		return []byte("result"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.Do(ctx, key, fn)
	}
}

func BenchmarkGroup_Do_Parallel(b *testing.B) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "parallel-key"
	fn := func() ([]byte, error) {
		time.Sleep(time.Microsecond)
		return []byte("result"), nil
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = g.Do(ctx, key, fn)
		}
	})
}

func BenchmarkGroup_Do_DifferentKeys(b *testing.B) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	fn := func() ([]byte, error) {
		return []byte("result"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i % 100))
		_, _ = g.Do(ctx, key, fn)
	}
}

func BenchmarkGroup_Forget(b *testing.B) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	fn := func() ([]byte, error) {
		return []byte("result"), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune(i))
		_, _ = g.Do(ctx, key, fn)
		g.Forget(key)
	}
}

func BenchmarkGroup_Do_WithError(b *testing.B) {
	g := singlefght.NewGroup()
	ctx := context.Background()
	key := "error-key"
	errFn := func() ([]byte, error) {
		return nil, errors.New("bench error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.Do(ctx, key, errFn)
	}
}

func TestGroup_Do_ContextCancellationDuringExecution(t *testing.T) {
	g := singlefght.NewGroup()
	key := "cancel-execution-key"

	// Create a function that respects context cancellation
	fn := func() ([]byte, error) {
		// This function doesn't actually check context, so cancellation
		// won't affect it once started
		time.Sleep(100 * time.Millisecond)
		return []byte("result"), nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	resultCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := g.Do(ctx, key, fn)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context while the function is executing
	cancel()

	// Wait for completion
	select {
	case result := <-resultCh:
		t.Errorf("unexpected result: %s", string(result))
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestGroup_Do_ImmediateContextCancellation(t *testing.T) {
	g := singlefght.NewGroup()
	key := "immediate-cancel-key"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fn := func() ([]byte, error) {
		return []byte("should-not-execute"), nil
	}

	_, err := g.Do(ctx, key, fn)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
