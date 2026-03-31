package eviction

import (
	"testing"
)

// func TestNew(t *testing.T) {
// 	tests := []struct {
// 		name   string
// 		policy config.EvictionPolicy
// 		want   string
// 	}{
// 		{"LRU", config.EvictLRU, "*EvictionPolicy.LRU"},
// 		{"LFU", config.EvictLFU, "*EvictionPolicy.LFU"},
// 		{"FIFO", config.EvictFIFO, "*EvictionPolicy.FIFO"},
// 		{"LIFO", config.EvictLIFO, "*EvictionPolicy.LIFO"},
// 		{"MRU", config.EvictMRU, "*EvictionPolicy.MRU"},
// 		{"Random", config.EvictRR, "*EvictionPolicy.Random"},
// 		{"TinyLFU", config.EvictTinyLFU, "*EvictionPolicy.TinyLFU"},
// 		{"Default", config.EvictionPolicy(999), "*EvictionPolicy.LRU"},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			evictor := new(tt.policy, 100)
// 			if got := getTypeName(evictor); got != tt.want {
// 				t.Errorf("New() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestLRU(t *testing.T) {
// 	lru := newLRU(10)

// 	// Test OnAdd
// 	lru.OnAdd("key1", &Entry{Key: "key1"})
// 	lru.OnAdd("key2", &Entry{Key: "key2"})
// 	lru.OnAdd("key3", &Entry{Key: "key3"})

// 	// Test OnAccess - move key2 to front
// 	lru.OnAccess("key2", nil)

// 	// Evict should remove least recently used (key1)
// 	evicted := lru.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key1" {
// 		t.Errorf("Evict() = %v, want [key1]", evicted)
// 	}

// 	// Add more entries
// 	lru.OnAdd("key4", &Entry{Key: "key4"})
// 	lru.OnAdd("key5", &Entry{Key: "key5"})

// 	// Test OnDelete
// 	lru.OnDelete("key3")

// 	// Evict should skip deleted key
// 	evicted = lru.Evict(2)
// 	if len(evicted) != 2 {
// 		t.Errorf("Evict() returned %d items, want 2", len(evicted))
// 	}

// 	// Test Reset
// 	lru.Reset()
// 	if lru.list.Len() != 0 {
// 		t.Error("Reset should clear all entries")
// 	}
// }

// func TestMRU(t *testing.T) {
// 	mru := newMRU(10)

// 	mru.OnAdd("key1", &Entry{Key: "key1"})
// 	mru.OnAdd("key2", &Entry{Key: "key2"})
// 	mru.OnAdd("key3", &Entry{Key: "key3"})

// 	// Access key2 - should become most recent
// 	mru.OnAccess("key2", nil)

// 	// Evict should remove most recently used (key2)
// 	evicted := mru.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key2" {
// 		t.Errorf("Evict() = %v, want [key2]", evicted)
// 	}
// }

// func TestLFU(t *testing.T) {
// 	lfu := newLFU(10)

// 	lfu.OnAdd("key1", &Entry{Key: "key1"})
// 	lfu.OnAdd("key2", &Entry{Key: "key2"})
// 	lfu.OnAdd("key3", &Entry{Key: "key3"})

// 	// Access key2 multiple times to increase frequency
// 	lfu.OnAccess("key2", nil)
// 	lfu.OnAccess("key2", nil)
// 	lfu.OnAccess("key1", nil)

// 	// Evict should remove least frequently used (key3)
// 	evicted := lfu.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key3" {
// 		t.Errorf("Evict() = %v, want [key3]", evicted)
// 	}

// 	// Test deletion
// 	lfu.OnDelete("key2")

// 	// Evict remaining - should get both key1 and key2 is deleted, so only key1 remains
// 	evicted = lfu.Evict(2)
// 	if len(evicted) != 1 {
// 		t.Errorf("Evict() returned %d items, want 1 (key2 was deleted)", len(evicted))
// 	}
// 	if len(evicted) > 0 && evicted[0] != "key1" {
// 		t.Errorf("Evict() = %v, want [key1]", evicted)
// 	}

// 	// Test Reset
// 	lfu.Reset()
// 	if len(lfu.frequency) != 0 {
// 		t.Error("Reset should clear frequency map")
// 	}
// }

// func TestFIFO(t *testing.T) {
// 	fifo := newFIFO(10)

// 	fifo.OnAdd("key1", &Entry{Key: "key1"})
// 	fifo.OnAdd("key2", &Entry{Key: "key2"})
// 	fifo.OnAdd("key3", &Entry{Key: "key3"})

// 	// FIFO should evict oldest (key1)
// 	evicted := fifo.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key1" {
// 		t.Errorf("Evict() = %v, want [key1]", evicted)
// 	}

