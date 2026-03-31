// Package syncutil_test provides tests for generic atomic types and per-key mutex.
package syncutil_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/syncutil"
)

func TestAtomicValue(t *testing.T) {
	t.Run("NewAtomicValue initializes correctly", func(t *testing.T) {
		av := syncutil.NewAtomicValue(42)
		if got := av.Load(); got != 42 {
			t.Errorf("Load() = %v, want 42", got)
		}
	})

	t.Run("Store updates value", func(t *testing.T) {
		av := syncutil.NewAtomicValue("initial")
		av.Store("updated")
		if got := av.Load(); got != "updated" {
			t.Errorf("Load() = %v, want updated", got)
		}
	})

	t.Run("CompareAndSwap succeeds with matching old value", func(t *testing.T) {
		av := syncutil.NewAtomicValue(10)
		if !av.CompareAndSwap(10, 20) {
			t.Error("CompareAndSwap should succeed")
		}
		if got := av.Load(); got != 20 {
			t.Errorf("Load() = %v, want 20", got)
		}
	})

	t.Run("CompareAndSwap fails with mismatched old value", func(t *testing.T) {
		av := syncutil.NewAtomicValue(10)
		if av.CompareAndSwap(99, 20) {
			t.Error("CompareAndSwap should fail")
		}
		if got := av.Load(); got != 10 {
			t.Errorf("Load() = %v, want 10", got)
		}
	})

	t.Run("concurrent operations", func(t *testing.T) {
		av := syncutil.NewAtomicValue(0)
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					current := av.Load()
					if av.CompareAndSwap(current, current+1) {
						break
					}
				}
			}()
		}
		wg.Wait()

		if got := av.Load(); got != 100 {
			t.Errorf("final value = %v, want 100", got)
		}
	})
}

func TestAtomicValue_WithCustomType(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	initial := User{ID: 1, Name: "Alice"}
	av := syncutil.NewAtomicValue(initial)

	if got := av.Load(); got != initial {
		t.Errorf("Load() = %v, want %v", got, initial)
	}

	updated := User{ID: 2, Name: "Bob"}
	av.Store(updated)
	if got := av.Load(); got != updated {
		t.Errorf("Load() = %v, want %v", got, updated)
	}
}

func TestAtomicPointer(t *testing.T) {
	t.Run("Load returns nil before Store", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[string]{}
		if got := ap.Load(); got != nil {
			t.Errorf("Load() = %v, want nil", got)
		}
	})

	t.Run("Store and Load", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[string]{}
		val := "hello"
		ap.Store(&val)

		if got := ap.Load(); got == nil {
			t.Error("Load() returned nil")
		} else if *got != "hello" {
			t.Errorf("Load() = %v, want hello", *got)
		}
	})

	t.Run("Swap returns old value", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[int]{}
		oldVal := 42
		newVal := 100

		ap.Store(&oldVal)
		old := ap.Swap(&newVal)

		if old == nil {
			t.Error("Swap() returned nil")
		} else if *old != 42 {
			t.Errorf("Swap() = %v, want 42", *old)
		}

		if got := ap.Load(); got == nil {
			t.Error("Load() returned nil")
		} else if *got != 100 {
			t.Errorf("Load() = %v, want 100", *got)
		}
	})

	t.Run("CompareAndSwap succeeds with matching pointer", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[string]{}
		oldVal := "old"
		newVal := "new"

		ap.Store(&oldVal)
		if !ap.CompareAndSwap(&oldVal, &newVal) {
			t.Error("CompareAndSwap should succeed")
		}

		if got := ap.Load(); got == nil {
			t.Error("Load() returned nil")
		} else if *got != "new" {
			t.Errorf("Load() = %v, want new", *got)
		}
	})

	t.Run("CompareAndSwap fails with mismatched pointer", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[string]{}
		val1 := "val1"
		val2 := "val2"
		val3 := "val3"

		ap.Store(&val1)
		if ap.CompareAndSwap(&val2, &val3) {
			t.Error("CompareAndSwap should fail")
		}

		if got := ap.Load(); got == nil {
			t.Error("Load() returned nil")
		} else if *got != "val1" {
			t.Errorf("Load() = %v, want val1", *got)
		}
	})

	t.Run("concurrent operations", func(t *testing.T) {
		ap := &syncutil.AtomicPointer[int]{}
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				ap.Store(&val)
			}(i)
		}
		wg.Wait()

		if got := ap.Load(); got == nil {
			t.Error("Load() returned nil")
		}
	})
}

