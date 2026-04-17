package memory

import (
	"context"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// LRU eviction
// ---------------------------------------------------------------------------

func TestEviction_LRU(t *testing.T) {
	const maxEntries = 10
	c, err := New(
		WithMaxEntries(maxEntries),
		WithCleanupInterval(0),
		WithLRU(),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Fill the cache with keys 0..9.
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		if err := c.Set(ctx, key, []byte{byte(i)}, 0); err != nil {
			t.Fatalf("Set(%s) error = %v", key, err)
		}
	}

	// Access 'a' and 'b' so they are recently used.
	_, _ = c.Get(ctx, "a")
	_, _ = c.Get(ctx, "b")

	// Insert one more key to trigger eviction.
	if err := c.Set(ctx, "x", []byte("overflow"), 0); err != nil {
		t.Fatalf("Set(x) error = %v", err)
	}

	// 'c' was the least recently used (after 'a' and 'b' were touched).
	// The eviction picks victims from all shards, so we need to check that
	// one of the older items was evicted while 'a' and 'b' survived.
	size, _ := c.Size(ctx)
	if size > int64(maxEntries) {
		t.Errorf("size after overflow = %d, want <= %d", size, maxEntries)
	}

	// 'a' and 'b' should still exist (they were recently accessed).
	for _, key := range []string{"a", "b"} {
		_, err := c.Get(ctx, key)
		if err != nil {
			t.Errorf("recently accessed key %q was evicted", key)
		}
	}

	// At least one older key (c, d, e, ... j) should have been evicted.
	evicted := false
	for i := 2; i < maxEntries; i++ { // c=2 .. j=9
		key := string(rune('a' + i))
		_, err := c.Get(ctx, key)
		if err != nil {
			evicted = true
			break
		}
	}
	if !evicted {
		t.Error("expected at least one older key to be evicted under LRU")
	}
}

// ---------------------------------------------------------------------------
// LFU eviction
// ---------------------------------------------------------------------------

func TestEviction_LFU(t *testing.T) {
	const maxEntries = 10
	c, err := New(
		WithMaxEntries(maxEntries),
		WithCleanupInterval(0),
		WithLFU(),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Fill cache with keys 0..9.
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		_ = c.Set(ctx, key, []byte{byte(i)}, 0)
	}

	// Access 'a' and 'b' many times to increase their frequency.
	for i := 0; i < 50; i++ {
		_, _ = c.Get(ctx, "a")
		_, _ = c.Get(ctx, "b")
	}

	// Insert overflow.
	_ = c.Set(ctx, "x", []byte("overflow"), 0)

	size, _ := c.Size(ctx)
	if size > int64(maxEntries) {
		t.Errorf("size after overflow = %d, want <= %d", size, maxEntries)
	}

	// High-frequency keys should survive.
	for _, key := range []string{"a", "b"} {
		_, err := c.Get(ctx, key)
		if err != nil {
			t.Errorf("high-frequency key %q was evicted", key)
		}
	}
}

// ---------------------------------------------------------------------------
// FIFO eviction
// ---------------------------------------------------------------------------

func TestEviction_FIFO(t *testing.T) {
	const maxEntries = 10
	c, err := New(
		WithMaxEntries(maxEntries),
		WithCleanupInterval(0),
		WithFIFO(),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Fill cache in order.
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		_ = c.Set(ctx, key, []byte{byte(i)}, 0)
	}

	// The first key added ('a') should be evicted when we add an overflow.
	_ = c.Set(ctx, "x", []byte("overflow"), 0)

	_, err = c.Get(ctx, "a")
	if err == nil {
		t.Error("expected first-added key 'a' to be evicted under FIFO")
	}

	// Last-added keys should survive.
	for _, key := range []string{"j", "x"} {
		_, err := c.Get(ctx, key)
		if err != nil {
			t.Errorf("recently-added key %q was evicted", key)
		}
	}
}

// ---------------------------------------------------------------------------
// TinyLFU eviction
// ---------------------------------------------------------------------------

func TestEviction_TinyLFU(t *testing.T) {
	const maxEntries = 10
	c, err := New(
		WithMaxEntries(maxEntries),
		WithCleanupInterval(0),
		WithTinyLFU(),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Fill cache.
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		_ = c.Set(ctx, key, []byte{byte(i)}, 0)
	}

	// Access 'a' frequently.
	for i := 0; i < 50; i++ {
		_, _ = c.Get(ctx, "a")
	}

	// Trigger eviction.
	_ = c.Set(ctx, "x", []byte("overflow"), 0)

	size, _ := c.Size(ctx)
	if size > int64(maxEntries) {
		t.Errorf("size after overflow = %d, want <= %d", size, maxEntries)
	}
}

// ---------------------------------------------------------------------------
// OnEviction callback
// ---------------------------------------------------------------------------

func TestOnEvictionCallback(t *testing.T) {
	const maxEntries = 5
	var mu sync.Mutex
	evictedKeys := make(map[string]string)

	cb := func(key, reason string) {
		mu.Lock()
		evictedKeys[key] = reason
		mu.Unlock()
	}

	c, err := New(
		WithMaxEntries(maxEntries),
		WithCleanupInterval(0),
		WithOnEvictionPolicy(cb),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Fill cache to capacity.
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		_ = c.Set(ctx, key, []byte{byte(i)}, 0)
	}

	// Insert one more to trigger eviction.
	_ = c.Set(ctx, "overflow", []byte("x"), 0)

	mu.Lock()
	if len(evictedKeys) == 0 {
		mu.Unlock()
		t.Fatal("expected eviction callback to be called, but it wasn't")
	}
	// Verify at least one key was evicted with reason "capacity".
	for key, reason := range evictedKeys {
		if reason != "capacity" {
			t.Errorf("evicted key %q: reason = %q, want %q", key, reason, "capacity")
		}
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// Eviction respects MaxMemoryBytes
// ---------------------------------------------------------------------------

func TestEviction_MaxMemoryBytes(t *testing.T) {
	// 1 KB max, 1-byte entries, max 1 entry effectively.
	c, err := New(
		WithMaxEntries(1000),
		WithMaxMemoryBytes(200), // very small
		WithCleanupInterval(0),
		WithLRU(),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Insert a few items; the cache should enforce memory limits.
	for i := 0; i < 20; i++ {
		key := string(rune('A' + i))
		_ = c.Set(ctx, key, make([]byte, 50), 0) // each entry ~50 bytes key+value
	}

	size, _ := c.Size(ctx)
	if size > 5 { // generous upper bound given 200 byte max
		t.Errorf("with 200 byte max, size = %d, expected much smaller", size)
	}
}

// ---------------------------------------------------------------------------
// Table-driven shorthand eviction policies
// ---------------------------------------------------------------------------

func TestEvictionPolicies_Sanity(t *testing.T) {
	policies := []struct {
		name   string
		option Option
	}{
		{"LRU", WithLRU()},
		{"LFU", WithLFU()},
		{"FIFO", WithFIFO()},
		{"LIFO", WithLIFO()},
		{"MRU", WithMRU()},
		{"Random", WithRandom()},
		{"TinyLFU", WithTinyLFU()},
	}

	for _, tc := range policies {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(
				WithMaxEntries(5),
				WithCleanupInterval(0),
				tc.option,
			)
			if err != nil {
				t.Fatalf("New(%s) error = %v", tc.name, err)
			}
			defer c.Close(context.Background())
			ctx := context.Background()

			// Insert items beyond capacity.
			for i := 0; i < 10; i++ {
				_ = c.Set(ctx, string(rune('0'+i)), []byte{byte(i)}, 0)
			}

			// Cache size should not exceed max.
			size, _ := c.Size(ctx)
			if size > 5 {
				t.Errorf("policy %s: size = %d, want <= 5", tc.name, size)
			}
		})
	}
}
