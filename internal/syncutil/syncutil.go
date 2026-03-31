// Package syncutil provides generic atomic types and a per-key mutex.
package syncutil

import (
	"sync"
	"sync/atomic"
)

// ----------------------------------------------------------------------------
// AtomicValue — comparable T (lock-free)
// ----------------------------------------------------------------------------

// AtomicValue is a type-safe, lock-free atomic store for comparable types.
// For non-comparable types (slices, maps, funcs) use AtomicPointer instead.
type AtomicValue[T comparable] struct {
	v atomic.Value
}

// NewAtomicValue returns an AtomicValue initialized to initial.
func NewAtomicValue[T comparable](initial T) *AtomicValue[T] {
	av := &AtomicValue[T]{}
	av.v.Store(initial)
	return av
}

//nolint:forcetypeassert // Load returns the current value.
func (av *AtomicValue[T]) Load() T { return av.v.Load().(T) }

// Store sets the value atomically.
func (av *AtomicValue[T]) Store(val T) { av.v.Store(val) }

// CompareAndSwap atomically replaces the value when it equals old.
func (av *AtomicValue[T]) CompareAndSwap(old, val T) bool {
	return av.v.CompareAndSwap(old, val)
}

// ----------------------------------------------------------------------------
// AtomicPointer — any T via *T pointer swap (lock-free)
// ----------------------------------------------------------------------------

// AtomicPointer is a type-safe wrapper around atomic.Pointer[T].
// Use this for non-comparable or large structs where you want immutable swap
// semantics: callers receive a snapshot pointer; mutation requires a new value.
type AtomicPointer[T any] struct {
	p atomic.Pointer[T]
}

// Load returns the current pointer (may be nil before first Store).
func (ap *AtomicPointer[T]) Load() *T { return ap.p.Load() }

// Store atomically sets the pointer.
func (ap *AtomicPointer[T]) Store(v *T) { ap.p.Store(v) }

// Swap atomically sets the pointer and returns the old one.
func (ap *AtomicPointer[T]) Swap(v *T) *T { return ap.p.Swap(v) }

// CompareAndSwap atomically replaces old with new.
func (ap *AtomicPointer[T]) CompareAndSwap(old, val *T) bool {
	return ap.p.CompareAndSwap(old, val)
}

// ----------------------------------------------------------------------------
// KeyedMutex — per-key mutual exclusion with automatic cleanup
// ----------------------------------------------------------------------------

// KeyedMutex provides per-key mutual exclusion. Goroutines that contend on
// the same key are serialized; independent keys do not block each other.
// Map entries are removed automatically when no goroutine holds or waits for
// the corresponding lock, preventing unbounded memory growth.
type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedEntry
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

// NewKeyedMutex returns a ready-to-use KeyedMutex.
func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{locks: make(map[string]*keyedEntry)}
}

// Lock acquires the per-key lock, blocking until it is available.
func (km *KeyedMutex) Lock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		e = &keyedEntry{}
		km.locks[key] = e
	}
	e.refs++
	km.mu.Unlock()
	e.mu.Lock()
}

// Unlock releases the per-key lock.  Panics if called without a matching Lock.
func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		km.mu.Unlock()
		panic("syncutil.KeyedMutex: Unlock of unlocked key " + key)
	}
	e.refs--
	if e.refs == 0 {
		delete(km.locks, key)
	}
	km.mu.Unlock()
	e.mu.Unlock()
}

// Len returns the number of keys currently tracked (waiting or locked).
func (km *KeyedMutex) Len() int {
	km.mu.Lock()
	defer km.mu.Unlock()
	return len(km.locks)
}
