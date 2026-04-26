package middleware

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

func TestChainBuild(t *testing.T) {
	t.Run("empty chain returns final handler", func(t *testing.T) {
		chain := NewChain()
		called := false
		final := func(ctx context.Context, op contracts.Operation) error {
			called = true
			return nil
		}
		h := chain.Build(final)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("final handler should have been called")
		}
	})

	t.Run("single middleware wraps handler", func(t *testing.T) {
		var order []string
		mw := func(next Handler) Handler {
			return func(ctx context.Context, op contracts.Operation) error {
				order = append(order, "before")
				err := next(ctx, op)
				order = append(order, "after")
				return err
			}
		}
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			order = append(order, "handler")
			return nil
		}
		h := chain.Build(final)
		h(context.Background(), contracts.Operation{Name: "get"})

		if len(order) != 3 {
			t.Fatalf("expected 3 calls, got %d: %v", len(order), order)
		}
		if order[0] != "before" || order[1] != "handler" || order[2] != "after" {
			t.Errorf("unexpected order: %v", order)
		}
	})

	t.Run("multiple middleware in order", func(t *testing.T) {
		var order []string
		mw1 := func(next Handler) Handler {
			return func(ctx context.Context, op contracts.Operation) error {
				order = append(order, "mw1-before")
				err := next(ctx, op)
				order = append(order, "mw1-after")
				return err
			}
		}
		mw2 := func(next Handler) Handler {
			return func(ctx context.Context, op contracts.Operation) error {
				order = append(order, "mw2-before")
				err := next(ctx, op)
				order = append(order, "mw2-after")
				return err
			}
		}
		chain := NewChain(mw1, mw2)
		final := func(ctx context.Context, op contracts.Operation) error {
			order = append(order, "final")
			return nil
		}
		h := chain.Build(final)
		h(context.Background(), contracts.Operation{Name: "get"})

		expected := []string{"mw1-before", "mw2-before", "final", "mw2-after", "mw1-after"}
		if len(order) != len(expected) {
			t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
		}
		for i, exp := range expected {
			if order[i] != exp {
				t.Errorf("position %d: expected %s, got %s", i, exp, order[i])
			}
		}
	})

	t.Run("Use appends middleware", func(t *testing.T) {
		var count atomic.Int32
		mw := func(next Handler) Handler {
			return func(ctx context.Context, op contracts.Operation) error {
				count.Add(1)
				return next(ctx, op)
			}
		}
		chain := NewChain()
		chain.Use(mw)
		chain.Use(mw)

		h := chain.Build(func(ctx context.Context, op contracts.Operation) error { return nil })
		h(context.Background(), contracts.Operation{Name: "get"})

		if count.Load() != 2 {
			t.Errorf("expected 2 middleware calls, got %d", count.Load())
		}
	})
}

func TestNopChain(t *testing.T) {
	chain := NopChain()
	if chain == nil {
		t.Fatal("expected non-nil nop chain")
	}
	if len(chain.middlewares) != 0 {
		t.Errorf("expected 0 middleware, got %d", len(chain.middlewares))
	}
}

