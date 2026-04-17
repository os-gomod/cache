package eviction

import (
	"testing"

	"github.com/os-gomod/cache/config"
)

func TestNew_DefaultPolicy(t *testing.T) {
	e := New(config.EvictLRU, 1024)
	if e == nil {
		t.Fatal("New returned nil")
	}
}

func TestLRU_EvictOrder(t *testing.T) {
	e := newLRU(1024)
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		e.OnAdd(key, NewEntry(key, []byte{byte(i)}, 0, 0))
	}
	// Access 'a' and 'b' to move them to front
	e.OnAccess("a", nil)
	e.OnAccess("b", nil)
	// Evict 3: should remove c, d, e (least recently used)
	victims := e.Evict(3)
	if len(victims) != 3 {
		t.Fatalf("expected 3 victims, got %d", len(victims))
	}
	victimSet := make(map[string]bool)
	for _, v := range victims {
		victimSet[v] = true
	}
	for _, k := range []string{"c", "d", "e"} {
		if !victimSet[k] {
			t.Errorf("expected %q to be evicted", k)
		}
	}
}

func TestLRU_AccessMovesToFront(t *testing.T) {
	e := newLRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnAccess("a", nil)
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "b" {
		t.Errorf("expected 'b' evicted, got %v", victims)
	}
}

func TestLRU_Delete(t *testing.T) {
	e := newLRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnDelete("a")
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 victims after delete, got %d", len(victims))
	}
}

func TestLRU_Reset(t *testing.T) {
	e := newLRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 victims after reset, got %d", len(victims))
	}
}

func TestLFU_EvictOrder(t *testing.T) {
	e := newLFU(1024)
	// Create entries with different hit counts
	a := NewEntry("a", nil, 0, 0)
	b := NewEntry("b", nil, 0, 0)
	c := NewEntry("c", nil, 0, 0)
	a.IncrHits()
	a.IncrHits() // a has 3 hits
	b.IncrHits() // b has 2 hits
	// c has 1 hit (default)
	e.OnAdd("a", a)
	e.OnAdd("b", b)
	e.OnAdd("c", c)
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "c" {
		t.Errorf("expected 'c' (lowest freq) evicted, got %v", victims)
	}
}

func TestLFU_Reset(t *testing.T) {
	e := newLFU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestFIFO_EvictOrder(t *testing.T) {
	e := newFIFO(1024)
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		e.OnAdd(key, NewEntry(key, nil, 0, 0))
	}
	victims := e.Evict(2)
	if len(victims) != 2 {
		t.Fatalf("expected 2 victims, got %d", len(victims))
	}
	if victims[0] != "a" || victims[1] != "b" {
		t.Errorf("expected [a b] evicted, got %v", victims)
	}
}

func TestFIFO_AccessDoesNotMove(t *testing.T) {
	e := newFIFO(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnAccess("a", nil) // FIFO ignores access
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "a" {
		t.Errorf("expected 'a' evicted (FIFO), got %v", victims)
	}
}

func TestLIFO_EvictOrder(t *testing.T) {
	e := newLIFO(1024)
	for i := 0; i < 3; i++ {
		key := string(rune('a' + i))
		e.OnAdd(key, NewEntry(key, nil, 0, 0))
	}
	victims := e.Evict(1)
	if len(victims) != 1 || victims[0] != "c" {
		t.Errorf("expected 'c' (last in) evicted, got %v", victims)
	}
}

func TestMRU_EvictOrder(t *testing.T) {
	e := newMRU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.OnAdd("b", NewEntry("b", nil, 0, 0))
	e.OnAccess("a", nil) // move 'a' to front
	victims := e.Evict(1)
	// MRU evicts from front (most recently used)
	if len(victims) != 1 || victims[0] != "a" {
		t.Errorf("expected 'a' (most recently used) evicted, got %v", victims)
	}
}

func TestRandom_Evict(t *testing.T) {
	e := newRandom(1024)
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		e.OnAdd(key, NewEntry(key, nil, 0, 0))
	}
	victims := e.Evict(3)
	if len(victims) != 3 {
		t.Fatalf("expected 3 victims, got %d", len(victims))
	}
}

func TestTinyLFU_Evict(t *testing.T) {
	e := newTinyLFU(1024)
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		entry := NewEntry(key, nil, 0, 0)
		// Set different frequencies
		for j := 0; j < i; j++ {
			entry.IncrFrequency()
		}
		e.OnAdd(key, entry)
	}
	victims := e.Evict(5)
	if len(victims) != 5 {
		t.Fatalf("expected 5 victims, got %d", len(victims))
	}
}

func TestTinyLFU_Reset(t *testing.T) {
	e := newTinyLFU(1024)
	e.OnAdd("a", NewEntry("a", nil, 0, 0))
	e.Reset()
	victims := e.Evict(1)
	if len(victims) != 0 {
		t.Errorf("expected 0 after reset, got %d", len(victims))
	}
}

func TestNew_AllPolicies(t *testing.T) {
	policies := []config.EvictionPolicy{
		config.EvictLRU, config.EvictLFU, config.EvictFIFO,
		config.EvictLIFO, config.EvictMRU, config.EvictRR, config.EvictTinyLFU,
	}
	for _, p := range policies {
		e := New(p, 1024)
		if e == nil {
			t.Errorf("New(%v) returned nil", p)
		}
	}
}
