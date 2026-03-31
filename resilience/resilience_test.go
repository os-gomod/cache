package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	getFunc    func(ctx context.Context, key string) ([]byte, error)
	setFunc    func(ctx context.Context, key string, val []byte, ttl time.Duration) error
	deleteFunc func(ctx context.Context, key string) error
	existsFunc func(ctx context.Context, key string) (bool, error)
	ttlFunc    func(ctx context.Context, key string) (time.Duration, error)
	closeFunc  func(ctx context.Context) error
)

// mockBackend implements Backend interface for testing.
type mockBackend struct {
	getFunc    getFunc
	setFunc    setFunc
	deleteFunc deleteFunc
	existsFunc existsFunc
	ttlFunc    ttlFunc
}

func (m *mockBackend) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, key)
	}
	return []byte("value"), nil
}

func (m *mockBackend) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if m.setFunc != nil {
		return m.setFunc(ctx, key, val, ttl)
	}
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

func (m *mockBackend) Exists(ctx context.Context, key string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, key)
	}
	return true, nil
}

func (m *mockBackend) TTL(ctx context.Context, key string) (time.Duration, error) {
	if m.ttlFunc != nil {
		return m.ttlFunc(ctx, key)
	}
	return time.Minute, nil
}

type closableMockBackend struct {
	mockBackend
	closeFunc closeFunc
}

func (m *closableMockBackend) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Hooks Tests
// ----------------------------------------------------------------------------

func TestHooks_OnGet(t *testing.T) {
	var called bool
	hooks := &Hooks{
		OnGet: func(_ context.Context, key string, hit bool, errKind string, d time.Duration) {
			called = true
			assert.Equal(t, "test-key", key)
			assert.True(t, hit)
			assert.Empty(t, errKind)
			assert.Greater(t, d, time.Duration(0))
		},
	}

	hooks.onGet(context.Background(), "test-key", true, "", time.Millisecond)
	assert.True(t, called)
}

func TestHooks_OnGet_Nil(_ *testing.T) {
	var hooks *Hooks
	// Should not panic
	hooks.onGet(context.Background(), "key", true, "", time.Millisecond)
}

func TestHooks_OnSet(t *testing.T) {
	var called bool
	hooks := &Hooks{
		OnSet: func(_ context.Context, key string, size int, d time.Duration) {
			called = true
			assert.Equal(t, "test-key", key)
			assert.Equal(t, 10, size)
			assert.Greater(t, d, time.Duration(0))
		},
	}

	hooks.onSet(context.Background(), "test-key", 10, time.Millisecond)
	assert.True(t, called)
}

func TestHooks_OnError(t *testing.T) {
	var called bool
	testErr := errors.New("test error")
	hooks := &Hooks{
		OnError: func(_ context.Context, op string, err error) {
			called = true
			assert.Equal(t, "test-op", op)
			assert.Equal(t, testErr, err)
		},
	}

	hooks.onError(context.Background(), "test-op", testErr)
	assert.True(t, called)
}

func TestHooks_OnStateChange(t *testing.T) {
	var called bool
	hooks := &Hooks{
		OnStateChange: func(from, to State) {
			called = true
			assert.Equal(t, StateClosed, from)
			assert.Equal(t, StateOpen, to)
		},
	}

	hooks.onStateChange(StateClosed, StateOpen)
	assert.True(t, called)
}

func TestCache_Close_DelegatesToBackend(t *testing.T) {
	var called bool
	backend := &closableMockBackend{
		closeFunc: func(ctx context.Context) error {
			called = true
			assert.NotNil(t, ctx)
			return nil
		},
	}

	cache := NewCache(backend, Options{})

	require.NoError(t, cache.Close(context.Background()))
	assert.True(t, called)
}

func TestCache_Close_NoOpWhenBackendIsNotClosable(t *testing.T) {
	cache := NewCache(&mockBackend{}, Options{})

	require.NoError(t, cache.Close(context.Background()))
}

// ----------------------------------------------------------------------------
// Circuit Breaker Tests
// ----------------------------------------------------------------------------

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Second)
	assert.NotNil(t, cb)
	assert.Equal(t, int64(5), cb.failureThresh)
	assert.Equal(t, time.Second, cb.resetTimeout)
	assert.Equal(t, StateClosed, cb.State())
}

func TestNewCircuitBreaker_DefaultThreshold(t *testing.T) {
	cb := NewCircuitBreaker(0, time.Second)
	assert.Equal(t, int64(1), cb.failureThresh)
}