func TestRetryMiddleware(t *testing.T) {
	t.Run("no retry on success", func(t *testing.T) {
		var attempts atomic.Int32
		mw := RetryMiddleware(RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Second,
			Jitter:       false,
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			attempts.Add(1)
			return nil
		}
		h := chain.Build(final)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts.Load())
		}
	})

	t.Run("retries on failure", func(t *testing.T) {
		var attempts atomic.Int32
		mw := RetryMiddleware(RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Second,
			Jitter:       false,
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			attempts.Add(1)
			return fmt.Errorf("transient error")
		}
		h := chain.Build(final)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected error after retries exhausted")
		}
		if attempts.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("skips non-retryable errors", func(t *testing.T) {
		var attempts atomic.Int32
		mw := RetryMiddleware(RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Second,
			Jitter:       false,
			RetryableErr: func(err error) bool {
				return err.Error() != "fatal"
			},
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			attempts.Add(1)
			return fmt.Errorf("fatal")
		}
		h := chain.Build(final)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected error")
		}
		if attempts.Load() != 1 {
			t.Errorf("expected 1 attempt (non-retryable), got %d", attempts.Load())
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		mw := RetryMiddleware(RetryConfig{
			MaxAttempts:  5,
			InitialDelay: 10 * time.Second,
			MaxDelay:     time.Minute,
			Jitter:       false,
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			return fmt.Errorf("transient")
		}
		h := chain.Build(final)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()

		err := h(ctx, contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected error from context cancellation")
		}
	})
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	t.Run("allows requests when closed", func(t *testing.T) {
		var stateChanges []struct{ from, to CircuitState }
		mw := CircuitBreakerMiddleware(CircuitBreakerConfig{
			Threshold: 3,
			Timeout:   time.Second,
			OnStateChange: func(from, to CircuitState) {
				stateChanges = append(stateChanges, struct{ from, to CircuitState }{from, to})
			},
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			return nil
		}
		h := chain.Build(final)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("opens after threshold failures", func(t *testing.T) {
		var stateChanges []struct{ from, to CircuitState }
		cfg := CircuitBreakerConfig{
			Threshold: 2,
			Timeout:   time.Second,
			OnStateChange: func(from, to CircuitState) {
				stateChanges = append(stateChanges, struct{ from, to CircuitState }{from, to})
			},
		}
		mw := CircuitBreakerMiddleware(cfg)
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			return fmt.Errorf("failure")
		}
		h := chain.Build(final)

		// First failure
		h(context.Background(), contracts.Operation{Name: "get"})
		// Second failure - should open circuit
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected error when circuit opens")
		}

		// Third request should be rejected
		err = h(context.Background(), contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected error from open circuit")
		}

		if len(stateChanges) < 1 {
			t.Fatal("expected at least one state change")
		}
		if stateChanges[0].to != CircuitOpen {
			t.Errorf("expected transition to open, got %s", stateChanges[0].to)
		}
	})

	t.Run("half-open allows probe request", func(t *testing.T) {
		mw := CircuitBreakerMiddleware(CircuitBreakerConfig{
			Threshold: 1,
			Timeout:   50 * time.Millisecond,
		})
		chain := NewChain(mw)

		// Open the circuit
		failHandler := func(ctx context.Context, op contracts.Operation) error {
			return fmt.Errorf("failure")
		}
		h := chain.Build(failHandler)
		h(context.Background(), contracts.Operation{Name: "get"})

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Should be half-open now, probe request
		successHandler := func(ctx context.Context, op contracts.Operation) error {
			return nil
		}
		h = chain.Build(successHandler)
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err != nil {
			t.Fatalf("expected success on half-open probe: %v", err)
		}
	})
}

func TestRateLimiterMiddleware(t *testing.T) {
	t.Run("allows requests within rate", func(t *testing.T) {
		mw := RateLimiterMiddleware(RateLimiterConfig{
			Rate:  1000,
			Burst: 100,
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			return nil
		}
		h := chain.Build(final)

		for i := 0; i < 10; i++ {
			err := h(context.Background(), contracts.Operation{Name: "get"})
			if err != nil {
				t.Fatalf("request %d should be allowed: %v", i, err)
			}
		}
	})

	t.Run("rejects requests when exhausted", func(t *testing.T) {
		mw := RateLimiterMiddleware(RateLimiterConfig{
			Rate:  0.1,
			Burst: 2,
		})
		chain := NewChain(mw)
		final := func(ctx context.Context, op contracts.Operation) error {
			return nil
		}
		h := chain.Build(final)

		// Should use both burst tokens
		h(context.Background(), contracts.Operation{Name: "get"})
		h(context.Background(), contracts.Operation{Name: "get"})

		// Third should be rejected
		err := h(context.Background(), contracts.Operation{Name: "get"})
		if err == nil {
			t.Fatal("expected rate limit error")
		}
	})
}

func TestMetricsMiddleware(t *testing.T) {
	type record struct {
		op      string
		latency time.Duration
		err     error
	}

	var mu sync.Mutex
	var records []record

	recorder := &mockRecorder{
		recordFn: func(op string, latency time.Duration, err error) {
			mu.Lock()
			records = append(records, record{op, latency, err})
			mu.Unlock()
		},
	}

	mw := MetricsMiddleware(recorder)
	chain := NewChain(mw)
	final := func(ctx context.Context, op contracts.Operation) error {
		return nil
	}
	h := chain.Build(final)
	err := h(context.Background(), contracts.Operation{Name: "get", Backend: "redis"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].op != "get" {
		t.Errorf("expected op=get, got %s", records[0].op)
	}
	if records[0].err != nil {
		t.Errorf("expected nil error, got %v", records[0].err)
	}
	mu.Unlock()
}

type mockRecorder struct {
	recordFn func(op string, latency time.Duration, err error)
}

func (m *mockRecorder) Record(op string, latency time.Duration, err error) {
	m.recordFn(op, latency, err)
}

func TestLoggingMiddleware(t *testing.T) {
	var mu sync.Mutex
	var entries []LogEntry

	logger := &mockLogger{
		logFn: func(entry LogEntry) {
			mu.Lock()
			entries = append(entries, entry)
			mu.Unlock()
		},
	}

	mw := LoggingMiddleware(logger)
	chain := NewChain(mw)
	final := func(ctx context.Context, op contracts.Operation) error {
		return fmt.Errorf("not found")
	}
	h := chain.Build(final)
	h(context.Background(), contracts.Operation{
		Name:    "get",
		Key:     "user:123",
		Backend: "redis",
	})

	mu.Lock()
	defer mu.Unlock()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Op != "get" {
		t.Errorf("expected Op=get, got %s", entries[0].Op)
	}
	if entries[0].Key != "user:123" {
		t.Errorf("expected Key=user:123, got %s", entries[0].Key)
	}
	if entries[0].Backend != "redis" {
		t.Errorf("expected Backend=redis, got %s", entries[0].Backend)
	}
	if entries[0].Err == nil {
		t.Error("expected error in log entry")
	}
	if entries[0].Latency <= 0 {
		t.Error("expected positive latency in log entry")
	}
}

type mockLogger struct {
	logFn func(LogEntry)
}

func (m *mockLogger) Log(entry LogEntry) {
	m.logFn(entry)
}

func TestChainBeforeAfter(t *testing.T) {
	chain := NewChain()
	var beforeCalled, afterCalled bool

	chain.beforeHooks = append(
		chain.beforeHooks,
		func(ctx context.Context, op contracts.Operation) context.Context {
			beforeCalled = true
			return ctx
		},
	)
	chain.afterHooks = append(
		chain.afterHooks,
		func(ctx context.Context, op contracts.Operation, result contracts.Result) {
			afterCalled = true
		},
	)

	op := contracts.Operation{Name: "get", Key: "k"}
	ctx := chain.Before(context.Background(), op)
	if !beforeCalled {
		t.Error("expected Before to be called")
	}

	chain.After(ctx, op, contracts.Result{Hit: true})
	if !afterCalled {
		t.Error("expected After to be called")
	}
}
