// Package eviction implements seven cache eviction strategies: LRU, LFU, TinyLFU,
// FIFO, LIFO, MRU, and Random Replacement.
package eviction

import (
	"container/heap"
	"container/list"
	"math/rand/v2"
	"sync"

	"github.com/os-gomod/cache/config"
)

type Evictor interface {
	OnAccess(key string, entry *Entry)
	OnAdd(key string, entry *Entry)
	OnDelete(key string)
	Evict(count int) []string
	Reset()
}

func New(policy config.EvictionPolicy, maxBytes int64) Evictor {
	switch policy {
	case config.EvictLFU:
		return newLFU(maxBytes)
	case config.EvictFIFO:
		return newFIFO(maxBytes)
	case config.EvictLIFO:
		return newLIFO(maxBytes)
	case config.EvictMRU:
		return newMRU(maxBytes)
	case config.EvictRR:
		return newRandom(maxBytes)
	case config.EvictTinyLFU:
		return newTinyLFU(maxBytes)
	default:
		return newLRU(maxBytes)
	}
}

type mapSliceEvictor struct {
	mu       sync.Mutex
	keys     []string
	keyIndex map[string]int
	maxBytes int64
	size     int
}

func newMapSliceEvictor(maxBytes int64) *mapSliceEvictor {
	return &mapSliceEvictor{
		maxBytes: maxBytes,
		keys:     make([]string, 0, 64),
		keyIndex: make(map[string]int, 64),
	}
}

func (e *mapSliceEvictor) addKey(key string) {
	if _, exists := e.keyIndex[key]; exists {
		return
	}
	e.keyIndex[key] = len(e.keys)
	e.keys = append(e.keys, key)
	e.size++
}

func (e *mapSliceEvictor) removeKey(key string) {
	idx, ok := e.keyIndex[key]
	if !ok {
		return
	}
	last := len(e.keys) - 1
	if idx != last {
		lastKey := e.keys[last]
		e.keys[idx] = lastKey
		e.keyIndex[lastKey] = idx
	}
	e.keys = e.keys[:last]
	delete(e.keyIndex, key)
	e.size--
}

func evictFrontKeys(
	order *list.List,
	items map[string]*list.Element,
	count int,
) []string {
	victims := make([]string, 0, count)
	for i := 0; i < count && order.Len() > 0; i++ {
		front := order.Front()
		if front == nil {
			break
		}
		key := front.Value.(string)
		order.Remove(front)
		delete(items, key)
		victims = append(victims, key)
	}
	return victims
}

type lruEntry struct {
	key   string
	entry *Entry
}
type lruEvictor struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	maxBytes int64
}

func newLRU(maxBytes int64) *lruEvictor {
	return &lruEvictor{
		items:    make(map[string]*list.Element, 64),
		order:    list.New(),
		maxBytes: maxBytes,
	}
}

func (e *lruEvictor) OnAccess(key string, _ *Entry) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.MoveToFront(el)
	}
	e.mu.Unlock()
}

func (e *lruEvictor) OnAdd(key string, entry *Entry) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.MoveToFront(el)
		el.Value.(*lruEntry).entry = entry
	} else {
		elem := e.order.PushFront(&lruEntry{key: key, entry: entry})
		e.items[key] = elem
	}
	e.mu.Unlock()
}

func (e *lruEvictor) OnDelete(key string) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.Remove(el)
		delete(e.items, key)
	}
	e.mu.Unlock()
}

func (e *lruEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	victims := make([]string, 0, count)
	for i := 0; i < count && e.order.Len() > 0; i++ {
		back := e.order.Back()
		if back == nil {
			break
		}
		le := back.Value.(*lruEntry)
		e.order.Remove(back)
		delete(e.items, le.key)
		victims = append(victims, le.key)
	}
	return victims
}

func (e *lruEvictor) Reset() {
	e.mu.Lock()
	e.items = make(map[string]*list.Element, 64)
	e.order.Init()
	e.mu.Unlock()
}

type lfuItem struct {
	key   string
	freq  int64
	index int
}
type lfuHeap []*lfuItem

