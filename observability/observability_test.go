package observability

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Interceptor / Chain tests
// ---------------------------------------------------------------------------

func TestNopInterceptor(t *testing.T) {
	ctx := context.Background()
	op := Op{Backend: "memory", Name: "get", Key: "k1"}

	ni := NopInterceptor{}
	returnedCtx := ni.Before(ctx, op)
	assert.Equal(t, ctx, returnedCtx, "NopInterceptor.Before should return the same context")

	ni.After(ctx, op, Result{Hit: true, Latency: time.Millisecond})
	// No panic = success
}

func TestNewChain_Empty(t *testing.T) {
	c := NewChain()
	assert.True(t, c.IsEmpty(), "empty chain should report IsEmpty")
	assert.Equal(t, NopChain(), c, "empty chain should equal NopChain")
}

func TestNewChain_WithInterceptors(t *testing.T) {
	rec := &recordingInterceptor{}
	c := NewChain(rec)
	assert.False(t, c.IsEmpty())

	ctx := context.Background()
	op := Op{Backend: "memory", Name: "get"}
	ctx = c.Before(ctx, op)
	c.After(ctx, op, Result{Hit: true})

	assert.Equal(t, 1, rec.beforeCalls, "Before should be called once")
	assert.Equal(t, 1, rec.afterCalls, "After should be called once")
}

func TestChain_Order(t *testing.T) {
	var order []string
	mk := func(name string) Interceptor {
		return &interceptorFuncs{
			before: func(ctx context.Context, _ Op) context.Context {
				order = append(order, name+".before")
				return ctx
			},
			after: func(_ context.Context, _ Op, _ Result) {
				order = append(order, name+".after")
			},
		}
	}

	c := NewChain(mk("a"), mk("b"), mk("c"))
	ctx := c.Before(context.Background(), Op{})
	c.After(ctx, Op{}, Result{})

	expected := []string{
		"a.before", "b.before", "c.before",
		"c.after", "b.after", "a.after",
	}
	assert.Equal(t, expected, order, "Before should be in order, After in reverse")
}

func TestChain_Append(t *testing.T) {
	rec := &recordingInterceptor{}
	c := NewChain(rec)
	c2 := c.Append(NopInterceptor{})
	assert.False(t, c2.IsEmpty())
	assert.Equal(t, 1, len(c.interceptors), "original chain should be unchanged")
	assert.Equal(t, 2, len(c2.interceptors), "appended chain should have 2 interceptors")
}

// ---------------------------------------------------------------------------
// OTelInterceptor tests
// ---------------------------------------------------------------------------

func TestOTelInterceptor_PanicsOnNilTracer(t *testing.T) {
	assert.Panics(t, func() {
		NewOTelInterceptor(nil)
	}, "NewOTelInterceptor with nil tracer should panic")
}

// ---------------------------------------------------------------------------
// PrometheusInterceptor tests
// ---------------------------------------------------------------------------

func TestPrometheusInterceptor_NilRegisterer(t *testing.T) {
	_, err := NewPrometheusInterceptor(nil)
	assert.Error(t, err, "should error on nil registerer")
}

func TestPrometheusInterceptor_WithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	pi, err := NewPrometheusInterceptor(reg)
	require.NoError(t, err)
	require.NotNil(t, pi)

	// Verify that After does not panic.
	ctx := context.Background()
	op := Op{Backend: "memory", Name: "get", Key: "k1"}
	pi.After(ctx, op, Result{Hit: true, Latency: time.Millisecond})
	pi.After(ctx, op, Result{Hit: false, Latency: 500 * time.Microsecond})
	pi.After(ctx, op, Result{Err: errors.New("boom"), Latency: time.Millisecond})
}

func TestPrometheusInterceptor_BatchOps(t *testing.T) {
	reg := prometheus.NewRegistry()
	pi, err := NewPrometheusInterceptor(reg)
	require.NoError(t, err)

	ctx := context.Background()
	op := Op{Backend: "redis", Name: "get_multi", KeyCount: 5}
	pi.After(ctx, op, Result{Hit: true, ByteSize: 1024, Latency: time.Millisecond})
}

// ---------------------------------------------------------------------------
// LoggingInterceptor tests
// ---------------------------------------------------------------------------

func TestLoggingInterceptor_PanicsOnNilLogger(t *testing.T) {
	assert.Panics(t, func() {
		NewLoggingInterceptor(nil)
	})
}

func TestLoggingInterceptor_BasicLog(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	li := NewLoggingInterceptor(logger)

	ctx := context.Background()
	op := Op{Backend: "memory", Name: "get", Key: "test-key"}
	li.After(ctx, op, Result{Hit: true, Latency: time.Millisecond})

	output := buf.String()
	assert.Contains(t, output, "memory")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "hit")
}

func TestLoggingInterceptor_SlowQuery(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	li := NewLoggingInterceptor(logger, WithSlowThreshold(100*time.Millisecond))

	ctx := context.Background()
	op := Op{Backend: "redis", Name: "get", Key: "slow-key"}
	li.After(ctx, op, Result{Hit: true, Latency: 200 * time.Millisecond})

	output := buf.String()
	assert.Contains(t, output, "slow", "slow query should be flagged")
}