func TestAtomicPointer_WithLargeStruct(t *testing.T) {
	type LargeStruct struct {
		ID      int
		Data    [1024]byte
		Metrics map[string]float64
	}

	ap := &syncutil.AtomicPointer[LargeStruct]{}

	initial := &LargeStruct{
		ID:      1,
		Metrics: map[string]float64{"cpu": 0.5},
	}
	ap.Store(initial)

	if got := ap.Load(); got != initial {
		t.Error("Load() returned different pointer")
	}

	updated := &LargeStruct{
		ID:      2,
		Metrics: map[string]float64{"cpu": 0.8},
	}
	ap.Store(updated)

	if got := ap.Load(); got != updated {
		t.Error("Load() returned different pointer")
	}
}

func TestKeyedMutex(t *testing.T) {
	t.Run("NewKeyedMutex initializes correctly", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		if km == nil {
			t.Fatal("NewKeyedMutex returned nil")
		}
		if got := km.Len(); got != 0 {
			t.Errorf("Len() = %v, want 0", got)
		}
	})

	t.Run("Lock and Unlock on same key", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key := "test-key"

		km.Lock(key)
		if got := km.Len(); got != 1 {
			t.Errorf("Len() after Lock = %v, want 1", got)
		}
		km.Unlock(key)
		if got := km.Len(); got != 0 {
			t.Errorf("Len() after Unlock = %v, want 0", got)
		}
	})

	t.Run("Lock blocks on same key", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key := "blocking-key"

		var started atomic.Bool
		var blocked atomic.Bool
		var wg sync.WaitGroup

		// First goroutine acquires lock
		km.Lock(key)
		started.Store(true)

		wg.Add(1)
		go func() {
			defer wg.Done()
			blocked.Store(true)
			km.Lock(key)
			blocked.Store(false)
			km.Unlock(key)
		}()

		// Wait for second goroutine to start and block
		time.Sleep(10 * time.Millisecond)
		if !blocked.Load() {
			t.Log("second goroutine may not have started yet")
		}

		// Release first lock
		km.Unlock(key)

		// Wait for second goroutine to complete
		wg.Wait()
		if blocked.Load() {
			t.Error("second goroutine did not acquire lock")
		}
	})

	t.Run("Different keys do not block", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key1 := "key1"
		key2 := "key2"

		var wg sync.WaitGroup
		var completed1, completed2 atomic.Bool

		km.Lock(key1)

		wg.Add(2)
		go func() {
			defer wg.Done()
			km.Lock(key2)
			completed2.Store(true)
			km.Unlock(key2)
		}()
		go func() {
			defer wg.Done()
			km.Lock(key1)
			completed1.Store(true)
			km.Unlock(key1)
		}()

		time.Sleep(10 * time.Millisecond)

		// key2 should not be blocked
		if !completed2.Load() {
			t.Error("key2 lock was blocked by key1 lock")
		}

		// key1 should still be blocked
		if completed1.Load() {
			t.Error("key1 lock was acquired before release")
		}

		km.Unlock(key1)
		wg.Wait()

		if !completed1.Load() {
			t.Error("key1 lock was never acquired")
		}
	})

	t.Run("Len tracks active keys", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		keys := []string{"a", "b", "c"}

		for _, key := range keys {
			km.Lock(key)
		}

		if got := km.Len(); got != 3 {
			t.Errorf("Len() = %v, want 3", got)
		}

		// Unlock two keys
		km.Unlock(keys[0])
		if got := km.Len(); got != 2 {
			t.Errorf("Len() after one unlock = %v, want 2", got)
		}

		km.Unlock(keys[1])
		if got := km.Len(); got != 1 {
			t.Errorf("Len() after two unlocks = %v, want 1", got)
		}

		km.Unlock(keys[2])
		if got := km.Len(); got != 0 {
			t.Errorf("Len() after all unlocks = %v, want 0", got)
		}
	})

	t.Run("Multiple locks on same key increase ref count", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key := "ref-key"

		// Hold the key once, then queue multiple waiters behind it.
		km.Lock(key)
		var wg sync.WaitGroup

		wg.Add(2)
		go func() {
			defer wg.Done()
			km.Lock(key)
			km.Unlock(key)
		}()
		go func() {
			defer wg.Done()
			km.Lock(key)
			km.Unlock(key)
		}()

		time.Sleep(10 * time.Millisecond)
		if got := km.Len(); got != 1 {
			t.Errorf("Len() with waiting goroutines = %v, want 1", got)
		}

		km.Unlock(key)
		wg.Wait()

		if got := km.Len(); got != 0 {
			t.Errorf("Len() after all unlocks = %v, want 0", got)
		}
	})

	t.Run("Concurrent locks on different keys", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		const numKeys = 100
		const numGoroutinesPerKey = 10

		var wg sync.WaitGroup
		errors := make(chan error, numKeys*numGoroutinesPerKey)

		for i := 0; i < numKeys; i++ {
			key := string(rune('A' + i%26))
			for j := 0; j < numGoroutinesPerKey; j++ {
				wg.Add(1)
				go func(k string) {
					defer wg.Done()
					km.Lock(k)
					// Simulate work
					time.Sleep(time.Microsecond)
					km.Unlock(k)
				}(key)
			}
		}

		wg.Wait()
		close(errors)

		if got := km.Len(); got != 0 {
			t.Errorf("Len() after all operations = %v, want 0", got)
		}
	})

	t.Run("Panic on unlocking unlocked key", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		defer func() {
			if r := recover(); r == nil {
				t.Error("Unlock on unlocked key did not panic")
			}
		}()
		km.Unlock("non-existent-key")
	})

	t.Run("Lock and Unlock with empty key", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key := ""

		km.Lock(key)
		if got := km.Len(); got != 1 {
			t.Errorf("Len() = %v, want 1", got)
		}
		km.Unlock(key)
		if got := km.Len(); got != 0 {
			t.Errorf("Len() = %v, want 0", got)
		}
	})

	t.Run("Lock with same key from many goroutines", func(t *testing.T) {
		km := syncutil.NewKeyedMutex()
		key := "contended-key"
		const numGoroutines = 100

		var counter int64
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				km.Lock(key)
				atomic.AddInt64(&counter, 1)
				km.Unlock(key)
			}()
		}

		wg.Wait()

		if counter != numGoroutines {
			t.Errorf("counter = %v, want %v", counter, numGoroutines)
		}
		if got := km.Len(); got != 0 {
			t.Errorf("Len() after all operations = %v, want 0", got)
		}
	})
}

