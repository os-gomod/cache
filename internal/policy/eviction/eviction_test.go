package eviction

import (
	"testing"

	"github.com/os-gomod/cache/v2/memory"
)

// mockEntry creates a memory.Entry for testing.
func mockEntry(key string, value []byte) *memory.Entry {
	return &memory.Entry{
		Key:        key,
		Value:      value,
		Expiry:     0,
		Size:       int64(len(value)),
		LastAccess: 1000,
		Frequency:  1,
	}
}

func TestLRU_Basic(t *testing.T) {
	l := newLRU()

	// OnAdd
	l.OnAdd("a", mockEntry("a", []byte("1")))
	l.OnAdd("b", mockEntry("b", []byte("2")))
	l.OnAdd("c", mockEntry("c", []byte("3")))

	// Access should move to front
	l.OnAccess("a", mockEntry("a", []byte("1")))

	// Evict should return "b" (least recently used after "a" was accessed)
	key, ok := l.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}

	// Next eviction should return "c"
	key, ok = l.Evict()
	if !ok || key != "c" {
		t.Errorf("Evict() = (%q, %v), want (\"c\", true)", key, ok)
	}

	// Last should be "a"
	key, ok = l.Evict()
	if !ok || key != "a" {
		t.Errorf("Evict() = (%q, %v), want (\"a\", true)", key, ok)
	}

	// Empty
	key, ok = l.Evict()
	if ok {
		t.Errorf("Evict() on empty = (%q, %v), want (\"\", false)", key, ok)
	}
}

func TestLRU_OnRemove(t *testing.T) {
	l := newLRU()

	l.OnAdd("a", mockEntry("a", []byte("1")))
	l.OnAdd("b", mockEntry("b", []byte("2")))

	l.OnRemove("a")

	// "a" was removed, next should be "b"
	key, ok := l.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}
}

func TestLRU_Reset(t *testing.T) {
	l := newLRU()
	l.OnAdd("a", mockEntry("a", []byte("1")))
	l.OnAdd("b", mockEntry("b", []byte("2")))
	l.Reset()

	key, ok := l.Evict()
	if ok {
		t.Errorf("Evict() after Reset = (%q, %v), want (\"\", false)", key, ok)
	}
}

func TestLFU_Basic(t *testing.T) {
	l := newLFU()

	l.OnAdd("a", mockEntry("a", []byte("1")))
	l.OnAdd("b", mockEntry("b", []byte("2")))
	l.OnAdd("c", mockEntry("c", []byte("3")))

	// Access "a" 5 times
	for i := 0; i < 5; i++ {
		l.OnAccess("a", mockEntry("a", []byte("1")))
	}
	// Access "b" 3 times
	for i := 0; i < 3; i++ {
		l.OnAccess("b", mockEntry("b", []byte("2")))
	}
	// "c" has frequency 1

	// Evict should return "c" (lowest frequency)
	key, ok := l.Evict()
	if !ok || key != "c" {
		t.Errorf("Evict() = (%q, %v), want (\"c\", true)", key, ok)
	}

	// Next should be "b"
	key, ok = l.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}
}

func TestFIFO_Basic(t *testing.T) {
	f := newFIFO()

	f.OnAdd("a", mockEntry("a", []byte("1")))
	f.OnAdd("b", mockEntry("b", []byte("2")))
	f.OnAdd("c", mockEntry("c", []byte("3")))

	// Access doesn't change order
	f.OnAccess("a", mockEntry("a", []byte("1")))

	// FIFO: evict oldest first
	key, ok := f.Evict()
	if !ok || key != "a" {
		t.Errorf("Evict() = (%q, %v), want (\"a\", true)", key, ok)
	}

	key, ok = f.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}
}

func TestLIFO_Basic(t *testing.T) {
	l := newLIFO()

	l.OnAdd("a", mockEntry("a", []byte("1")))
	l.OnAdd("b", mockEntry("b", []byte("2")))
	l.OnAdd("c", mockEntry("c", []byte("3")))

	// LIFO: evict newest first
	key, ok := l.Evict()
	if !ok || key != "c" {
		t.Errorf("Evict() = (%q, %v), want (\"c\", true)", key, ok)
	}

	key, ok = l.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}
}

