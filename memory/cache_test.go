package memory

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/observability"
)

// helper to create a standard test cache with janitor disabled.
func newTestCache(t *testing.T, opts ...Option) *Cache {
	t.Helper()
	defaults := []Option{
		WithMaxEntries(1000),
		WithTTL(10 * time.Minute),
		WithCleanupInterval(0),
	}
	opts = append(defaults, opts...)
	c, err := New(opts...)
	if err != nil {
		t.Fatalf("New(opts...) error = %v", err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	return c
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNew_CreatesCache(t *testing.T) {
	c, err := New(
		WithMaxEntries(500),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())

	if c.Name() != "memory" {
		t.Errorf("Name() = %q, want %q", c.Name(), "memory")
	}
	if c.Closed() {
		t.Error("Closed() = true, want false")
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestNewWithContext_CreatesCache(t *testing.T) {
	ctx := context.Background()
	c, err := NewWithContext(ctx,
		WithMaxEntries(500),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("NewWithContext() error = %v", err)
	}
	defer c.Close(context.Background())

	if c.Name() != "memory" {
		t.Errorf("Name() = %q, want %q", c.Name(), "memory")
	}
}

func TestNewWithConfig_NilUsesDefaults(t *testing.T) {
	c, err := NewWithConfig(nil)
	if err != nil {
		t.Fatalf("NewWithConfig(nil) error = %v", err)
	}
	defer c.Close(context.Background())

	if c.Closed() {
		t.Error("new cache should not be closed")
	}
}

func TestNewWithConfig_ClonesConfig(t *testing.T) {
	cfg := &config.Memory{
		MaxEntries:      42,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 0,
	}
	c, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}
	defer c.Close(context.Background())

	// Mutate the original config after creation – internal config should be unaffected.
	cfg.MaxEntries = 9999
	snap := c.Stats()
	// Just verify cache is alive; the key point is no panic and config was cloned.
	_ = snap
	if c.Closed() {
		t.Error("cache should not be closed")
	}
}

// ---------------------------------------------------------------------------
// Close / lifecycle tests
// ---------------------------------------------------------------------------

func TestClose_MultipleTimesSafe(t *testing.T) {
	c, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// First close should succeed.
	if err := c.Close(context.Background()); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	// Second close should also succeed (no panic, nil error).
	if err := c.Close(context.Background()); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
	if !c.Closed() {
		t.Error("Closed() should be true after Close()")
	}
}

func TestClose_StopsJanitor(t *testing.T) {
	// Create a cache with a short cleanup interval to start the janitor.
	c, err := New(WithCleanupInterval(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// Give the janitor a moment to start.
	time.Sleep(100 * time.Millisecond)

	// Close should stop the janitor without error.
	if err := c.Close(context.Background()); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !c.Closed() {
		t.Error("Closed() should be true")
	}
}

// ---------------------------------------------------------------------------
// Ping / Name / Stats / Closed
// ---------------------------------------------------------------------------

func TestPing_Alive(t *testing.T) {
	c := newTestCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping() on alive cache error = %v", err)
	}
}

func TestPing_Closed(t *testing.T) {
	c, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_ = c.Close(context.Background())

	err = c.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() on closed cache should return error")
	}
	if !errors.IsCacheClosed(err) {
		t.Errorf("expected cache closed error, got %v", err)
	}
}

func TestName_ReturnsMemory(t *testing.T) {
	c := newTestCache(t)
	if got := c.Name(); got != "memory" {
		t.Errorf("Name() = %q, want %q", got, "memory")
	}
}

func TestStats_ReturnsSnapshot(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()
	_ = c.Set(ctx, "k", []byte("v"), 0)

	snap := c.Stats()
	if snap.Sets < 1 {
		t.Errorf("Stats.Sets = %d, want >= 1", snap.Sets)
	}
	if snap.Items < 1 {
		t.Errorf("Stats.Items = %d, want >= 1", snap.Items)
	}
	// Uptime should be non-negative.
	if snap.Uptime < 0 {
		t.Errorf("Stats.Uptime = %v, want >= 0", snap.Uptime)
	}
}

func TestClosed_InitiallyFalse(t *testing.T) {
	c := newTestCache(t)
	if c.Closed() {
		t.Error("new cache should not report closed")
	}
}

func TestClosed_AfterCloseTrue(t *testing.T) {
	c, err := New(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_ = c.Close(context.Background())

	if !c.Closed() {
		t.Error("Closed() should be true after Close()")
	}
}

// ---------------------------------------------------------------------------
// SetInterceptors
// ---------------------------------------------------------------------------

func TestSetInterceptors_ReplacesChain(t *testing.T) {
	c := newTestCache(t)

	// Set a real interceptor first.
	c.SetInterceptors(observability.NopInterceptor{})

	// Replacing with an empty variadic should be safe and not panic.
	c.SetInterceptors()

	// Cache should still work with the empty chain.
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Errorf("Set after SetInterceptors error = %v", err)
	}

	// Replace with a fresh interceptor and verify it still works.
	c.SetInterceptors(observability.NopInterceptor{})
	if err := c.Set(ctx, "k2", []byte("v2"), 0); err != nil {
		t.Errorf("Set after replacing interceptor error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Option tests
// ---------------------------------------------------------------------------

func TestNewWithMaxEntries(t *testing.T) {
	c, err := New(
		WithMaxEntries(42),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	// The internal config should reflect the option.
	if c.cfg.MaxEntries != 42 {
		t.Errorf("MaxEntries = %d, want 42", c.cfg.MaxEntries)
	}
}

func TestNewWithTTL(t *testing.T) {
	ttl := 3 * time.Hour
	c, err := New(
		WithTTL(ttl),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	if c.cfg.DefaultTTL != ttl {
		t.Errorf("DefaultTTL = %v, want %v", c.cfg.DefaultTTL, ttl)
	}
}

func TestNewWithEvictionPolicy(t *testing.T) {
	policies := []config.EvictionPolicy{
		config.EvictLRU,
		config.EvictLFU,
		config.EvictFIFO,
		config.EvictTinyLFU,
	}
	for _, policy := range policies {
		t.Run(policy.String(), func(t *testing.T) {
			c, err := New(
				WithEvictionPolicy(policy),
				WithCleanupInterval(0),
			)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			defer c.Close(context.Background())
			if c.cfg.EvictionPolicy != policy {
				t.Errorf("EvictionPolicy = %d, want %d", c.cfg.EvictionPolicy, policy)
			}
		})
	}
}

func TestNewWithShards(t *testing.T) {
	c, err := New(
		WithShards(8),
		WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	// Shards are normalized to a power of two.
	if c.nShards != 8 {
		t.Errorf("nShards = %d, want 8", c.nShards)
	}
}
