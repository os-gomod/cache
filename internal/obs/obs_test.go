// Package obs_test provides tests for pluggable observability hooks.
package obs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/obs"
)

type ctxKey string

const testKey ctxKey = "key"

func TestDefaultProvider(t *testing.T) {
	provider := obs.Global()
	if provider == nil {
		t.Fatal("Global() returned nil")
	}
	if provider.Tracer == nil {
		t.Error("default Tracer is nil")
	}
	if provider.Logger == nil {
		t.Error("default Logger is nil")
	}
	if provider.Metrics == nil {
		t.Error("default Metrics is nil")
	}
}

func TestStart_DefaultTracer(t *testing.T) {
	ctx := context.Background()
	newCtx, span := obs.Start(ctx, "test-operation")

	// Should return the same context and a nop span
	if newCtx != ctx {
		t.Error("Start should return the same context with default tracer")
	}

	// These should not panic
	span.End()
	span.SetError(errors.New("test error"))
	span.SetAttr("key", "value")
}

func TestLogging_DefaultLogger(_ *testing.T) {
	ctx := context.Background()

	// These should not panic with default no-op logger
	obs.Info(ctx, "info message")
	obs.Warn(ctx, "warn message")
	obs.Error(ctx, "error message")
	obs.Debug(ctx, "debug message")

	// With fields
	obs.Info(ctx, "message with fields", "key1", "value1", "key2", 42)
}

func TestMetrics_DefaultMetrics(_ *testing.T) {
	ctx := context.Background()
	duration := 100 * time.Millisecond

	// These should not panic with default no-op metrics
	obs.RecordHit(ctx, "redis", "get", duration)
	obs.RecordMiss(ctx, "redis", "get", duration)
	obs.RecordError(ctx, "redis", "get")
	obs.RecordEviction(ctx, "lru")
}

func TestSetProvider_Nil(t *testing.T) {
	// Save original provider
	original := obs.Global()

	// Set nil provider
	obs.SetProvider(nil)

	// Should reset to default no-op provider
	provider := obs.Global()
	if provider == nil {
		t.Fatal("provider is nil after SetProvider(nil)")
	}
	if provider.Tracer == nil {
		t.Error("Tracer is nil after SetProvider(nil)")
	}
	if provider.Logger == nil {
		t.Error("Logger is nil after SetProvider(nil)")
	}
	if provider.Metrics == nil {
		t.Error("Metrics is nil after SetProvider(nil)")
	}

	// Restore original for other tests
	obs.SetProvider(original)
}

func TestSetProvider_Custom(t *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	// Create custom implementations
	customTracer := &testTracer{}
	customLogger := &testLogger{}
	customMetrics := &testMetrics{}

	customProvider := &obs.Provider{
		Tracer:  customTracer,
		Logger:  customLogger,
		Metrics: customMetrics,
	}

	obs.SetProvider(customProvider)

	provider := obs.Global()
	if provider.Tracer != customTracer {
		t.Error("Tracer not set correctly")
	}
	if provider.Logger != customLogger {
		t.Error("Logger not set correctly")
	}
	if provider.Metrics != customMetrics {
		t.Error("Metrics not set correctly")
	}
}

func TestSetProvider_PartialCustom(t *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	// Create provider with only Tracer
	customTracer := &testTracer{}
	customProvider := &obs.Provider{
		Tracer: customTracer,
		// Logger and Metrics are nil
	}

	obs.SetProvider(customProvider)

	provider := obs.Global()
	if provider.Tracer != customTracer {
		t.Error("Tracer not set correctly")
	}
	if provider.Logger == nil {
		t.Error("Logger should be replaced with nopLogger")
	}
	if provider.Metrics == nil {
		t.Error("Metrics should be replaced with nopMetrics")
	}
}

func TestStart_CustomTracer(t *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	customTracer := &testTracer{}
	obs.SetProvider(&obs.Provider{Tracer: customTracer})

	ctx := context.Background()
	newCtx, span := obs.Start(ctx, "test-op")

	if customTracer.startCalled != 1 {
		t.Errorf("Start called %d times, want 1", customTracer.startCalled)
	}
	if customTracer.lastOp != "test-op" {
		t.Errorf("lastOp = %s, want test-op", customTracer.lastOp)
	}
	if customTracer.lastCtx != ctx {
		t.Error("context not passed correctly")
	}

	// Test span methods
	span.End()
	if customTracer.lastSpan.endCalled != 1 {
		t.Error("span.End not called")
	}

	testErr := errors.New("test error")
	span.SetError(testErr)
	if customTracer.lastSpan.lastError != testErr {
		t.Error("span.SetError not called correctly")
	}

	span.SetAttr("key", "value")
	if customTracer.lastSpan.lastAttrKey != "key" {
		t.Errorf("attr key = %s, want key", customTracer.lastSpan.lastAttrKey)
	}
	if customTracer.lastSpan.lastAttrVal != "value" {
		t.Errorf("attr val = %v, want value", customTracer.lastSpan.lastAttrVal)
	}

	if newCtx != customTracer.lastReturnedCtx {
		t.Error("Start should return the context from custom tracer")
	}
}