// 	// Test deletion
// 	fifo.OnDelete("key2")

// 	// Evict should skip deleted key
// 	evicted = fifo.Evict(2)
// 	if len(evicted) != 1 || evicted[0] != "key3" {
// 		t.Errorf("Evict() = %v, want [key3]", evicted)
// 	}

// 	// Test growth
// 	for i := 0; i < 100; i++ {
// 		fifo.OnAdd(string(rune(i)), &Entry{})
// 	}

// 	// Test Reset
// 	fifo.Reset()
// 	if fifo.size != 0 {
// 		t.Error("Reset should clear all entries")
// 	}
// }

// func TestLIFO(t *testing.T) {
// 	lifo := newLIFO(10)

// 	lifo.OnAdd("key1", &Entry{Key: "key1"})
// 	lifo.OnAdd("key2", &Entry{Key: "key2"})
// 	lifo.OnAdd("key3", &Entry{Key: "key3"})

// 	// LIFO should evict newest (key3)
// 	evicted := lifo.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key3" {
// 		t.Errorf("Evict() = %v, want [key3]", evicted)
// 	}

// 	// Test deletion
// 	lifo.OnDelete("key2")

// 	// Evict should skip deleted key
// 	evicted = lifo.Evict(2)
// 	if len(evicted) != 1 || evicted[0] != "key1" {
// 		t.Errorf("Evict() = %v, want [key1]", evicted)
// 	}

// 	// Test Reset
// 	lifo.Reset()
// 	if len(lifo.keys) != 0 {
// 		t.Error("Reset should clear all entries")
// 	}
// }

// func TestRandom(t *testing.T) {
// 	random := newRandom(10)

// 	// Add entries
// 	for i := 0; i < 10; i++ {
// 		random.OnAdd(string(rune('A'+i)), &Entry{})
// 	}

// 	// Test deletion
// 	random.OnDelete("C")

// 	// Evict should work with remaining entries
// 	evicted := random.Evict(3)
// 	if len(evicted) != 3 {
// 		t.Errorf("Evict() returned %d items, want 3", len(evicted))
// 	}

// 	// Test Reset
// 	random.Reset()
// 	if len(random.keys) != 0 {
// 		t.Error("Reset should clear all entries")
// 	}
// }

// func TestTinyLFU(t *testing.T) {
// 	tinylfu := newTinyLFU(100)

// 	// Add entries
// 	for i := 0; i < 50; i++ {
// 		key := string(rune('A' + i%26))
// 		tinylfu.OnAdd(key, &Entry{Key: key})

// 		// Access some keys multiple times
// 		if i%3 == 0 {
// 			for j := 0; j < 5; j++ {
// 				tinylfu.OnAccess(key, nil)
// 			}
// 		}
// 	}

// 	// Test deletion
// 	tinylfu.OnDelete("A")

// 	// Evict should remove least frequently used
// 	evicted := tinylfu.Evict(10)
// 	if len(evicted) != 10 {
// 		t.Errorf("Evict() returned %d items, want 10", len(evicted))
// 	}

// 	// Verify that deleted key is not in evicted list
// 	for _, key := range evicted {
// 		if key == "A" {
// 			t.Error("Deleted key should not be evicted")
// 		}
// 	}

// 	// Test Reset
// 	tinylfu.Reset()
// 	if len(tinylfu.keys) != 0 {
// 		t.Error("Reset should clear all entries")
// 	}
// }

// func TestMultipleEvictionPolicys(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		setup   func() Evictor
// 		evictFn func(Evictor) []string
// 	}{
// 		{
// 			name: "LRU",
// 			setup: func() Evictor {
// 				lru := newLRU(10)
// 				for i := 0; i < 5; i++ {
// 					lru.OnAdd(string(rune('A'+i)), &Entry{})
// 				}
// 				return lru
// 			},
// 			evictFn: func(e Evictor) []string {
// 				return e.Evict(3)
// 			},
// 		},
// 		{
// 			name: "LFU",
// 			setup: func() Evictor {
// 				lfu := newLFU(10)
// 				for i := 0; i < 5; i++ {
// 					lfu.OnAdd(string(rune('A'+i)), &Entry{})
// 				}
// 				return lfu
// 			},
// 			evictFn: func(e Evictor) []string {
// 				return e.Evict(3)
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			e := tt.setup()
// 			evicted := tt.evictFn(e)

// 			if len(evicted) != 3 {
// 				t.Errorf("Expected 3 evicted items, got %d", len(evicted))
// 			}

// 			// Evict should not return duplicates
// 			seen := make(map[string]bool)
// 			for _, key := range evicted {
// 				if seen[key] {
// 					t.Errorf("Duplicate key in evicted list: %s", key)
// 				}
// 				seen[key] = true
// 			}
// 		})
// 	}
// }