func TestKeyedMutex_ConcurrentWithMixedKeys(t *testing.T) {
	km := syncutil.NewKeyedMutex()
	keys := []string{"apple", "banana", "cherry", "date", "elderberry"}

	var wg sync.WaitGroup
	var counters sync.Map

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		key := keys[i%len(keys)]
		go func(k string) {
			defer wg.Done()
			km.Lock(k)

			// Increment counter for this key
			val, _ := counters.LoadOrStore(k, new(int64))
			counter := val.(*int64)
			atomic.AddInt64(counter, 1)

			km.Unlock(k)
		}(key)
	}

	wg.Wait()

	// Verify each key was accessed
	for _, key := range keys {
		if val, ok := counters.Load(key); ok {
			counter := val.(*int64)
			if *counter == 0 {
				t.Errorf("key %s was never accessed", key)
			}
		} else {
			t.Errorf("key %s was never accessed", key)
		}
	}

	if got := km.Len(); got != 0 {
		t.Errorf("Len() after all operations = %v, want 0", got)
	}
}

func TestKeyedMutex_MemoryCleanup(t *testing.T) {
	km := syncutil.NewKeyedMutex()

	// Lock and unlock many keys
	for i := 0; i < 1000; i++ {
		key := string(rune(i))
		km.Lock(key)
		km.Unlock(key)

		// Len should return to 0 after each pair
		if got := km.Len(); got != 0 {
			t.Errorf("Len() after unlock = %v, want 0", got)
		}
	}

	if got := km.Len(); got != 0 {
		t.Errorf("final Len() = %v, want 0", got)
	}
}

func BenchmarkAtomicValue(b *testing.B) {
	av := syncutil.NewAtomicValue(0)

	b.Run("Load", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = av.Load()
		}
	})

	b.Run("Store", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			av.Store(i)
		}
	})

	b.Run("CompareAndSwap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			av.CompareAndSwap(i, i+1)
		}
	})
}

func BenchmarkAtomicPointer(b *testing.B) {
	ap := &syncutil.AtomicPointer[int]{}
	val := 42

	b.Run("Load", func(b *testing.B) {
		ap.Store(&val)
		for i := 0; i < b.N; i++ {
			_ = ap.Load()
		}
	})

	b.Run("Store", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ap.Store(&val)
		}
	})

	b.Run("Swap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ap.Swap(&val)
		}
	})
}

func BenchmarkKeyedMutex(b *testing.B) {
	km := syncutil.NewKeyedMutex()
	keys := []string{"key1", "key2", "key3", "key4", "key5"}

	b.Run("LockUnlock same key", func(b *testing.B) {
		key := "bench-key"
		for i := 0; i < b.N; i++ {
			km.Lock(key)
			km.Unlock(key)
		}
	})

	b.Run("LockUnlock different keys", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := keys[i%len(keys)]
			km.Lock(key)
			km.Unlock(key)
		}
	})

	b.Run("Parallel same key", func(b *testing.B) {
		key := "parallel-key"
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				km.Lock(key)
				km.Unlock(key)
			}
		})
	})

	b.Run("Parallel different keys", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := keys[i%len(keys)]
				km.Lock(key)
				km.Unlock(key)
				i++
			}
		})
	})
}
