// Package testutil provides test helpers, mock backends, and assertion
// utilities for cache backend testing. It depends on the testing package
// for the canonical Backend interface.
package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/memory"
)

// Backend is an alias for the canonical backend interface.
type Backend = backendiface.Backend

// NewMemoryBackend creates a new in-memory cache backend for testing.
// The backend is automatically closed when the test finishes.
func NewMemoryBackend(t *testing.T) Backend {
	t.Helper()
	c, err := memory.New()
	if err != nil {
		t.Fatalf("testutil: failed to create memory backend: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// CallRecord records a single cache operation for verification in tests.
type CallRecord struct {
	Op    string
	Key   string
	Value []byte
	TTL   time.Duration
	Err   error
}

// MockBackend wraps a real backend and records all operations for test assertions.
type MockBackend struct {
	Backend
	mu     sync.Mutex
	Calls  []CallRecord
	closed bool
}

// NewMockBackend creates a new mock backend backed by an in-memory cache.
// It records all Get/Set/Delete operations for later inspection via GetCalls.
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

// Close closes the underlying backend and marks the mock as closed.
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

// GetCalls returns a copy of all recorded operations.
func (m *MockBackend) GetCalls() []CallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CallRecord, len(m.Calls))
	copy(out, m.Calls)
	return out
}

// AssertKeyExists fails the test if the key does not exist in the backend.
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

// AssertKeyNotFound fails the test if the key exists in the backend.
func AssertKeyNotFound(t *testing.T, b Backend, key string) {
	t.Helper()
	_, err := b.Get(context.Background(), key)
	if err == nil {
		t.Fatalf("AssertKeyNotFound: key %q should not exist but does", key)
	}
}

// AssertHitRate fails the test if the hit rate is below the minimum.
func AssertHitRate(t *testing.T, b Backend, minRate float64) {
	t.Helper()
	snap := b.Stats()
	rate := snap.HitRate / 100.0
	if rate < minRate {
		t.Fatalf("AssertHitRate: hit rate %.2f%% is below minimum %.2f%%", rate*100, minRate*100)
	}
}

// WaitGroupWithTimeout waits for the WaitGroup with a timeout, failing the test on timeout.
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
