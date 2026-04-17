package eviction

import (
	"testing"
	"time"
)

func TestFIFO_Delete(t *testing.T) {
	e := newFIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted after deleting 'a', got %v", victims)
	}
}

func TestFIFO_Reset(t *testing.T) {
	e := newFIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestLIFO_Delete(t *testing.T) {
	e := newLIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnDelete("b")
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "a" {
		t.Errorf("expected 'a' evicted after deleting 'b', got %v", victims)
	}
}

func TestLIFO_AccessDoesNotMove(t *testing.T) {
	e := newLIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnAccess("a", nil) // LIFO ignores access
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted (LIFO), got %v", victims)
	}
}

func TestLIFO_Reset(t *testing.T) {
	e := newLIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestMRU_Delete(t *testing.T) {
	e := newMRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted after deleting 'a', got %v", victims)
	}
}

func TestMRU_Reset(t *testing.T) {
	e := newMRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestRandom_Delete(t *testing.T) {
	e := newRandom(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 1 {
		t.Errorf("expected 1 victim, got %d", len(victims))
	}
	if victims[0] != "b" {
		t.Errorf("expected 'b' evicted after deleting 'a', got %v", victims)
	}
}

func TestRandom_AccessDoesNotMove(t *testing.T) {
	e := newRandom(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnAccess("a", nil) // random ignores access
	victims := e.Evict(2)
	if len(victims) != 2 {
		t.Errorf("expected 2 victims, got %d", len(victims))
	}
}

func TestRandom_Reset(t *testing.T) {
	e := newRandom(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestTinyLFU_Access(t *testing.T) {
	e := newTinyLFU(1024)
	entryA := NewEntry("a", nil, 0, 0)
	entryA.SetFrequency(10)
	e.OnAdd("a", entryA)
	e.OnAccess("a", entryA)
}

func TestTinyLFU_Delete(t *testing.T) {
	e := newTinyLFU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted after deleting 'a', got %v", victims)
	}
}

func TestTinyLFU_DeleteNonexistent(t *testing.T) {
	e := newTinyLFU(1024)
	e.OnDelete("nonexistent") // should not panic
}

func TestTinyLFU_Evict_Zero(t *testing.T) {
	e := newTinyLFU(1024)
	victims := e.Evict(0)
	if len(victims) != 0 {
		t.Errorf("expected 0 victims, got %d", len(victims))
	}
}

func TestMapSliceEvictor(t *testing.T) {
	e := newMapSliceEvictor(1024)

	// Add keys
	e.addKey("a")
	e.addKey("b")
	e.addKey("c")
	if e.size != 3 {
		t.Errorf("size = %d, want 3", e.size)
	}

	// Duplicate add should be no-op
	e.addKey("a")
	if e.size != 3 {
		t.Errorf("size = %d after duplicate add, want 3", e.size)
	}

	// Remove middle key
	e.removeKey("b")
	if e.size != 2 {
		t.Errorf("size = %d after remove, want 2", e.size)
	}
	if _, exists := e.keyIndex["b"]; exists {
		t.Error("key 'b' should be removed from keyIndex")
	}

	// Remove nonexistent key should be no-op
	e.removeKey("nonexistent")
	if e.size != 2 {
		t.Errorf("size = %d after removing nonexistent, want 2", e.size)
	}

	// Remove last key
	e.removeKey("c")
	if e.size != 1 {
		t.Errorf("size = %d, want 1", e.size)
	}
}

func TestEntry_WithNewTTL_ZeroTTL(t *testing.T) {
	original := NewEntry("key", []byte("val"), time.Minute, 0)
	newEntry := original.WithNewTTL(0)
	if newEntry.SoftExpiresAt != 0 {
		t.Error("SoftExpiresAt should be 0 for zero TTL")
	}
	if newEntry.Key != "key" {
		t.Error("Key should be preserved")
	}
}

func TestLFU_AccessOnAdd(t *testing.T) {
	e := newLFU(1024)
	a := NewEntry("a", nil, 0, 0)
	a.IncrHits() // freq = 2
	e.OnAdd("a", a)

	// Update with higher frequency
	a.IncrHits() // freq = 3
	e.OnAccess("a", a)

	b := NewEntry("b", nil, 0, 0)
	e.OnAdd("b", b)

	// b should be evicted (lower frequency)
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted, got %v", victims)
	}
}

func TestLFU_Delete(t *testing.T) {
	e := newLFU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(victims))
	}
}