func TestLoggingInterceptor_TruncateKey(t *testing.T) {
	assert.Equal(t, "short", truncateKey("short", 32))
	longKey := "this-is-a-very-long-key-that-exceeds-the-max-length"
	truncated := truncateKey(longKey, 32)
	assert.Equal(t, 32, len(truncated), "truncated key should be exactly 32 chars")
	assert.True(t, strings.HasSuffix(truncated, "..."), "truncated key should end with ...")
	assert.Equal(t, "ab...", truncateKey("abcdef", 5))
}

// ---------------------------------------------------------------------------
// ProviderInterceptor tests — removed (NewProviderInterceptor was removed)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// HitRateWindow tests
// ---------------------------------------------------------------------------

func TestHitRateWindow_Basic(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)

	// Record 7 hits and 3 misses.
	for i := 0; i < 7; i++ {
		w.Record(true)
	}
	for i := 0; i < 3; i++ {
		w.Record(false)
	}

	rate := w.HitRate()
	assert.InDelta(t, 0.7, rate, 0.01, "hit rate should be ~70%%")
}

func TestHitRateWindow_NoOperations(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	assert.Equal(t, 0.0, w.HitRate(), "empty window should return 0")
}

func TestHitRateWindow_Advance(t *testing.T) {
	w := NewHitRateWindow(time.Second, 3)

	// Fill current bucket with misses.
	w.Record(false)
	w.Record(false)

	// Advance twice (current bucket is now index 2, previous ones are 0,1).
	w.Advance()
	w.Advance()

	// Record hits in the new bucket.
	w.Record(true)
	w.Record(true)

	// Hit rate should reflect all 3 buckets.
	rate := w.HitRate()
	assert.InDelta(t, 0.5, rate, 0.01, "2 hits / 4 total = 50%%")
}

func TestHitRateWindow_Reset(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	w.Record(true)
	w.Record(true)
	w.Record(false)
	w.Reset()
	assert.Equal(t, 0.0, w.HitRate(), "after reset, hit rate should be 0")
}

func TestHitRateWindow_LatencyPercentiles(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	for i := 0; i < 100; i++ {
		w.RecordLatency(time.Duration(i+1) * time.Millisecond)
	}

	p50 := w.P50Latency()
	p99 := w.P99Latency()

	assert.Greater(t, p50, time.Duration(0), "p50 should be > 0")
	assert.Greater(t, p99, p50, "p99 should be greater than p50")
}

func TestHitRateWindow_NoLatencySamples(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	assert.Equal(t, time.Duration(0), w.P50Latency())
	assert.Equal(t, time.Duration(0), w.P99Latency())
}

func TestHitRateWindow_Defaults(t *testing.T) {
	w := NewHitRateWindow(0, 0)
	require.NotNil(t, w)
	// Should use defaults (1s interval, 60 buckets).
	w.Record(true)
	assert.Greater(t, w.HitRate(), 0.0)
}

// ---------------------------------------------------------------------------
// Concurrent access test
// ---------------------------------------------------------------------------

func TestHitRateWindow_ConcurrentRecord(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(hit bool) {
			defer wg.Done()
			w.Record(hit)
		}(i%2 == 0)
	}
	wg.Wait()

	rate := w.HitRate()
	assert.Greater(t, rate, 0.0)
	assert.Less(t, rate, 1.0)
}

// ---------------------------------------------------------------------------
// Integration: Chain + NopInterceptor
// ---------------------------------------------------------------------------

func TestChainWithNopInterceptor(t *testing.T) {
	// Verify NopInterceptor works in a chain.
	ni := NopInterceptor{}
	rec := &recordingInterceptor{}
	c := NewChain(ni, rec)

	ctx := context.Background()
	op := Op{Backend: "memory", Name: "get", Key: "k1"}
	ctx = c.Before(ctx, op)
	c.After(ctx, op, Result{Hit: true, Latency: time.Millisecond})

	assert.Equal(t, 1, rec.beforeCalls, "interceptor Before should be called once")
	assert.Equal(t, 1, rec.afterCalls, "interceptor After should be called once")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type recordingInterceptor struct {
	mu          sync.Mutex
	beforeCalls int
	afterCalls  int
}

func (r *recordingInterceptor) Before(ctx context.Context, _ Op) context.Context {
	r.mu.Lock()
	r.beforeCalls++
	r.mu.Unlock()
	return ctx
}

func (r *recordingInterceptor) After(_ context.Context, _ Op, _ Result) {
	r.mu.Lock()
	r.afterCalls++
	r.mu.Unlock()
}

type interceptorFuncs struct {
	before func(ctx context.Context, op Op) context.Context
	after  func(ctx context.Context, op Op, result Result)
}

func (i *interceptorFuncs) Before(ctx context.Context, op Op) context.Context {
	if i.before != nil {
		return i.before(ctx, op)
	}
	return ctx
}

func (i *interceptorFuncs) After(ctx context.Context, op Op, result Result) {
	if i.after != nil {
		i.after(ctx, op, result)
	}
}
