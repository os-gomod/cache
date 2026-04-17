package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/stats"
)

// mockBackend implements backendiface.Backend with overridable function fields.
type mockBackend struct {
	getFn         func(ctx context.Context, key string) ([]byte, error)
	setFn         func(ctx context.Context, key string, value []byte, ttl time.Duration) error
	deleteFn      func(ctx context.Context, key string) error
	existsFn      func(ctx context.Context, key string) (bool, error)
	ttlFn         func(ctx context.Context, key string) (time.Duration, error)
	closeFn       func(ctx context.Context) error
	pingFn        func(ctx context.Context) error
	statsFn       func() stats.Snapshot
	closedFn      func() bool
	nameFn        func() string
	getMultiFn    func(ctx context.Context, keys ...string) (map[string][]byte, error)
	setMultiFn    func(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	deleteMultiFn func(ctx context.Context, keys ...string) error
}

func (m *mockBackend) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	return nil, nil
}

func (m *mockBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, key)
	}
	return nil
}

func (m *mockBackend) Exists(ctx context.Context, key string) (bool, error) {
	if m.existsFn != nil {
		return m.existsFn(ctx, key)
	}
	return false, nil
}

func (m *mockBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	if m.ttlFn != nil {
		return m.ttlFn(ctx, key)
	}
	return 0, nil
}

func (m *mockBackend) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if m.getMultiFn != nil {
		return m.getMultiFn(ctx, keys...)
	}
	return nil, nil
}

func (m *mockBackend) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if m.setMultiFn != nil {
		return m.setMultiFn(ctx, items, ttl)
	}
	return nil
}

func (m *mockBackend) DeleteMulti(ctx context.Context, keys ...string) error {
	if m.deleteMultiFn != nil {
		return m.deleteMultiFn(ctx, keys...)
	}
	return nil
}

func (m *mockBackend) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockBackend) Close(ctx context.Context) error {
	if m.closeFn != nil {
		return m.closeFn(ctx)
	}
	return nil
}

func (m *mockBackend) Stats() stats.Snapshot {
	if m.statsFn != nil {
		return m.statsFn()
	}
	return stats.Snapshot{}
}

func (m *mockBackend) Closed() bool {
	if m.closedFn != nil {
		return m.closedFn()
	}
	return false
}

func (m *mockBackend) Name() string {
	if m.nameFn != nil {
		return m.nameFn()
	}
	return "mock"
}

// Compile-time check.
var _ backendiface.Backend = (*mockBackend)(nil)

func TestNewCacheWithPolicy(t *testing.T) {
	backend := &mockBackend{}
	p := DefaultPolicy()
	c := NewCacheWithPolicy(backend, p)
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestCache_Get(t *testing.T) {
	want := []byte("hello")
	backend := &mockBackend{
		getFn: func(ctx context.Context, key string) ([]byte, error) {
			if key != "mykey" {
				t.Errorf("expected key=mykey, got %q", key)
			}
			return want, nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	got, err := c.Get(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCache_Set(t *testing.T) {
	var gotKey string
	var gotVal []byte
	backend := &mockBackend{
		setFn: func(ctx context.Context, key string, value []byte, ttl time.Duration) error {
			gotKey = key
			gotVal = value
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	err := c.Set(context.Background(), "mykey", []byte("val"), time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "mykey" {
		t.Errorf("got key %q, want mykey", gotKey)
	}
	if string(gotVal) != "val" {
		t.Errorf("got val %q, want val", gotVal)
	}
}

func TestCache_Delete(t *testing.T) {
	var gotKey string
	backend := &mockBackend{
		deleteFn: func(ctx context.Context, key string) error {
			gotKey = key
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	err := c.Delete(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "mykey" {
		t.Errorf("got key %q, want mykey", gotKey)
	}
}

func TestCache_Exists(t *testing.T) {
	backend := &mockBackend{
		existsFn: func(ctx context.Context, key string) (bool, error) {
			return key == "exists-key", nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	ok, err := c.Exists(context.Background(), "exists-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected exists to return true")
	}
	ok, err = c.Exists(context.Background(), "no-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected exists to return false for unknown key")
	}
}

func TestCache_Close_MultipleTimes(t *testing.T) {
	closeCount := 0
	backend := &mockBackend{
		closeFn: func(ctx context.Context) error {
			closeCount++
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())

	// First close should call backend.
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("first close error: %v", err)
	}
	if closeCount != 1 {
		t.Fatalf("expected backend close called once, got %d", closeCount)
	}

	// Second close should be a no-op.
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("second close error: %v", err)
	}
	if closeCount != 1 {
		t.Fatalf("expected backend close still called once, got %d", closeCount)
	}

	// Third close should also be a no-op.
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("third close error: %v", err)
	}
	if closeCount != 1 {
		t.Fatalf("expected backend close still called once, got %d", closeCount)
	}
}

func TestCache_Name(t *testing.T) {
	c := NewCacheWithPolicy(&mockBackend{}, DefaultPolicy())
	if name := c.Name(); name != "resilience" {
		t.Errorf("expected name 'resilience', got %q", name)
	}
}

func TestCache_Policy(t *testing.T) {
	p := DefaultPolicy()
	c := NewCacheWithPolicy(&mockBackend{}, p)
	got := c.Policy()
	if got.Retry.MaxAttempts != p.Retry.MaxAttempts {
		t.Errorf("expected MaxAttempts=%d, got %d", p.Retry.MaxAttempts, got.Retry.MaxAttempts)
	}
}

func TestCache_Get_Error(t *testing.T) {
	wantErr := errors.New("backend error")
	backend := &mockBackend{
		getFn: func(ctx context.Context, key string) ([]byte, error) {
			return nil, wantErr
		},
	}
	c := NewCacheWithPolicy(backend, NoRetryPolicy())
	_, err := c.Get(context.Background(), "mykey")
	if err == nil {
		t.Fatal("expected error from Get")
	}
	// The error from the backend is wrapped via the policy execution.
	// Since it's non-retryable by default (no RetryableErr function and not a cache error),
	// it should be returned as-is from the first attempt.
}

func TestCache_Ping(t *testing.T) {
	pinged := false
	backend := &mockBackend{
		pingFn: func(ctx context.Context) error {
			pinged = true
			return nil
		},
	}
	c := NewCacheWithPolicy(backend, DefaultPolicy())
	err := c.Ping(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pinged {
		t.Error("expected ping to call backend")
	}
}