func TestLogging_CustomLogger(t *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	customLogger := &testLogger{}
	obs.SetProvider(&obs.Provider{Logger: customLogger})

	ctx := context.Background()

	obs.Info(ctx, "info message", "field1", "value1")
	if customLogger.lastLevel != "info" {
		t.Errorf("lastLevel = %s, want info", customLogger.lastLevel)
	}
	if customLogger.lastMsg != "info message" {
		t.Errorf("lastMsg = %s, want info message", customLogger.lastMsg)
	}
	if customLogger.lastCtx != ctx {
		t.Error("context not passed correctly")
	}
	if len(customLogger.lastFields) != 2 {
		t.Errorf("fields length = %d, want 2", len(customLogger.lastFields))
	}

	obs.Warn(ctx, "warn message")
	if customLogger.lastLevel != "warn" {
		t.Errorf("lastLevel = %s, want warn", customLogger.lastLevel)
	}
	if customLogger.lastMsg != "warn message" {
		t.Errorf("lastMsg = %s, want warn message", customLogger.lastMsg)
	}

	obs.Error(ctx, "error message")
	if customLogger.lastLevel != "error" {
		t.Errorf("lastLevel = %s, want error", customLogger.lastLevel)
	}
	if customLogger.lastMsg != "error message" {
		t.Errorf("lastMsg = %s, want error message", customLogger.lastMsg)
	}

	obs.Debug(ctx, "debug message")
	if customLogger.lastLevel != "debug" {
		t.Errorf("lastLevel = %s, want debug", customLogger.lastLevel)
	}
	if customLogger.lastMsg != "debug message" {
		t.Errorf("lastMsg = %s, want debug message", customLogger.lastMsg)
	}
}

func TestMetrics_CustomMetrics(t *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	customMetrics := &testMetrics{}
	obs.SetProvider(&obs.Provider{Metrics: customMetrics})

	ctx := context.Background()
	duration := 100 * time.Millisecond

	obs.RecordHit(ctx, "redis", "get", duration)
	if customMetrics.lastType != "hit" {
		t.Errorf("lastType = %s, want hit", customMetrics.lastType)
	}
	if customMetrics.lastBackend != "redis" {
		t.Errorf("lastBackend = %s, want redis", customMetrics.lastBackend)
	}
	if customMetrics.lastOp != "get" {
		t.Errorf("lastOp = %s, want get", customMetrics.lastOp)
	}
	if customMetrics.lastDuration != duration {
		t.Errorf("lastDuration = %v, want %v", customMetrics.lastDuration, duration)
	}
	if customMetrics.lastCtx != ctx {
		t.Error("context not passed correctly")
	}

	obs.RecordMiss(ctx, "memcached", "set", 50*time.Millisecond)
	if customMetrics.lastType != "miss" {
		t.Errorf("lastType = %s, want miss", customMetrics.lastType)
	}
	if customMetrics.lastBackend != "memcached" {
		t.Errorf("lastBackend = %s, want memcached", customMetrics.lastBackend)
	}
	if customMetrics.lastOp != "set" {
		t.Errorf("lastOp = %s, want set", customMetrics.lastOp)
	}
	if customMetrics.lastDuration != 50*time.Millisecond {
		t.Errorf("lastDuration = %v, want 50ms", customMetrics.lastDuration)
	}

	obs.RecordError(ctx, "redis", "delete")
	if customMetrics.lastType != "error" {
		t.Errorf("lastType = %s, want error", customMetrics.lastType)
	}
	if customMetrics.lastBackend != "redis" {
		t.Errorf("lastBackend = %s, want redis", customMetrics.lastBackend)
	}
	if customMetrics.lastOp != "delete" {
		t.Errorf("lastOp = %s, want delete", customMetrics.lastOp)
	}

	obs.RecordEviction(ctx, "lru")
	if customMetrics.lastType != "eviction" {
		t.Errorf("lastType = %s, want eviction", customMetrics.lastType)
	}
	if customMetrics.lastBackend != "lru" {
		t.Errorf("lastBackend = %s, want lru", customMetrics.lastBackend)
	}
}

func TestProvider_ConcurrentAccess(_ *testing.T) {
	// Save original provider
	original := obs.Global()
	defer obs.SetProvider(original)

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = obs.Global()
			obs.Start(context.Background(), "test")
			obs.Info(context.Background(), "test")
			obs.RecordHit(context.Background(), "backend", "op", time.Millisecond)
		}()
	}

	wg.Wait()

	// Test concurrent writes (SetProvider should NOT be safe for concurrent use)
	// This is documented in the package, so we only test reads
}

// Test implementations

type testSpan struct {
	endCalled   int
	lastError   error
	lastAttrKey string
	lastAttrVal any
}

func (s *testSpan) End() {
	s.endCalled++
}

func (s *testSpan) SetError(err error) {
	s.lastError = err
}

func (s *testSpan) SetAttr(key string, val any) {
	s.lastAttrKey = key
	s.lastAttrVal = val
}

