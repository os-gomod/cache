package stampede

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/memory/eviction"
	"github.com/os-gomod/cache/observability"
)

func TestGenerateToken(t *testing.T) {
	t1 := GenerateToken()
	t2 := GenerateToken()
	if t1 == "" {
		t.Error("GenerateToken should return non-empty string")
	}
	if t1 == t2 {
		t.Error("two tokens should be different")
	}
	if len(t1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("token length = %d, want 32", len(t1))
	}
}

func TestDetector_WithObserver(t *testing.T) {
	var log []string
	ic := &logInterceptor{log: &log}
	chain := observability.NewChain(ic)

	d := NewDetector(1.0, chain)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)

	fn := func(ctx context.Context) ([]byte, error) {
		return []byte("new"), nil
	}

	d.Do(context.Background(), "key", []byte("old"), entry, fn, nil)
	d.Close() // Wait for async refresh

	if len(log) == 0 {
		t.Error("observer should have recorded the early refresh")
	}
}

type logInterceptor struct {
	log *[]string
}

func (l *logInterceptor) Before(ctx context.Context, op observability.Op) context.Context {
	return ctx
}

func (l *logInterceptor) After(ctx context.Context, op observability.Op, result observability.Result) {
	*l.log = append(*l.log, op.Name)
}

func TestDetector_Do_MultipleKeys(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		fn := func(ctx context.Context) ([]byte, error) {
			return []byte("value"), nil
		}
		val, err := d.Do(context.Background(), key, nil, nil, fn, nil)
		if err != nil {
			t.Errorf("key %q: unexpected error: %v", key, err)
		}
		if string(val) != "value" {
			t.Errorf("key %q: got %q, want %q", key, val, "value")
		}
	}
}

func TestDetector_Do_ErrorInRefresh(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)

	fn := func(ctx context.Context) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}

	var onRefreshCalled bool
	onRefresh := func(val []byte) {
		onRefreshCalled = true
	}

	// Should return current value, async refresh fails silently
	val, err := d.Do(context.Background(), "key", []byte("old"), entry, fn, onRefresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "old" {
		t.Errorf("got %q, want 'old'", val)
	}

	d.Close()
	if onRefreshCalled {
		t.Error("onRefresh should not be called when refresh fails")
	}
}

func TestDetector_Do_NilOnRefresh(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)

	fn := func(ctx context.Context) ([]byte, error) {
		return []byte("new"), nil
	}

	// onRefresh is nil, should not panic
	val, err := d.Do(context.Background(), "key", []byte("old"), entry, fn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "old" {
		t.Errorf("got %q, want 'old'", val)
	}

	d.Close() // Wait for goroutine
}

func TestDetector_Do_ConcurrentEarlyRefresh(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)

	callCount := 0
	var mu sync.Mutex
	fn := func(ctx context.Context) ([]byte, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return []byte("new"), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := d.Do(context.Background(), "key", []byte("old"), entry, fn, nil)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(val) != "old" {
				t.Errorf("got %q, want 'old'", val)
			}
		}()
	}
	wg.Wait()
	d.Close()

	mu.Lock()
	cc := callCount
	mu.Unlock()
	// Due to inflight dedup, only 1 goroutine should trigger refresh
	if cc != 1 {
		t.Errorf("expected 1 refresh call, got %d", cc)
	}
}

func TestDetector_Do_EarlyRefreshError(t *testing.T) {
	d := NewDetector(1.0, nil)
	defer d.Close()

	entry := eviction.NewEntry("key", []byte("old"), 1*time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)

	fn := func(ctx context.Context) ([]byte, error) {
		return nil, context.Canceled
	}

	// Should return current value, async refresh error is swallowed
	val, err := d.Do(context.Background(), "key", []byte("old"), entry, fn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "old" {
		t.Errorf("got %q, want 'old'", val)
	}
	d.Close()
}