func (h lfuHeap) Len() int           { return len(h) }
func (h lfuHeap) Less(i, j int) bool { return h[i].freq < h[j].freq }
func (h lfuHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *lfuHeap) Push(
	x any,
) {
	item := x.(*lfuItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *lfuHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

type lfuEvictor struct {
	mu       sync.Mutex
	items    map[string]*lfuItem
	h        lfuHeap
	maxBytes int64
}

func newLFU(maxBytes int64) *lfuEvictor {
	h := make(lfuHeap, 0, 64)
	heap.Init(&h)
	return &lfuEvictor{items: make(map[string]*lfuItem, 64), h: h, maxBytes: maxBytes}
}

func (e *lfuEvictor) OnAccess(key string, entry *Entry) {
	e.mu.Lock()
	if item, ok := e.items[key]; ok {
		item.freq = entry.GetHits()
		heap.Fix(&e.h, item.index)
	}
	e.mu.Unlock()
}

func (e *lfuEvictor) OnAdd(key string, entry *Entry) {
	e.mu.Lock()
	if item, ok := e.items[key]; ok {
		item.freq = entry.GetHits()
		heap.Fix(&e.h, item.index)
	} else {
		newItem := &lfuItem{key: key, freq: entry.GetHits()}
		heap.Push(&e.h, newItem)
		e.items[key] = newItem
	}
	e.mu.Unlock()
}

func (e *lfuEvictor) OnDelete(key string) {
	e.mu.Lock()
	if item, ok := e.items[key]; ok {
		heap.Remove(&e.h, item.index)
		delete(e.items, key)
	}
	e.mu.Unlock()
}

func (e *lfuEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	victims := make([]string, 0, count)
	for i := 0; i < count && e.h.Len() > 0; i++ {
		item := heap.Pop(&e.h).(*lfuItem)
		delete(e.items, item.key)
		victims = append(victims, item.key)
	}
	return victims
}

func (e *lfuEvictor) Reset() {
	e.mu.Lock()
	e.items = make(map[string]*lfuItem, 64)
	e.h = e.h[:0]
	heap.Init(&e.h)
	e.mu.Unlock()
}

type fifoEvictor struct {
	mu       sync.Mutex
	order    *list.List
	items    map[string]*list.Element
	maxBytes int64
}

func newFIFO(maxBytes int64) *fifoEvictor {
	return &fifoEvictor{
		order:    list.New(),
		items:    make(map[string]*list.Element, 64),
		maxBytes: maxBytes,
	}
}
func (e *fifoEvictor) OnAccess(_ string, _ *Entry) {}
func (e *fifoEvictor) OnAdd(key string, _ *Entry) {
	e.mu.Lock()
	if _, ok := e.items[key]; !ok {
		el := e.order.PushBack(key)
		e.items[key] = el
	}
	e.mu.Unlock()
}

func (e *fifoEvictor) OnDelete(key string) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.Remove(el)
		delete(e.items, key)
	}
	e.mu.Unlock()
}

func (e *fifoEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return evictFrontKeys(e.order, e.items, count)
}

func (e *fifoEvictor) Reset() {
	e.mu.Lock()
	e.order.Init()
	e.items = make(map[string]*list.Element, 64)
	e.mu.Unlock()
}

type lifoEvictor struct {
	*mapSliceEvictor
}

func newLIFO(maxBytes int64) *lifoEvictor {
	return &lifoEvictor{newMapSliceEvictor(maxBytes)}
}
func (e *lifoEvictor) OnAccess(_ string, _ *Entry) {}
func (e *lifoEvictor) OnAdd(key string, _ *Entry) {
	e.mu.Lock()
	e.addKey(key)
	e.mu.Unlock()
}

func (e *lifoEvictor) OnDelete(key string) {
	e.mu.Lock()
	e.removeKey(key)
	e.mu.Unlock()
}

func (e *lifoEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	victims := make([]string, 0, count)
	for i := 0; i < count && len(e.keys) > 0; i++ {
		last := len(e.keys) - 1
		key := e.keys[last]
		e.keys = e.keys[:last]
		delete(e.keyIndex, key)
		e.size--
		victims = append(victims, key)
	}
	return victims
}

func (e *lifoEvictor) Reset() {
	e.mu.Lock()
	e.keys = e.keys[:0]
	e.keyIndex = make(map[string]int, 64)
	e.size = 0
	e.mu.Unlock()
}

type mruEvictor struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	maxBytes int64
}

func newMRU(maxBytes int64) *mruEvictor {
	return &mruEvictor{
		items:    make(map[string]*list.Element, 64),
		order:    list.New(),
		maxBytes: maxBytes,
	}
}

func (e *mruEvictor) OnAccess(key string, _ *Entry) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.MoveToFront(el)
	}
	e.mu.Unlock()
}

func (e *mruEvictor) OnAdd(key string, _ *Entry) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.MoveToFront(el)
	} else {
		elem := e.order.PushFront(key)
		e.items[key] = elem
	}
	e.mu.Unlock()
}

func (e *mruEvictor) OnDelete(key string) {
	e.mu.Lock()
	if el, ok := e.items[key]; ok {
		e.order.Remove(el)
		delete(e.items, key)
	}
	e.mu.Unlock()
}

