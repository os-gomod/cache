package eviction

import (
	"sync"

	"github.com/os-gomod/cache/v2/memory"
)

// TinyLFU implements a simplified TinyLFU eviction approximation. It uses a
// Count-Min Sketch for frequency counting and a Doorkeeper (Bloom filter
// variant) to prevent one-hit wonders from polluting the frequency sketch.
//
// TinyLFU combines the recency of LRU with the frequency awareness of LFU
// while maintaining low memory overhead.
type tinyLFU struct {
	mu          sync.Mutex
	items       map[string]*tinyEntry
	order       []string // simple FIFO ordering for recency
	maxBytes    int64
	totalBytes  int64
	cms         *countMinSketch
	doorkeeper  *doorkeeper
	sampleCount int64
	windowSize  int64
}

// tinyEntry wraps an entry with its tracking data.
type tinyEntry struct {
	key   string
	e     *memory.Entry
	index int // position in order slice
}

const (
	// DefaultWindowSize is the number of items in the admission window.
	defaultWindowSize = 10000
	// CmsDepth is the number of hash functions/rows in the Count-Min Sketch.
	cmsDepth = 4
	// CmsWidth is the number of counters per row in the Count-Min Sketch.
	cmsWidth = 1 << 16 // 65536
	// DoorkeeperBits is the number of bits in the doorkeeper bloom filter.
	doorkeeperBits = 1 << 20 // ~1M bits = ~128KB
)

// countMinSketch is a probabilistic frequency counter that provides an
// approximate count of how many times each key has been accessed.
type countMinSketch struct {
	table [cmsDepth][cmsWidth]int64
}

func newCountMinSketch() *countMinSketch {
	return &countMinSketch{}
}

// increment adds one to the estimated count for the key.
func (c *countMinSketch) increment(key string) {
	for i := range cmsDepth {
		idx := cmsHash(key, i) % cmsWidth
		c.table[i][idx]++
	}
}

// estimate returns the estimated frequency count for the key.
func (c *countMinSketch) estimate(key string) int64 {
	minCount := int64(1<<63 - 1)
	for i := range cmsDepth {
		idx := cmsHash(key, i) % cmsWidth
		if c.table[i][idx] < minCount {
			minCount = c.table[i][idx]
		}
	}
	return minCount
}

// reset sets all counters to zero.
func (c *countMinSketch) reset() {
	for i := range cmsDepth {
		for j := range cmsWidth {
			c.table[i][j] = 0
		}
	}
}

// cmsHash computes a hash for the key at the given depth level.
func cmsHash(key string, depth int) uint32 {
	h := uint32(2166136261) // FNV-1a offset
	h ^= uint32(depth)
	for i := range len(key) {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

// doorkeeper is a lightweight Bloom filter variant that filters out one-hit
// wonders. Items must be seen at least twice (passing through the door)
// before being counted in the frequency sketch.
type doorkeeper struct {
	bits []uint64
	size int
}

func newDoorkeeper(size int) *doorkeeper {
	return &doorkeeper{
		bits: make([]uint64, (size+63)/64),
		size: size,
	}
}

// testAndSet checks if the key has been seen before. Returns true if this
// is the first time the key has been seen (passing through the door).
func (d *doorkeeper) testAndSet(key string) bool {
	idx := doorHash(key) % uint64(d.size)
	word := idx / 64
	bit := idx % 64
	if d.bits[word]&(1<<bit) != 0 {
		return false // already seen
	}
	d.bits[word] |= 1 << bit
	return true // first time seeing this key
}

// doorHash computes a hash for the doorkeeper bloom filter.
func doorHash(key string) uint64 {
	h := uint64(2166136261)
	for i := range len(key) {
		h ^= uint64(key[i])
		h *= uint64(16777619)
	}
	return h
}

// newTinyLFU creates a new TinyLFU evictor with the given max byte capacity.
func newTinyLFU(maxBytes int64) *tinyLFU {
	ws := defaultWindowSize
	if maxBytes > 0 {
		ws = int(maxBytes / 1024) // approximate window size from maxBytes
		if ws < 100 {
			ws = 100
		}
		if ws > 100000 {
			ws = 100000
		}
	}
	return &tinyLFU{
		items:      make(map[string]*tinyEntry),
		order:      make([]string, 0, ws),
		maxBytes:   maxBytes,
		cms:        newCountMinSketch(),
		doorkeeper: newDoorkeeper(doorkeeperBits),
		windowSize: int64(ws),
	}
}

// Name returns "tinylfu".
func (*tinyLFU) Name() string { return "tinylfu" }

// OnAccess updates frequency tracking for the accessed key.
func (t *tinyLFU) OnAccess(key string, e *memory.Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sampleCount++

	// Use doorkeeper to filter one-hit wonders
	if !t.doorkeeper.testAndSet(key) {
		// Not a one-hit wonder - count in CMS
		t.cms.increment(key)
	}

	if item, ok := t.items[key]; ok {
		item.e = e
	}
}

// OnAdd adds a new entry to tracking.
func (t *tinyLFU) OnAdd(key string, e *memory.Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.items[key]; ok {
		t.items[key].e = e
		return
	}

	t.totalBytes += e.Size
	t.order = append(t.order, key)
	t.items[key] = &tinyEntry{
		key:   key,
		e:     e,
		index: len(t.order) - 1,
	}

	// Periodically reset the CMS to handle frequency staleness
	if t.sampleCount > t.windowSize*2 {
		t.cms.reset()
		t.doorkeeper = newDoorkeeper(doorkeeperBits)
		t.sampleCount = 0
	}
}

// OnRemove removes the entry from tracking.
func (t *tinyLFU) OnRemove(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, ok := t.items[key]; ok {
		t.totalBytes -= item.e.Size
		delete(t.items, key)
	}
}

// Evict returns the key with the lowest estimated frequency.
func (t *tinyLFU) Evict() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.items) == 0 {
		return "", false
	}

	// Find the entry with the lowest frequency
	var victimKey string
	var victimFreq int64 = 1<<63 - 1
	victimIdx := -1

	for i := len(t.order) - 1; i >= 0; i-- {
		key := t.order[i]
		_, ok := t.items[key]
		if !ok {
			continue
		}
		freq := t.cms.estimate(key)
		if freq < victimFreq {
			victimFreq = freq
			victimKey = key
			victimIdx = i
		}
	}

	if victimIdx < 0 {
		// No valid item found; evict the first one
		for _, key := range t.order {
			if item, ok := t.items[key]; ok {
				t.totalBytes -= item.e.Size
				delete(t.items, key)
				return key, true
			}
		}
		return "", false
	}

	// Remove victim from order
	t.order = append(t.order[:victimIdx], t.order[victimIdx+1:]...)
	if item, ok := t.items[victimKey]; ok {
		t.totalBytes -= item.e.Size
	}
	delete(t.items, victimKey)
	return victimKey, true
}

// Reset clears all internal state.
func (t *tinyLFU) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.items = make(map[string]*tinyEntry)
	t.order = t.order[:0]
	t.totalBytes = 0
	t.cms.reset()
	t.doorkeeper = newDoorkeeper(doorkeeperBits)
	t.sampleCount = 0
}
