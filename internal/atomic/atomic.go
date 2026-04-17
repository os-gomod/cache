// Package atomic provides atomic utility types for lock-free concurrent operations.
package atomic

import (
	"sync"
	"sync/atomic"
)

type AtomicValue[T comparable] struct {
	v atomic.Value
}

func NewAtomicValue[T comparable](initial T) *AtomicValue[T] {
	av := &AtomicValue[T]{}
	av.v.Store(initial)
	return av
}
func (av *AtomicValue[T]) Load() T     { return av.v.Load().(T) }
func (av *AtomicValue[T]) Store(val T) { av.v.Store(val) }
func (av *AtomicValue[T]) CompareAndSwap(old, val T) bool {
	return av.v.CompareAndSwap(old, val)
}

type AtomicPointer[T any] struct {
	p atomic.Pointer[T]
}

func (ap *AtomicPointer[T]) Load() *T     { return ap.p.Load() }
func (ap *AtomicPointer[T]) Store(v *T)   { ap.p.Store(v) }
func (ap *AtomicPointer[T]) Swap(v *T) *T { return ap.p.Swap(v) }
func (ap *AtomicPointer[T]) CompareAndSwap(old, val *T) bool {
	return ap.p.CompareAndSwap(old, val)
}

type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedEntry
}
type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{locks: make(map[string]*keyedEntry)}
}

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

func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		km.mu.Unlock()
		panic("atomic.KeyedMutex: Unlock of unlocked key " + key)
	}
	e.refs--
	if e.refs == 0 {
		delete(km.locks, key)
	}
	km.mu.Unlock()
	e.mu.Unlock()
}

func (km *KeyedMutex) Len() int {
	km.mu.Lock()
	defer km.mu.Unlock()
	return len(km.locks)
}