func TestNewCircuitBreaker_NegativeTimeout(t *testing.T) {
	cb := NewCircuitBreaker(3, -time.Second)
	assert.Equal(t, time.Duration(0), cb.resetTimeout)
}

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_Allow_Open_AfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	assert.True(t, cb.Allow())
	cb.Failure()
	assert.False(t, cb.Allow())
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_Allow_Open_Timeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	cb.Failure()
	cb.Failure()
	assert.False(t, cb.Allow())

	time.Sleep(150 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_Allow_HalfOpen_SingleTrial(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	cb.Failure()
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(150 * time.Millisecond)

	var wg sync.WaitGroup
	allowed := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.Allow() {
				allowed++
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, allowed)
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_Success_ClosesBreaker(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())

	cb.Success()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_Success_ResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	cb.Success()
	cb.Failure()
	// Should not open because failure count was reset
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_Failure_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	cb.Failure()
	cb.Failure()
	time.Sleep(150 * time.Millisecond)

	cb.Allow() // Transition to half-open
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_Failure_Threshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)

	cb.Failure()
	assert.Equal(t, StateClosed, cb.State())
	cb.Failure()
	assert.Equal(t, StateClosed, cb.State())
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.Failure()
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())

	cb.Reset()
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_Nil(t *testing.T) {
	var cb *CircuitBreaker
	assert.True(t, cb.Allow())
	cb.Success()
	cb.Failure()
	cb.Reset()
}

// ----------------------------------------------------------------------------
// Token Bucket Tests
// ----------------------------------------------------------------------------

func TestTokenBucket_Unlimited(t *testing.T) {
	b := newTokenBucket(0, 10)
	for i := 0; i < 100; i++ {
		assert.True(t, b.allow(time.Now()))
	}
}

func TestTokenBucket_RateLimit(t *testing.T) {
	b := newTokenBucket(10, 1)
	now := time.Now()

	// First token should be available
	assert.True(t, b.allow(now))

	// Second token should be denied (burst=1)
	assert.False(t, b.allow(now))

	// After waiting, token should be available
	time.Sleep(110 * time.Millisecond)
	assert.True(t, b.allow(time.Now()))
}

func TestTokenBucket_Burst(t *testing.T) {
	b := newTokenBucket(10, 5)
	now := time.Now()

	for i := 0; i < 5; i++ {
		assert.True(t, b.allow(now))
	}
	assert.False(t, b.allow(now))
}

func TestTokenBucket_Nil(t *testing.T) {
	var b *tokenBucket
	assert.True(t, b.allow(time.Now()))
}

// ----------------------------------------------------------------------------
// Limiter Tests
// ----------------------------------------------------------------------------

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(100, 10)
	assert.NotNil(t, l)
	assert.NotNil(t, l.read)
	assert.NotNil(t, l.write)
}

func TestNewLimiterWithConfig(t *testing.T) {
	cfg := LimiterConfig{
		ReadRPS:    100,
		ReadBurst:  20,
		WriteRPS:   50,
		WriteBurst: 10,
	}
	l := NewLimiterWithConfig(cfg)
	assert.NotNil(t, l)
}

func TestLimiter_AllowRead(t *testing.T) {
	l := NewLimiter(1, 1)
	ctx := context.Background()

	assert.True(t, l.AllowRead(ctx))
	assert.False(t, l.AllowRead(ctx))
}

func TestLimiter_AllowWrite(t *testing.T) {
	l := NewLimiter(1, 1)
	ctx := context.Background()

	assert.True(t, l.AllowWrite(ctx))
	assert.False(t, l.AllowWrite(ctx))
}