// func TestEvictWithDeletedEntries(t *testing.T) {
// 	lru := newLRU(10)

// 	// Add entries
// 	for i := 0; i < 5; i++ {
// 		key := string(rune('A' + i))
// 		lru.OnAdd(key, &Entry{})
// 	}

// 	// Delete some entries
// 	lru.OnDelete("B")
// 	lru.OnDelete("D")

// 	// Evict should skip deleted entries
// 	evicted := lru.Evict(5)

// 	// Should only evict remaining entries (A, C, E)
// 	if len(evicted) != 3 {
// 		t.Errorf("Expected 3 evicted items, got %d", len(evicted))
// 	}

// 	for _, key := range evicted {
// 		if key == "B" || key == "D" {
// 			t.Errorf("Deleted key %s should not be evicted", key)
// 		}
// 	}
// }

// func TestEvictZeroCount(t *testing.T) {
// 	lru := newLRU(10)
// 	lru.OnAdd("key1", &Entry{})

// 	evicted := lru.Evict(0)
// 	if len(evicted) != 0 {
// 		t.Errorf("Evict(0) should return empty slice, got %d items", len(evicted))
// 	}
// }

// func TestEvictEmpty(t *testing.T) {
// 	lru := newLRU(10)

// 	evicted := lru.Evict(5)
// 	if len(evicted) != 0 {
// 		t.Errorf("Evict on empty cache should return empty slice, got %d items", len(evicted))
// 	}
// }

// func TestDuplicateKeyOperations(t *testing.T) {
// 	lru := newLRU(10)

// 	// Add same key multiple times
// 	lru.OnAdd("key1", &Entry{})
// 	lru.OnAdd("key1", &Entry{})

// 	// Delete and re-add
// 	lru.OnDelete("key1")
// 	lru.OnAdd("key1", &Entry{})

// 	// Should work without errors
// 	evicted := lru.Evict(1)
// 	if len(evicted) != 1 || evicted[0] != "key1" {
// 		t.Errorf("Evict() = %v, want [key1]", evicted)
// 	}
// }

// // Helper function to get type name for testing.
// func getTypeName(v any) string {
// 	return getTypeNameImpl(v)
// }

// func getTypeNameImpl(v any) string {
// 	switch v.(type) {
// 	case *LRU:
// 		return "*EvictionPolicy.LRU"
// 	case *LFU:
// 		return "*EvictionPolicy.LFU"
// 	case *FIFO:
// 		return "*EvictionPolicy.FIFO"
// 	case *LIFO:
// 		return "*EvictionPolicy.LIFO"
// 	case *MRU:
// 		return "*EvictionPolicy.MRU"
// 	case *Random:
// 		return "*EvictionPolicy.Random"
// 	case *TinyLFU:
// 		return "*EvictionPolicy.TinyLFU"
// 	default:
// 		return ""
// 	}
// }

// // ----------------------------------------------------------------------------
// // Benchmark Tests
// // ----------------------------------------------------------------------------

// func BenchmarkLRUEvict(b *testing.B) {
// 	lru := newLRU(1000)
// 	for i := 0; i < 1000; i++ {
// 		lru.OnAdd(string(rune(i)), &Entry{})
// 	}

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		lru.Evict(10)
// 	}
// }

// func BenchmarkLFUEvict(b *testing.B) {
// 	lfu := newLFU(1000)
// 	for i := 0; i < 1000; i++ {
// 		lfu.OnAdd(string(rune(i)), &Entry{})
// 	}

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		lfu.Evict(10)
// 	}
// }

// func BenchmarkFIFOEvict(b *testing.B) {
// 	fifo := newFIFO(1000)
// 	for i := 0; i < 1000; i++ {
// 		fifo.OnAdd(string(rune(i)), &Entry{})
// 	}

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		fifo.Evict(10)
// 	}
// }

// func BenchmarkLIFOEvict(b *testing.B) {
// 	lifo := newLIFO(1000)
// 	for i := 0; i < 1000; i++ {
// 		lifo.OnAdd(string(rune(i)), &Entry{})
// 	}

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		lifo.Evict(10)
// 	}
// }

func BenchmarkRandomEvict(b *testing.B) {
	random := newRandom(1000)
	for i := 0; i < 1000; i++ {
		random.OnAdd(string(rune(i)), &Entry{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		random.Evict(10)
	}
}

func BenchmarkTinyLFUEvict(b *testing.B) {
	tinylfu := newTinyLFU(1000)
	for i := 0; i < 1000; i++ {
		tinylfu.OnAdd(string(rune(i)), &Entry{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tinylfu.Evict(10)
	}
}
