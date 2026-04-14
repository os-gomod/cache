package eviction

import (
	"testing"
	"time"

	"github.com/os-gomod/cache/config"
)

func TestNew_LRUDefault(t *testing.T) {
	e := New(config.EvictLRU, 1024)
	if _, ok := e.(*lruEvictor); !ok {
		t.Error("expected LRU evictor")
	}
}

func TestNew_LFU(t *testing.T) {
	e := New(config.EvictLFU, 1024)
	if _, ok := e.(*lfuEvictor); !ok {
		t.Error("expected LFU evictor")
	}
}

func TestNew_FIFO(t *testing.T) {
	e := New(config.EvictFIFO, 1024)
	if _, ok := e.(*fifoEvictor); !ok {
		t.Error("expected FIFO evictor")
	}
}

func TestNew_LIFO(t *testing.T) {
	e := New(config.EvictLIFO, 1024)
	if _, ok := e.(*lifoEvictor); !ok {
		t.Error("expected LIFO evictor")
	}
}

func TestNew_MRU(t *testing.T) {
	e := New(config.EvictMRU, 1024)
	if _, ok := e.(*mruEvictor); !ok {
		t.Error("expected MRU evictor")
	}
}

func TestNew_Random(t *testing.T) {
	e := New(config.EvictRR, 1024)
	if _, ok := e.(*randomEvictor); !ok {
		t.Error("expected random evictor")
	}
}

func TestNew_TinyLFU(t *testing.T) {
	e := New(config.EvictTinyLFU, 1024)
	if _, ok := e.(*tinyLFUEvictor); !ok {
		t.Error("expected TinyLFU evictor")
	}
}

func TestLRU_Evict(t *testing.T) {
	e := New(config.EvictLRU, 0)

	entry1 := NewEntry("key1", []byte("val1"), 0, 0)
	entry2 := NewEntry("key2", []byte("val2"), 0, 0)
	entry3 := NewEntry("key3", []byte("val3"), 0, 0)

	e.OnAdd("key1", entry1)
	e.OnAdd("key2", entry2)
	e.OnAdd("key3", entry3)

	// Access key1 to make it recently used
	e.OnAccess("key1", entry1)

	victims := e.Evict(1)
	if len(victims) != 1 {
		t.Fatalf("expected 1 victim, got %d", len(victims))
	}
	// key2 should be evicted (least recently used)
	if victims[0] != "key2" {
		t.Errorf("evicted %q, want %q", victims[0], "key2")
	}
}

func TestLRU_OnDelete(t *testing.T) {
	e := New(config.EvictLRU, 0)

	entry := NewEntry("key1", []byte("val1"), 0, 0)
	e.OnAdd("key1", entry)
	e.OnDelete("key1")

	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 victims after delete, got %d", len(victims))
	}
}

func TestLRU_Reset(t *testing.T) {
	e := New(config.EvictLRU, 0)

	entry := NewEntry("key1", []byte("val1"), 0, 0)
	e.OnAdd("key1", entry)
	e.Reset()

	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 victims after reset, got %d", len(victims))
	}
}

func TestFIFO_Evict(t *testing.T) {
	e := New(config.EvictFIFO, 0)

	e.OnAdd("first", NewEntry("first", []byte("1"), 0, 0))
	e.OnAdd("second", NewEntry("second", []byte("2"), 0, 0))
	e.OnAdd("third", NewEntry("third", []byte("3"), 0, 0))

	victims := e.Evict(2)
	if len(victims) != 2 {
		t.Fatalf("expected 2 victims, got %d", len(victims))
	}
	// FIFO: first items added should be evicted first
	if victims[0] != "first" {
		t.Errorf("first victim = %q, want %q", victims[0], "first")
	}
	if victims[1] != "second" {
		t.Errorf("second victim = %q, want %q", victims[1], "second")
	}
}

func TestFIFO_OnAccess_NoReorder(t *testing.T) {
	e := New(config.EvictFIFO, 0)

	e.OnAdd("first", NewEntry("first", []byte("1"), 0, 0))
	e.OnAdd("second", NewEntry("second", []byte("2"), 0, 0))

	// Accessing first should NOT change FIFO order
	e.OnAccess("first", NewEntry("first", []byte("1"), 0, 0))

	victims := e.Evict(1)
	if victims[0] != "first" {
		t.Errorf("FIFO should not reorder on access, victim = %q", victims[0])
	}
}

// Entry tests

func TestNewEntry(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 5*time.Minute, 0)

	if e.Key != "key1" {
		t.Errorf("Key = %q, want %q", e.Key, "key1")
	}
	if string(e.Value) != "val1" {
		t.Errorf("Value = %q, want %q", e.Value, "val1")
	}
	if e.Hits != 1 {
		t.Errorf("Hits = %d, want 1", e.Hits)
	}
	if e.Size <= 0 {
		t.Error("Size should be positive")
	}
}

func TestEntry_IsExpired(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 0, 0) // TTL=0 means never expires
	if e.IsExpired() {
		t.Error("entry with TTL=0 should not expire")
	}

	e2 := NewEntry("key2", []byte("val2"), -1, 0) // Negative TTL
	// calcExpiry returns error for negative TTL, ExpiresAt stays 0
	if e2.IsExpired() {
		t.Error("entry with failed expiry calc should not report expired")
	}
}

func TestEntry_Touch(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 0, 0)
	hitsBefore := e.GetHits()
	e.Touch()
	if e.GetHits() != hitsBefore+1 {
		t.Errorf("Hits after Touch = %d, want %d", e.GetHits(), hitsBefore+1)
	}
}

func TestEntry_WithNewTTL(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 5*time.Minute, 0)
	e2 := e.WithNewTTL(10 * time.Minute)

	if e2.Key != e.Key {
		t.Error("WithNewTTL should preserve key")
	}
	if string(e2.Value) != string(e.Value) {
		t.Error("WithNewTTL should preserve value")
	}
}

func TestAcquireReleaseEntry(t *testing.T) {
	e := AcquireEntry("pool-key", []byte("pool-val"), time.Minute, 0)
	if e.Key != "pool-key" {
		t.Errorf("Key = %q, want %q", e.Key, "pool-key")
	}
	ReleaseEntry(e)

	// Should be able to acquire again after release
	e2 := AcquireEntry("pool-key2", []byte("pool-val2"), time.Minute, 0)
	_ = e2
	ReleaseEntry(e2)
}

func TestEntry_SetFrequency(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 0, 0)
	e.SetFrequency(5)
	if e.GetFrequency() != 5 {
		t.Errorf("Frequency = %d, want 5", e.GetFrequency())
	}
}

func TestEntry_IncrFrequency(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 0, 0)
	result := e.IncrFrequency()
	if result != 1 {
		t.Errorf("IncrFrequency = %d, want 1", result)
	}
}

func TestEntry_SetRecency(t *testing.T) {
	e := NewEntry("key1", []byte("val1"), 0, 0)
	e.SetRecency(42)
	if e.GetRecency() != 42 {
		t.Errorf("Recency = %d, want 42", e.GetRecency())
	}
}