func (e *mruEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return evictFrontKeys(e.order, e.items, count)
}

func (e *mruEvictor) Reset() {
	e.mu.Lock()
	e.items = make(map[string]*list.Element, 64)
	e.order.Init()
	e.mu.Unlock()
}

type randomEvictor struct {
	*mapSliceEvictor
	rng *rand.Rand
}

func newRandom(maxBytes int64) *randomEvictor {
	return &randomEvictor{
		mapSliceEvictor: newMapSliceEvictor(maxBytes),
		rng:             rand.New(rand.NewPCG(0, 0)),
	}
}
func (e *randomEvictor) OnAccess(_ string, _ *Entry) {}
func (e *randomEvictor) OnAdd(key string, _ *Entry) {
	e.mu.Lock()
	e.addKey(key)
	e.mu.Unlock()
}

func (e *randomEvictor) OnDelete(key string) {
	e.mu.Lock()
	e.removeKey(key)
	e.mu.Unlock()
}

func (e *randomEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	victims := make([]string, 0, count)
	for i := 0; i < count && len(e.keys) > 0; i++ {
		idx := e.rng.IntN(len(e.keys))
		key := e.keys[idx]
		e.removeKey(key)
		victims = append(victims, key)
	}
	return victims
}

func (e *randomEvictor) Reset() {
	e.mu.Lock()
	e.keys = e.keys[:0]
	e.keyIndex = make(map[string]int, 64)
	e.size = 0
	e.mu.Unlock()
}

type tinyLFUEvictor struct {
	mu       sync.Mutex
	keys     []string
	keyIndex map[string]int
	entries  map[string]*Entry
	rng      *rand.Rand
	maxBytes int64
	size     int
}

func newTinyLFU(maxBytes int64) *tinyLFUEvictor {
	return &tinyLFUEvictor{
		keys:     make([]string, 0, 64),
		keyIndex: make(map[string]int, 64),
		entries:  make(map[string]*Entry, 64),
		rng: rand.New(
			rand.NewPCG(1, 0),
		),
		maxBytes: maxBytes,
	}
}

func (e *tinyLFUEvictor) OnAccess(key string, entry *Entry) {
	e.mu.Lock()
	if entry != nil {
		e.entries[key] = entry
	}
	e.mu.Unlock()
}

func (e *tinyLFUEvictor) OnAdd(key string, entry *Entry) {
	e.mu.Lock()
	if _, exists := e.keyIndex[key]; !exists {
		e.keyIndex[key] = len(e.keys)
		e.keys = append(e.keys, key)
		e.size++
	}
	if entry != nil {
		e.entries[key] = entry
	}
	e.mu.Unlock()
}

func (e *tinyLFUEvictor) OnDelete(key string) {
	e.mu.Lock()
	idx, ok := e.keyIndex[key]
	if !ok {
		e.mu.Unlock()
		return
	}
	last := len(e.keys) - 1
	if idx != last {
		lastKey := e.keys[last]
		e.keys[idx] = lastKey
		e.keyIndex[lastKey] = idx
	}
	e.keys = e.keys[:last]
	delete(e.keyIndex, key)
	delete(e.entries, key)
	e.size--
	e.mu.Unlock()
}

func (e *tinyLFUEvictor) Evict(count int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	victims := make([]string, 0, count)
	for i := 0; i < count && len(e.keys) > 0; i++ {
		victim := e.pickVictim()
		if victim == "" {
			break
		}
		idx := e.keyIndex[victim]
		last := len(e.keys) - 1
		if idx != last {
			lastKey := e.keys[last]
			e.keys[idx] = lastKey
			e.keyIndex[lastKey] = idx
		}
		e.keys = e.keys[:last]
		delete(e.keyIndex, victim)
		delete(e.entries, victim)
		e.size--
		victims = append(victims, victim)
	}
	return victims
}

func (e *tinyLFUEvictor) pickVictim() string {
	n := len(e.keys)
	if n == 0 {
		return ""
	}
	sampleSize := 5
	if n < sampleSize {
		sampleSize = n
	}
	victim := ""
	var minFreq int64 = -1
	for i := 0; i < sampleSize; i++ {
		key := e.keys[e.rng.IntN(n)]
		var freq int64
		if ent, ok := e.entries[key]; ok {
			freq = ent.GetFrequency()
		}
		if minFreq < 0 || freq < minFreq {
			minFreq = freq
			victim = key
		}
	}
	return victim
}

func (e *tinyLFUEvictor) Reset() {
	e.mu.Lock()
	e.keys = e.keys[:0]
	e.keyIndex = make(map[string]int, 64)
	e.entries = make(map[string]*Entry, 64)
	e.size = 0
	e.mu.Unlock()
}
