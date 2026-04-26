package eviction

import (
	"container/heap"
	"sync"

	"github.com/os-gomod/cache/v2/memory"
)

// lfu implements the Least Frequently Used eviction policy using a min-heap
// keyed by access frequency. When frequency ties occur, the entry with the
// oldest last-access time is evicted first.
type lfu struct {
	mu       sync.Mutex
	items    map[string]*lfuItem
	freqHeap *lfuHeap
	counter  int64 // monotonic counter for tie-breaking
}

// lfuItem represents a tracked entry in the LFU policy.
type lfuItem struct {
	key        string
	e          *memory.Entry
	frequency  int64
	lastAccess int64
	index      int   // heap index
	counter    int64 // insertion order for tie-breaking
}

//nolint:recvcheck // heap.Interface requires mix of value and pointer receivers per Go container/heap convention
type lfuHeap []*lfuItem

func (h lfuHeap) Len() int { return len(h) }

func (h lfuHeap) Less(i, j int) bool {
	if h[i].frequency != h[j].frequency {
		return h[i].frequency < h[j].frequency
	}
	// Tie-break by insertion order (FIFO within same frequency)
	return h[i].counter < h[j].counter
}

func (h lfuHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *lfuHeap) Push(x interface{}) {
	item := x.(*lfuItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *lfuHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1
	*h = old[:n-1]
	return item
}

// newLFU creates a new LFU evictor.
func newLFU() *lfu {
	h := &lfuHeap{}
	heap.Init(h)
	return &lfu{
		items:    make(map[string]*lfuItem),
		freqHeap: h,
	}
}

// Name returns "lfu".
func (*lfu) Name() string { return "lfu" }

// OnAccess increments the frequency counter for the key and re-heapifies.
func (l *lfu) OnAccess(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if item, ok := l.items[key]; ok {
		item.frequency++
		item.lastAccess = e.LastAccess
		item.e = e
		heap.Fix(l.freqHeap, item.index)
		return
	}

	l.addItem(key, e)
}

// OnAdd tracks a newly added entry with frequency 1.
func (l *lfu) OnAdd(key string, e *memory.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if item, ok := l.items[key]; ok {
		item.e = e
		return
	}

	l.addItem(key, e)
}

// addItem adds a new entry to the LFU tracking.
func (l *lfu) addItem(key string, e *memory.Entry) {
	l.counter++
	item := &lfuItem{
		key:        key,
		e:          e,
		frequency:  1,
		lastAccess: e.LastAccess,
		counter:    l.counter,
	}
	l.items[key] = item
	heap.Push(l.freqHeap, item)
}

// OnRemove removes the entry from tracking.
func (l *lfu) OnRemove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if item, ok := l.items[key]; ok {
		heap.Remove(l.freqHeap, item.index)
		delete(l.items, key)
	}
}

// Evict returns the least frequently used key.
func (l *lfu) Evict() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.freqHeap.Len() == 0 {
		return "", false
	}

	item := heap.Pop(l.freqHeap).(*lfuItem)
	delete(l.items, item.key)
	return item.key, true
}

// Reset clears all internal state.
func (l *lfu) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.items = make(map[string]*lfuItem)
	l.freqHeap = &lfuHeap{}
	heap.Init(l.freqHeap)
	l.counter = 0
}