func TestLimiter_AllowRead_Cancelled(t *testing.T) {
	l := NewLimiter(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.False(t, l.AllowRead(ctx))
}

func TestLimiter_Nil(t *testing.T) {
	var l *Limiter
	ctx := context.Background()
	assert.True(t, l.AllowRead(ctx))
	assert.True(t, l.AllowWrite(ctx))
}

// ----------------------------------------------------------------------------
// Cache Wrapper Tests
// ----------------------------------------------------------------------------

func TestNewCache(t *testing.T) {
	backend := &mockBackend{}
	c := NewCache(backend, Options{})
	assert.NotNil(t, c)
	assert.Equal(t, backend, c.backend)
}

func TestNewCache_WithCircuitBreaker(t *testing.T) {
	backend := &mockBackend{}
	cb := NewCircuitBreaker(3, time.Second)
	c := NewCache(backend, Options{CircuitBreaker: cb})
	assert.NotNil(t, c)
	assert.Equal(t, cb, c.cb)
}

func TestNewCache_WithLimiter(t *testing.T) {
	backend := &mockBackend{}
	l := NewLimiter(100, 10)
	c := NewCache(backend, Options{Limiter: l})
	assert.NotNil(t, c)
	assert.Equal(t, l, c.limiter)
}

func TestCache_Get_Success(t *testing.T) {
	backend := &mockBackend{
		getFunc: func(_ context.Context, _ string) ([]byte, error) {
			return []byte("value"), nil
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	val, err := c.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), val)
}

func TestCache_Get_Error(t *testing.T) {
	testErr := errors.New("backend error")
	backend := &mockBackend{
		getFunc: func(_ context.Context, _ string) ([]byte, error) {
			return nil, testErr
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	_, err := c.Get(ctx, "key")
	assert.Equal(t, testErr, err)
}

func TestCache_Get_Unlimited(t *testing.T) {
	backend := &mockBackend{}
	l := NewLimiter(0, 1)

	c := NewCache(backend, Options{Limiter: l})
	ctx := context.Background()

	_, err := c.Get(ctx, "key")
	require.NoError(t, err)

	_, err = c.Get(ctx, "key")
	require.NoError(t, err)
}

func TestCache_Get_RateLimited(t *testing.T) {
	backend := &mockBackend{}
	l := NewLimiter(1, 1)

	c := NewCache(backend, Options{Limiter: l})
	ctx := context.Background()

	_, err := c.Get(ctx, "key")
	require.NoError(t, err)

	_, err = c.Get(ctx, "key")
	assert.Error(t, err)
}

func TestCache_Get_CircuitOpen(t *testing.T) {
	backend := &mockBackend{
		getFunc: func(_ context.Context, _ string) ([]byte, error) {
			return nil, errors.New("error")
		},
	}
	cb := NewCircuitBreaker(1, time.Hour)
	c := NewCache(backend, Options{CircuitBreaker: cb})
	ctx := context.Background()

	_, err := c.Get(ctx, "key")
	require.Error(t, err)

	_, err = c.Get(ctx, "key")
	assert.Error(t, err)
}

func TestCache_Set_Success(t *testing.T) {
	var setCalled bool
	backend := &mockBackend{
		setFunc: func(_ context.Context, key string, val []byte, _ time.Duration) error {
			setCalled = true
			assert.Equal(t, "key", key)
			assert.Equal(t, []byte("value"), val)
			return nil
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	err := c.Set(ctx, "key", []byte("value"), time.Minute)
	require.NoError(t, err)
	assert.True(t, setCalled)
}

func TestCache_Delete_Success(t *testing.T) {
	var deleteCalled bool
	backend := &mockBackend{
		deleteFunc: func(_ context.Context, key string) error {
			deleteCalled = true
			assert.Equal(t, "key", key)
			return nil
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	err := c.Delete(ctx, "key")
	require.NoError(t, err)
	assert.True(t, deleteCalled)
}

func TestCache_Exists_Success(t *testing.T) {
	backend := &mockBackend{
		existsFunc: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	exists, err := c.Exists(ctx, "key")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCache_TTL_Success(t *testing.T) {
	backend := &mockBackend{
		ttlFunc: func(_ context.Context, _ string) (time.Duration, error) {
			return 30 * time.Second, nil
		},
	}
	c := NewCache(backend, Options{})
	ctx := context.Background()

	ttl, err := c.TTL(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, ttl)
}

// ----------------------------------------------------------------------------
// Integration Tests
// ----------------------------------------------------------------------------

func TestCache_CircuitBreaker_Recovery(t *testing.T) {
	failCount := 0
	backend := &mockBackend{
		getFunc: func(_ context.Context, _ string) ([]byte, error) {
			failCount++
			if failCount <= 2 {
				return nil, errors.New("error")
			}
			return []byte("value"), nil
		},
	}
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	c := NewCache(backend, Options{CircuitBreaker: cb})
	ctx := context.Background()

	// First two failures
	_, _ = c.Get(ctx, "key")
	_, _ = c.Get(ctx, "key")

	// Circuit should be open
	_, err := c.Get(ctx, "key")
	assert.Error(t, err)

	// Wait for recovery window
	time.Sleep(150 * time.Millisecond)

	// Should allow trial request and succeed
	val, err := c.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), val)
}

// ----------------------------------------------------------------------------
// Concurrency Tests
// ----------------------------------------------------------------------------

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Second)
	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Failure()
		}()
	}
	wg.Wait()

	assert.Equal(t, StateOpen, cb.State())
}

func TestLimiter_Concurrent(t *testing.T) {
	l := NewLimiter(1000, 100)
	ctx := context.Background()
	var wg sync.WaitGroup
	concurrency := 100
	allowed := int32(0)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.AllowRead(ctx) {
				atomic.AddInt32(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	assert.Greater(t, allowed, int32(0))
	assert.LessOrEqual(t, allowed, int32(100))
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkCircuitBreaker_Allow(b *testing.B) {
	cb := NewCircuitBreaker(10, time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Allow()
	}
}

func BenchmarkCircuitBreaker_Failure(b *testing.B) {
	cb := NewCircuitBreaker(10, time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Failure()
	}
}

func BenchmarkLimiter_AllowRead(b *testing.B) {
	l := NewLimiter(10000, 100)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.AllowRead(ctx)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	backend := &mockBackend{}
	c := NewCache(backend, Options{})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "key")
	}
}

func BenchmarkCache_Get_WithCircuitBreaker(b *testing.B) {
	backend := &mockBackend{}
	cb := NewCircuitBreaker(100, time.Second)
	c := NewCache(backend, Options{CircuitBreaker: cb})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "key")
	}
}

func BenchmarkCache_Get_WithLimiter(b *testing.B) {
	backend := &mockBackend{}
	l := NewLimiter(10000, 100)
	c := NewCache(backend, Options{Limiter: l})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "key")
	}
}
