package atomic

import (
	"sync"
	"testing"
)

func TestAtomicValue_NewAndLoad(t *testing.T) {
	av := NewAtomicValue(42)
	if av.Load() != 42 {
		t.Errorf("expected 42, got %d", av.Load())
	}
}

func TestAtomicValue_Store(t *testing.T) {
	av := NewAtomicValue("old")
	av.Store("new")
	if av.Load() != "new" {
		t.Errorf("expected 'new', got %q", av.Load())
	}
}

func TestAtomicValue_CompareAndSwap(t *testing.T) {
	av := NewAtomicValue(10)
	// Successful CAS.
	ok := av.CompareAndSwap(10, 20)
	if !ok {
		t.Error("expected CAS to succeed")
	}
	if av.Load() != 20 {
		t.Errorf("expected 20, got %d", av.Load())
	}
	// Failed CAS.
	ok = av.CompareAndSwap(10, 30)
	if ok {
		t.Error("expected CAS to fail")
	}
	if av.Load() != 20 {
		t.Errorf("expected still 20, got %d", av.Load())
	}
}

func TestAtomicValue_Concurrent(t *testing.T) {
	av := NewAtomicValue(0)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			av.Store(v)
		}(i)
	}
	wg.Wait()
	// Should have some final value.
	_ = av.Load()
}

func TestAtomicPointer_NewAndLoad(t *testing.T) {
	ap := &AtomicPointer[int]{}
	v := 42
	ap.Store(&v)
	got := ap.Load()
	if got == nil || *got != 42 {
		t.Error("expected 42")
	}
}

func TestAtomicPointer_Swap(t *testing.T) {
	ap := &AtomicPointer[int]{}
	old := 10
	newVal := 20
	ap.Store(&old)
	prev := ap.Swap(&newVal)
	if prev == nil || *prev != 10 {
		t.Error("expected old value 10 from Swap")
	}
	if ap.Load() == nil || *ap.Load() != 20 {
		t.Error("expected new value 20 after Swap")
	}
}

func TestAtomicPointer_CompareAndSwap(t *testing.T) {
	ap := &AtomicPointer[string]{}
	old := "hello"
	newVal := "world"
	ap.Store(&old)

	ok := ap.CompareAndSwap(&old, &newVal)
	if !ok {
		t.Error("expected CAS to succeed")
	}
	if ap.Load() == nil || *ap.Load() != "world" {
		t.Error("expected 'world' after CAS")
	}

	other := "other"
	ok = ap.CompareAndSwap(&other, &old)
	if ok {
		t.Error("expected CAS to fail with wrong old value")
	}
}

func TestKeyedMutex_LockUnlock(t *testing.T) {
	km := NewKeyedMutex()
	km.Lock("key1")
	km.Unlock("key1")
}

func TestKeyedMutex_DifferentKeys(t *testing.T) {
	km := NewKeyedMutex()
	km.Lock("a")
	km.Lock("b")
	if km.Len() != 2 {
		t.Errorf("expected 2 locked keys, got %d", km.Len())
	}
	km.Unlock("a")
	km.Unlock("b")
	if km.Len() != 0 {
		t.Errorf("expected 0 locked keys, got %d", km.Len())
	}
}

func TestKeyedMutex_SameKey_Concurrent(t *testing.T) {
	km := NewKeyedMutex()
	var count int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			km.Lock("same-key")
			mu.Lock()
			count++
			mu.Unlock()
			km.Unlock("same-key")
		}()
	}
	wg.Wait()
	if count != 10 {
		t.Errorf("expected count=10, got %d", count)
	}
}

func TestKeyedMutex_Len(t *testing.T) {
	km := NewKeyedMutex()
	if km.Len() != 0 {
		t.Errorf("expected len=0, got %d", km.Len())
	}
	km.Lock("x")
	km.Lock("y")
	km.Lock("z")
	if km.Len() != 3 {
		t.Errorf("expected len=3, got %d", km.Len())
	}
	km.Unlock("y")
	if km.Len() != 2 {
		t.Errorf("expected len=2, got %d", km.Len())
	}
	km.Unlock("x")
	km.Unlock("z")
	if km.Len() != 0 {
		t.Errorf("expected len=0, got %d", km.Len())
	}
}
