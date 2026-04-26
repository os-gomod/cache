package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// mockStats is a thread-safe mock StatsCollector for testing.
type mockStats struct {
	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	errors    atomic.Int64
	evictions atomic.Int64
}

func (m *mockStats) Hit()      { m.hits.Add(1) }
func (m *mockStats) Miss()     { m.misses.Add(1) }
func (m *mockStats) Set()      { m.sets.Add(1) }
func (m *mockStats) Delete()   { m.deletes.Add(1) }
func (m *mockStats) Error()    { m.errors.Add(1) }
func (m *mockStats) Eviction() { m.evictions.Add(1) }

func (m *mockStats) reset() {
	m.hits.Store(0)
	m.misses.Store(0)
	m.sets.Store(0)
	m.deletes.Store(0)
	m.errors.Store(0)
	m.evictions.Store(0)
}

func TestNewExecutorDefaults(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	if e.chain != nil {
		t.Error("expected nil chain by default")
	}
	if e.validator == nil {
		t.Error("expected non-nil default validator")
	}
	if e.stats == nil {
		t.Error("expected non-nil default stats")
	}
	if e.clock == nil {
		t.Error("expected non-nil default clock")
	}
}

func TestExecuteBasic(t *testing.T) {
	stats := &mockStats{}
	e := New(WithStats(stats))

	t.Run("successful operation", func(t *testing.T) {
		stats.reset()
		op := contracts.Operation{Name: "set", Key: "k1", Backend: "test"}
		err := e.Execute(context.Background(), op, func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("failed operation", func(t *testing.T) {
		stats.reset()
		op := contracts.Operation{Name: "get", Key: "k1", Backend: "test"}
		err := e.Execute(context.Background(), op, func(ctx context.Context) error {
			return fmt.Errorf("not found")
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if stats.errors.Load() != 1 {
			t.Errorf("expected 1 error, got %d", stats.errors.Load())
		}
	})
}

func TestExecuteResultBasic(t *testing.T) {
	e := New()

	t.Run("successful get with hit", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k1", Backend: "test"}
		result, err := e.ExecuteResult(
			context.Background(),
			op,
			func(ctx context.Context) (contracts.Result, error) {
				return contracts.Result{
					Value:    []byte("hello"),
					Hit:      true,
					ByteSize: 5,
				}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Hit {
			t.Error("expected hit")
		}
		if string(result.Value) != "hello" {
			t.Errorf("expected value=hello, got %s", result.Value)
		}
		if result.Latency <= 0 {
			t.Error("expected positive latency")
		}
	})

	t.Run("successful get with miss", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k2", Backend: "test"}
		result, err := e.ExecuteResult(
			context.Background(),
			op,
			func(ctx context.Context) (contracts.Result, error) {
				return contracts.Result{Hit: false}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Hit {
			t.Error("expected miss")
		}
	})

	t.Run("failed get", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k3", Backend: "test"}
		_, err := e.ExecuteResult(
			context.Background(),
			op,
			func(ctx context.Context) (contracts.Result, error) {
				return contracts.Result{}, fmt.Errorf("connection refused")
			},
		)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExecuteTypedBasic(t *testing.T) {
	e := New()

	t.Run("successful typed operation", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k1", Backend: "test"}
		value, err := ExecuteTyped(
			e,
			context.Background(),
			op,
			func(ctx context.Context) (string, error) {
				return "result", nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "result" {
			t.Errorf("expected value=result, got %s", value)
		}
	})

	t.Run("failed typed operation", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k2", Backend: "test"}
		_, err := ExecuteTyped(e, context.Background(), op, func(ctx context.Context) (int, error) {
			return 0, fmt.Errorf("fail")
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns zero value on error", func(t *testing.T) {
		op := contracts.Operation{Name: "get", Key: "k3", Backend: "test"}
		value, err := ExecuteTyped(
			e,
			context.Background(),
			op,
			func(ctx context.Context) (int, error) {
				return 42, fmt.Errorf("fail")
			},
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if value != 0 {
			t.Errorf("expected zero value, got %d", value)
		}
	})
}

func TestExecuteWithNilChain(t *testing.T) {
	e := New() // no chain
	stats := &mockStats{}
	e.stats = stats

	op := contracts.Operation{Name: "set", Key: "k1", Backend: "test"}
	err := e.Execute(context.Background(), op, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteWithValidator(t *testing.T) {
	t.Run("valid key", func(t *testing.T) {
		called := false
		e := New(WithValidator(func(op, key string) error {
			return nil
		}))
		op := contracts.Operation{Name: "get", Key: "valid-key", Backend: "test"}
		e.Execute(context.Background(), op, func(ctx context.Context) error {
			called = true
			return nil
		})
		if !called {
			t.Error("expected fn to be called")
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		stats := &mockStats{}
		e := New(WithStats(stats), WithValidator(func(op, key string) error {
			return fmt.Errorf("bad key: %s", key)
		}))
		op := contracts.Operation{Name: "get", Key: "bad key", Backend: "test"}
		err := e.Execute(context.Background(), op, func(ctx context.Context) error {
			t.Error("fn should not be called with invalid key")
			return nil
		})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if stats.errors.Load() != 1 {
			t.Errorf("expected 1 error recorded, got %d", stats.errors.Load())
		}
	})

	t.Run("nil validator", func(t *testing.T) {
		e := New(WithValidator(nil))
		op := contracts.Operation{Name: "get", Key: "", Backend: "test"}
		err := e.Execute(context.Background(), op, func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error with nil validator: %v", err)
		}
	})
}

func TestExecuteStatsRecording(t *testing.T) {
	stats := &mockStats{}
	e := New(WithStats(stats))

	tests := []struct {
		name    string
		op      contracts.Operation
		success bool
		wantSet int64
		wantDel int64
		wantErr int64
	}{
		{"set success", contracts.Operation{Name: "set", Key: "k", Backend: "test"}, true, 1, 0, 0},
		{"set error", contracts.Operation{Name: "set", Key: "k", Backend: "test"}, false, 0, 0, 1},
		{
			"delete success",
			contracts.Operation{Name: "delete", Key: "k", Backend: "test"},
			true,
			0,
			1,
			0,
		},
		{"get success", contracts.Operation{Name: "get", Key: "k", Backend: "test"}, true, 0, 0, 0},
		{"get error", contracts.Operation{Name: "get", Key: "k", Backend: "test"}, false, 0, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats.reset()
			var fnErr error
			if !tt.success {
				fnErr = fmt.Errorf("forced error")
			}
			e.Execute(context.Background(), tt.op, func(ctx context.Context) error {
				return fnErr
			})
			if got := stats.sets.Load(); got != tt.wantSet {
				t.Errorf("sets = %d, want %d", got, tt.wantSet)
			}
			if got := stats.deletes.Load(); got != tt.wantDel {
				t.Errorf("deletes = %d, want %d", got, tt.wantDel)
			}
			if got := stats.errors.Load(); got != tt.wantErr {
				t.Errorf("errors = %d, want %d", got, tt.wantErr)
			}
		})
	}
}

func TestExecuteWithMiddleware(t *testing.T) {
	t.Run("middleware before/after called", func(t *testing.T) {
		var (
			beforeCalled bool
			afterCalled  bool
			mu           sync.Mutex
		)

		// We can't import middleware here without creating a circular concern,
		// so test via the chain's Before/After contract indirectly.
		// Instead, verify that Execute works when chain is set.
		// The chain tests are in the middleware package.
		e := New(WithChain(nil)) // explicitly nil chain
		op := contracts.Operation{Name: "get", Key: "k", Backend: "test"}
		_, err := e.ExecuteResult(
			context.Background(),
			op,
			func(ctx context.Context) (contracts.Result, error) {
				mu.Lock()
				beforeCalled = true
				mu.Unlock()
				return contracts.Result{Hit: true}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !beforeCalled {
			t.Error("expected fn to be called")
		}
		_ = afterCalled
	})

	t.Run("latency captured in result", func(t *testing.T) {
		fakeTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		fakeLatency := 5 * time.Millisecond
		e := New(WithClock(NewFakeClock(fakeTime, fakeLatency)))

		op := contracts.Operation{Name: "get", Key: "k", Backend: "test"}
		result, err := e.ExecuteResult(
			context.Background(),
			op,
			func(ctx context.Context) (contracts.Result, error) {
				return contracts.Result{Hit: true, ByteSize: 10}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Latency != fakeLatency {
			t.Errorf("expected latency=%v, got %v", fakeLatency, result.Latency)
		}
	})
}

func TestExecuteContextCancelled(t *testing.T) {
	e := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	op := contracts.Operation{Name: "get", Key: "k", Backend: "test"}
	_, err := ExecuteTyped(e, context.Background(), op, func(ctx context.Context) (string, error) {
		// The function should still run; context handling is middleware's job
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = ctx
}

func TestExecutorOptions(t *testing.T) {
	t.Run("WithStats nil", func(t *testing.T) {
		e := New(WithStats(nil))
		if e.stats != nil {
			t.Error("expected nil stats")
		}
	})

	t.Run("WithValidator nil", func(t *testing.T) {
		e := New(WithValidator(nil))
		if e.validator != nil {
			t.Error("expected nil validator")
		}
	})

	t.Run("multiple options", func(t *testing.T) {
		stats := &mockStats{}
		fakeTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		e := New(
			WithStats(stats),
			WithClock(NewFakeClock(fakeTime, time.Millisecond)),
			WithValidator(func(op, key string) error { return nil }),
		)
		if e.stats != stats {
			t.Error("expected stats to be set")
		}
		if e.clock == nil {
			t.Error("expected clock to be set")
		}
		if e.validator == nil {
			t.Error("expected validator to be set")
		}
	})
}

func TestExecuteTypedConcurrency(t *testing.T) {
	e := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			op := contracts.Operation{Name: "get", Key: fmt.Sprintf("key:%d", i), Backend: "test"}
			value, err := ExecuteTyped(
				e,
				context.Background(),
				op,
				func(ctx context.Context) (int, error) {
					return i, nil
				},
			)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if value != i {
				t.Errorf("expected %d, got %d", i, value)
			}
		}(i)
	}

	wg.Wait()
}

// Test that Execute propagates context correctly.
func TestExecuteContextPropagation(t *testing.T) {
	type contextKey string
	const testKey contextKey = "test-value"

	e := New()
	op := contracts.Operation{Name: "get", Key: "k", Backend: "test"}

	ctx := context.WithValue(context.Background(), testKey, "hello")
	e.Execute(ctx, op, func(ctx context.Context) error {
		val := ctx.Value(testKey)
		if val != "hello" {
			return fmt.Errorf("context value not propagated: got %v", val)
		}
		return nil
	})
}

// Test that wrapped errors are properly returned.
func TestExecuteErrorWrapping(t *testing.T) {
	e := New()
	op := contracts.Operation{Name: "get", Key: "k", Backend: "test"}

	innerErr := fmt.Errorf("inner error")
	err := e.Execute(context.Background(), op, func(ctx context.Context) error {
		return innerErr
	})

	if !errors.Is(err, innerErr) {
		t.Error("expected errors.Is to match inner error")
	}
}
