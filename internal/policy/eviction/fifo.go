package eviction

import (
	"math/rand"
	"sync"

	"github.com/os-gomod/cache/v2/memory"
)

// fifo implements the First In, First Out eviction policy using a slice-based
// queue. Entries are evicted in the order they were added, regardless of
// access patterns.
type fifo struct {
	mu    sync.Mutex
	items map[string]int           // key -> index in queue
	queue []string                 // ordered list of keys (oldest first)
	elems map[string]*memory.Entry // key -> entry reference
}

// newFIFO creates a new FIFO evictor.
func newFIFO() *fifo {
	return &fifo{
		items: make(map[string]int),
		elems: make(map[string]*memory.Entry),
	}
}

// Name returns "fifo".
func (*fifo) Name() string { return "fifo" }

// OnAccess records the access but does not change eviction order.
func (f *fifo) OnAccess(key string, e *memory.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[key]; ok {
		f.elems[key] = e
	} else {
		// Not tracked - add it
		f.items[key] = len(f.queue)
		f.queue = append(f.queue, key)
		f.elems[key] = e
	}
}

// OnAdd appends the entry to the end of the queue.
func (f *fifo) OnAdd(key string, e *memory.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.items[key]; ok {
		f.elems[key] = e
		return
	}

	f.items[key] = len(f.queue)
	f.queue = append(f.queue, key)
	f.elems[key] = e
}

// OnRemove removes the entry from the queue.
func (f *fifo) OnRemove(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.items[key]
	if !ok {
		return
	}

	delete(f.items, key)
	delete(f.elems, key)

	// Remove from queue by swapping with last element
	if idx < len(f.queue)-1 {
		last := f.queue[len(f.queue)-1]
		f.queue[idx] = last
		f.items[last] = idx
	}
	f.queue = f.queue[:len(f.queue)-1]
}

// Evict returns the oldest (first-in) key.
func (f *fifo) Evict() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.queue) == 0 {
		return "", false
	}

	key := f.queue[0]
	f.queue = f.queue[1:]
	delete(f.items, key)
	delete(f.elems, key)
	return key, true
}

// Reset clears all internal state.
func (f *fifo) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.items = make(map[string]int)
	f.queue = nil
	f.elems = make(map[string]*memory.Entry)
}

// randomEvictor implements the Random eviction policy, which selects a
// random entry for eviction. This is simple and avoids access-pattern bias.
type randomEvictor struct {
	mu    sync.RWMutex
	items map[string]*memory.Entry
	rng   *rand.Rand
}

// newRandom creates a new random evictor.
func newRandom() *randomEvictor {
	return &randomEvictor{
		items: make(map[string]*memory.Entry),
		rng:   rand.New(rand.NewSource(42)),
	}
}

// Name returns "random".
func (*randomEvictor) Name() string { return "random" }

// OnAccess tracks the entry.
func (r *randomEvictor) OnAccess(key string, e *memory.Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key] = e
}

// OnAdd tracks the entry.
func (r *randomEvictor) OnAdd(key string, e *memory.Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key] = e
}

// OnRemove removes the entry from tracking.
func (r *randomEvictor) OnRemove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, key)
}

// Evict returns a random key.
func (r *randomEvictor) Evict() (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(r.items)
	if n == 0 {
		return "", false
	}

	// Collect keys into a slice
	keys := make([]string, 0, n)
	for k := range r.items {
		keys = append(keys, k)
	}

	idx := r.rng.Intn(n)
	key := keys[idx]
	delete(r.items, key)
	return key, true
}

// Reset clears all internal state.
func (r *randomEvictor) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[string]*memory.Entry)
}
