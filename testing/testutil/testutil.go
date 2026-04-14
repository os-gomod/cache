// Package testutil provides shared test helpers for the cache library's
// enterprise testing suite. Every helper is designed to reduce boilerplate
// in contract, race, chaos, and benchmark tests.
package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/memory"
)

// Backend is a local interface that mirrors cache.Backend.
// Defined locally to avoid circular imports.
type Backend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Stats() stats.Snapshot
	Closed() bool
	Name() string
}

// NewMemoryBackend creates a fresh memory backend with sensible test defaults.
// The backend is automatically closed via t.Cleanup when the test finishes,
// preventing resource leaks even if the test panics.
func NewMemoryBackend(t *testing.T) Backend {
	t.Helper()
	c, err := memory.New()
	if err != nil {
		t.Fatalf("testutil: failed to create memory backend: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// CallRecord stores a single method call for later inspection in tests.
type CallRecord struct {
	Op    string
	Key   string
	Value []byte
	TTL   time.Duration
	Err   error
}

// MockBackend is a lightweight Backend implementation that records all calls.
// It delegates to an underlying memory backend for actual storage, but
// intercepts every call and appends it to the Calls slice for assertion.
// All access to Calls is protected by a mutex so the mock is safe for
// concurrent use in race tests.
type MockBackend struct {
	Backend // delegate to memory for real storage
	mu      sync.Mutex
	Calls   []CallRecord
	closed  bool
}

// NewMockBackend creates a MockBackend backed by a fresh memory cache.
// The underlying memory backend is cleaned up via t.Cleanup.
func NewMockBackend(t *testing.T) *MockBackend {
	t.Helper()
	c, err := memory.New()
	if err != nil {
		t.Fatalf("testutil: failed to create mock backend: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return &MockBackend{Backend: c}
}

func (m *MockBackend) record(op, key string, value []byte, ttl time.Duration, err error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, CallRecord{Op: op, Key: key, Value: value, TTL: ttl, Err: err})
	m.mu.Unlock()
}

// Get delegates to the underlying backend and records the call.
func (m *MockBackend) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := m.Backend.Get(ctx, key)
	m.record("get", key, val, 0, err)
	return val, err
}

// Set delegates to the underlying backend and records the call.
func (m *MockBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := m.Backend.Set(ctx, key, value, ttl)
	m.record("set", key, value, ttl, err)
	return err
}

// Delete delegates to the underlying backend and records the call.
func (m *MockBackend) Delete(ctx context.Context, key string) error {
	err := m.Backend.Delete(ctx, key)
	m.record("delete", key, nil, 0, err)
	return err
}

// Close marks the mock as closed and delegates to the underlying backend.
func (m *MockBackend) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return m.Backend.Close(ctx)
}

// Closed reports whether the mock has been closed.
func (m *MockBackend) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// GetCalls returns a copy of all recorded calls. Safe for concurrent use.
func (m *MockBackend) GetCalls() []CallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CallRecord, len(m.Calls))
	copy(out, m.Calls)
	return out
}

// AssertKeyExists asserts that the given key exists in the backend. It
// fatals the test immediately on failure (no continued execution).
func AssertKeyExists(t *testing.T, b Backend, key string) {
	t.Helper()
	ok, err := b.Exists(context.Background(), key)
	if err != nil {
		t.Fatalf("AssertKeyExists: Exists(%q) returned error: %v", key, err)
	}
	if !ok {
		t.Fatalf("AssertKeyExists: key %q should exist but does not", key)
	}
}

// AssertKeyNotFound asserts that the given key does NOT exist in the backend.
// It fatals the test immediately on failure.
func AssertKeyNotFound(t *testing.T, b Backend, key string) {
	t.Helper()
	_, err := b.Get(context.Background(), key)
	if err == nil {
		t.Fatalf("AssertKeyNotFound: key %q should not exist but does", key)
	}
	if !_errors.IsNotFound(err) {
		t.Fatalf("AssertKeyNotFound: expected NotFound for key %q, got %v", key, err)
	}
}

// AssertHitRate asserts that the backend's hit rate is at least minRate (0.0–1.0).
// It retrieves the stats snapshot from the backend and checks HitRate / 100
// (since stats.HitRate returns a percentage 0–100).
func AssertHitRate(t *testing.T, b Backend, minRate float64) {
	t.Helper()
	snap := b.Stats()
	rate := snap.HitRate / 100.0
	if rate < minRate {
		t.Fatalf("AssertHitRate: hit rate %.2f%% is below minimum %.2f%%", rate*100, minRate*100)
	}
}

// WaitGroupWithTimeout waits for a sync.WaitGroup with a timeout. It fatals
// the test if the WaitGroup doesn't complete within the given duration,
// preventing tests from hanging indefinitely.
func WaitGroupWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Fatalf("WaitGroupWithTimeout: timed out after %v", timeout)
	}
}