func TestMRU_Basic(t *testing.T) {
	m := newMRU()

	m.OnAdd("a", mockEntry("a", []byte("1")))
	m.OnAdd("b", mockEntry("b", []byte("2")))
	m.OnAdd("c", mockEntry("c", []byte("3")))

	// Access "a" makes it most recent
	m.OnAccess("a", mockEntry("a", []byte("1")))

	// MRU: evict most recently used first
	key, ok := m.Evict()
	if !ok || key != "a" {
		t.Errorf("Evict() = (%q, %v), want (\"a\", true)", key, ok)
	}
}

func TestRandom_Basic(t *testing.T) {
	r := newRandom()

	r.OnAdd("a", mockEntry("a", []byte("1")))
	r.OnAdd("b", mockEntry("b", []byte("2")))
	r.OnAdd("c", mockEntry("c", []byte("3")))

	// Should evict some key
	key, ok := r.Evict()
	if !ok {
		t.Error("Evict() should return a key")
	}
	if key != "a" && key != "b" && key != "c" {
		t.Errorf("Evict() = %q, want one of [a, b, c]", key)
	}

	// Two more evictions should drain
	r.Evict()
	_, _ = r.Evict()
	if !ok {
		t.Error("Evict() should return a key")
	}

	// Empty
	key, ok = r.Evict()
	if ok {
		t.Errorf("Evict() on empty = (%q, %v), want (\"\", false)", key, ok)
	}
}

func TestTinyLFU_Basic(t *testing.T) {
	tl := newTinyLFU(1024 * 1024)

	tl.OnAdd("a", mockEntry("a", []byte("1")))
	tl.OnAdd("b", mockEntry("b", []byte("2")))
	tl.OnAdd("c", mockEntry("c", []byte("3")))

	// Access "a" multiple times to increase frequency
	for i := 0; i < 10; i++ {
		tl.OnAccess("a", mockEntry("a", []byte("1")))
	}

	// "c" has lowest frequency, should be evicted
	key, ok := tl.Evict()
	if !ok || key != "c" {
		t.Errorf("Evict() = (%q, %v), want (\"c\", true)", key, ok)
	}
}

func TestTinyLFU_DoorkeeperFilters(t *testing.T) {
	tl := newTinyLFU(1024 * 1024)

	tl.OnAdd("a", mockEntry("a", []byte("1")))
	tl.OnAdd("b", mockEntry("b", []byte("2")))

	// Single access to "b" (one-hit wonder)
	tl.OnAccess("b", mockEntry("b", []byte("2")))

	// Multiple accesses to "a"
	for i := 0; i < 5; i++ {
		tl.OnAccess("a", mockEntry("a", []byte("1")))
	}

	// "b" should be evicted (lower frequency after doorkeeper filtering)
	key, ok := tl.Evict()
	if !ok || key != "b" {
		t.Errorf("Evict() = (%q, %v), want (\"b\", true)", key, ok)
	}
}

func TestPolicyString(t *testing.T) {
	tests := []struct {
		p    Policy
		want string
	}{
		{PolicyLRU, "lru"},
		{PolicyLFU, "lfu"},
		{PolicyFIFO, "fifo"},
		{PolicyLIFO, "lifo"},
		{PolicyMRU, "mru"},
		{PolicyRandom, "random"},
		{PolicyTinyLFU, "tinylfu"},
		{Policy(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Policy(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		p    Policy
		name string
	}{
		{PolicyLRU, "lru"},
		{PolicyLFU, "lfu"},
		{PolicyFIFO, "fifo"},
		{PolicyLIFO, "lifo"},
		{PolicyMRU, "mru"},
		{PolicyRandom, "random"},
		{PolicyTinyLFU, "tinylfu"},
	}

	for _, tt := range tests {
		e := New(tt.p, 1024)
		if e == nil {
			t.Errorf("New(%s) returned nil", tt.name)
			continue
		}
		if e.Name() != tt.name {
			t.Errorf("New(%s).Name() = %q", tt.name, e.Name())
		}
		// Test Reset
		e.Reset()
		key, ok := e.Evict()
		if ok {
			t.Errorf("New(%s).Evict() after Reset = (%q, true), want (\"\", false)", tt.name, key)
		}
	}
}

func BenchmarkLRU_Evict(b *testing.B) {
	l := newLRU()
	for i := 0; i < 10000; i++ {
		l.OnAdd(string(rune(i)), mockEntry(string(rune(i)), []byte("value")))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Evict()
	}
}

func BenchmarkLFU_Evict(b *testing.B) {
	l := newLFU()
	for i := 0; i < 10000; i++ {
		l.OnAdd(string(rune(i)), mockEntry(string(rune(i)), []byte("value")))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Evict()
	}
}