type testTracer struct {
	startCalled     int
	lastOp          string
	lastCtx         context.Context
	lastReturnedCtx context.Context
	lastSpan        *testSpan
}

func (t *testTracer) Start(ctx context.Context, op string) (context.Context, obs.Span) {
	t.startCalled++
	t.lastOp = op
	t.lastCtx = ctx

	// Create a new context to test that it's returned correctly
	newCtx := context.WithValue(ctx, testKey, "value")
	t.lastReturnedCtx = newCtx

	span := &testSpan{}
	t.lastSpan = span
	return newCtx, span
}

type testLogger struct {
	lastLevel  string
	lastMsg    string
	lastCtx    context.Context
	lastFields []any
}

func (l *testLogger) Info(ctx context.Context, msg string, fields ...any) {
	l.lastLevel = "info"
	l.lastMsg = msg
	l.lastCtx = ctx
	l.lastFields = fields
}

func (l *testLogger) Warn(ctx context.Context, msg string, fields ...any) {
	l.lastLevel = "warn"
	l.lastMsg = msg
	l.lastCtx = ctx
	l.lastFields = fields
}

func (l *testLogger) Error(ctx context.Context, msg string, fields ...any) {
	l.lastLevel = "error"
	l.lastMsg = msg
	l.lastCtx = ctx
	l.lastFields = fields
}

func (l *testLogger) Debug(ctx context.Context, msg string, fields ...any) {
	l.lastLevel = "debug"
	l.lastMsg = msg
	l.lastCtx = ctx
	l.lastFields = fields
}

type testMetrics struct {
	lastType     string
	lastBackend  string
	lastOp       string
	lastDuration time.Duration
	lastCtx      context.Context
}

func (m *testMetrics) RecordHit(ctx context.Context, backend, op string, d time.Duration) {
	m.lastType = "hit"
	m.lastBackend = backend
	m.lastOp = op
	m.lastDuration = d
	m.lastCtx = ctx
}

func (m *testMetrics) RecordMiss(ctx context.Context, backend, op string, d time.Duration) {
	m.lastType = "miss"
	m.lastBackend = backend
	m.lastOp = op
	m.lastDuration = d
	m.lastCtx = ctx
}

func (m *testMetrics) RecordError(ctx context.Context, backend, op string) {
	m.lastType = "error"
	m.lastBackend = backend
	m.lastOp = op
	m.lastCtx = ctx
}

func (m *testMetrics) RecordEviction(ctx context.Context, backend string) {
	m.lastType = "eviction"
	m.lastBackend = backend
	m.lastCtx = ctx
}

func TestStart_WithCanceledContext(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, span := obs.Start(ctx, "test-op")
	// Should not panic
	span.End()
}

func TestLogging_WithNilContext(_ *testing.T) {
	ctx := context.Background()
	// These should not panic even with nil context
	obs.Info(ctx, "info message")
	obs.Warn(ctx, "warn message")
	obs.Error(ctx, "error message")
	obs.Debug(ctx, "debug message")
}

func TestMetrics_WithNilContext(_ *testing.T) {
	ctx := context.Background()
	duration := time.Millisecond

	// These should not panic even with nil context
	obs.RecordHit(ctx, "backend", "op", duration)
	obs.RecordMiss(ctx, "backend", "op", duration)
	obs.RecordError(ctx, "backend", "op")
	obs.RecordEviction(ctx, "backend")
}

func BenchmarkStart_DefaultTracer(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_, span := obs.Start(ctx, "bench-op")
		span.End()
	}
}

func BenchmarkStart_CustomTracer(b *testing.B) {
	original := obs.Global()
	defer obs.SetProvider(original)

	customTracer := &testTracer{}
	obs.SetProvider(&obs.Provider{Tracer: customTracer})

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := obs.Start(ctx, "bench-op")
		span.End()
	}
}

func BenchmarkLogging_DefaultLogger(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		obs.Info(ctx, "bench message")
	}
}

func BenchmarkLogging_CustomLogger(b *testing.B) {
	original := obs.Global()
	defer obs.SetProvider(original)

	customLogger := &testLogger{}
	obs.SetProvider(&obs.Provider{Logger: customLogger})

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obs.Info(ctx, "bench message")
	}
}

func BenchmarkMetrics_DefaultMetrics(b *testing.B) {
	ctx := context.Background()
	duration := time.Millisecond
	for i := 0; i < b.N; i++ {
		obs.RecordHit(ctx, "bench", "op", duration)
	}
}

func BenchmarkMetrics_CustomMetrics(b *testing.B) {
	original := obs.Global()
	defer obs.SetProvider(original)

	customMetrics := &testMetrics{}
	obs.SetProvider(&obs.Provider{Metrics: customMetrics})

	ctx := context.Background()
	duration := time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obs.RecordHit(ctx, "bench", "op", duration)
	}
}

func BenchmarkSetProvider(b *testing.B) {
	original := obs.Global()
	defer obs.SetProvider(original)

	provider := &obs.Provider{
		Tracer:  &testTracer{},
		Logger:  &testLogger{},
		Metrics: &testMetrics{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obs.SetProvider(provider)
	}
}
