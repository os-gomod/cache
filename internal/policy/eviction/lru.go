package eviction

import (
	"container/list"
	"sync"

	"github.com/os-gomod/cache/v2/memory"
)

// lru implements the Least Recently Used eviction policy using a doubly-linked
// list for access-order tracking and a map for O(1) lookups.
type lru struct {
	mu    sync.RWMutex
	items map[string]*list.Element
	order *list.List // front = most recent, back = least recent
}

// lruEntry wraps a cache entry with its list element reference.
type lruEntry struct {
	key string
	e   *memory.Entry
}

// newLRU creates a new LRU evictor.
func newLRU() *lru {
	return &lru{
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Name returns "lru".
func (*lru) Name() string { return "lru" }

// OnAccess moves the accessed entry to the front of the list (most recent).
//
//nolint:dupl // structural similarity is intentional
func (l *lru) OnAccess(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		l.order.MoveToFront(elem)
		elem.Value.(*lruEntry).e = e
		return
	}

	// Entry not tracked (may have been evicted) - re-add it
	elem := l.order.PushFront(&lruEntry{key: key, e: e})
	l.items[key] = elem
}

// OnAdd adds a new entry at the front of the list.
//
//nolint:dupl // structural similarity is intentional
func (l *lru) OnAdd(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		l.order.MoveToFront(elem)
		elem.Value.(*lruEntry).e = e
		return
	}

	elem := l.order.PushFront(&lruEntry{key: key, e: e})
	l.items[key] = elem
}

// OnRemove removes the entry from tracking.
func (l *lru) OnRemove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		l.order.Remove(elem)
		delete(l.items, key)
	}
}

// Evict returns the least recently used key.
func (l *lru) Evict() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	back := l.order.Back()
	if back == nil {
		return "", false
	}

	entry := back.Value.(*lruEntry)
	l.order.Remove(back)
	delete(l.items, entry.key)
	return entry.key, true
}

// Reset clears all internal state.
func (l *lru) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.items = make(map[string]*list.Element)
	l.order.Init()
}

// lifo implements the Last In, First Out eviction policy.
type lifo struct {
	mu    sync.RWMutex
	items map[string]*list.Element
	order *list.List // front = newest, back = oldest
}

func newLIFO() *lifo {
	return &lifo{
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (*lifo) Name() string { return "lifo" }

func (l *lifo) OnAccess(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// LIFO doesn't change position on access
	if _, ok := l.items[key]; !ok {
		elem := l.order.PushFront(&lruEntry{key: key, e: e})
		l.items[key] = elem
	}
}

//nolint:dupl // structural similarity is intentional
func (l *lifo) OnAdd(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.items[key]; ok {
		l.order.MoveToFront(elem)
		elem.Value.(*lruEntry).e = e
		return
	}
	elem := l.order.PushFront(&lruEntry{key: key, e: e})
	l.items[key] = elem
}

func (l *lifo) OnRemove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.items[key]; ok {
		l.order.Remove(elem)
		delete(l.items, key)
	}
}

func (l *lifo) Evict() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// LIFO evicts the most recently added (front)
	front := l.order.Front()
	if front == nil {
		return "", false
	}
	entry := front.Value.(*lruEntry)
	l.order.Remove(front)
	delete(l.items, entry.key)
	return entry.key, true
}

func (l *lifo) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[string]*list.Element)
	l.order.Init()
}

// mru implements the Most Recently Used eviction policy. It evicts the
// most recently accessed entry, which is useful for sequential scan patterns.
type mru struct {
	mu    sync.RWMutex
	items map[string]*list.Element
	order *list.List // front = most recent
}

func newMRU() *mru {
	return &mru{
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (*mru) Name() string { return "mru" }

//nolint:dupl // structural similarity is intentional
func (m *mru) OnAccess(key string, e *memory.Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if elem, ok := m.items[key]; ok {
		m.order.MoveToFront(elem)
		elem.Value.(*lruEntry).e = e
		return
	}
	elem := m.order.PushFront(&lruEntry{key: key, e: e})
	m.items[key] = elem
}

//nolint:dupl // structural similarity is intentional
func (m *mru) OnAdd(key string, e *memory.Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if elem, ok := m.items[key]; ok {
		m.order.MoveToFront(elem)
		elem.Value.(*lruEntry).e = e
		return
	}
	elem := m.order.PushFront(&lruEntry{key: key, e: e})
	m.items[key] = elem
}

func (m *mru) OnRemove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if elem, ok := m.items[key]; ok {
		m.order.Remove(elem)
		delete(m.items, key)
	}
}

func (m *mru) Evict() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// MRU evicts the most recently used (front)
	front := m.order.Front()
	if front == nil {
		return "", false
	}
	entry := front.Value.(*lruEntry)
	m.order.Remove(front)
	delete(m.items, entry.key)
	return entry.key, true
}

func (m *mru) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*list.Element)
	m.order.Init()
}
