// Package eviction provides pluggable eviction policies for cache backends.
// Each policy implements the Evictor interface and tracks access patterns to
// determine which entries should be evicted when the cache is full.
package eviction

import (
	"github.com/os-gomod/cache/v2/memory"
)

// Evictor defines the interface for cache eviction policies. Implementations
// track access patterns and determine which key to evict when the cache
// needs to make room for new entries.
type Evictor interface {
	// Name returns the human-readable name of the eviction policy.
	Name() string

	// OnAccess is called when an entry is read from the cache.
	// Implementations may use this to update recency or frequency data.
	OnAccess(key string, e *memory.Entry)

	// OnAdd is called when a new entry is added to the cache.
	OnAdd(key string, e *memory.Entry)

	// OnRemove is called when an entry is explicitly deleted from the cache.
	OnRemove(key string)

	// Evict returns the key of the entry that should be evicted and true,
	// or ("", false) if no eviction candidate is available.
	Evict() (string, bool)

	// Reset clears all internal state.
	Reset()
}

// Policy identifies a specific eviction algorithm.
type Policy int

const (
	// PolicyLRU evicts the least recently used entry.
	PolicyLRU Policy = iota
	// PolicyLFU evicts the least frequently used entry.
	PolicyLFU
	// PolicyFIFO evicts the oldest entry (first in, first out).
	PolicyFIFO
	// PolicyLIFO evicts the newest entry (last in, first out).
	PolicyLIFO
	// PolicyMRU evicts the most recently used entry.
	PolicyMRU
	// PolicyRandom evicts a random entry.
	PolicyRandom
	// PolicyTinyLFU uses a frequency sketch for approximation.
	PolicyTinyLFU
)

// String returns the human-readable name of the eviction policy.
func (p Policy) String() string {
	switch p {
	case PolicyLRU:
		return "lru"
	case PolicyLFU:
		return "lfu"
	case PolicyFIFO:
		return "fifo"
	case PolicyLIFO:
		return "lifo"
	case PolicyMRU:
		return "mru"
	case PolicyRandom:
		return "random"
	case PolicyTinyLFU:
		return "tinylfu"
	default:
		return "unknown"
	}
}

// New creates an Evictor for the given policy and max byte capacity.
// The maxBytes parameter is used by some policies (e.g., TinyLFU) to
// configure their internal data structures.
func New(policy Policy, maxBytes int64) Evictor {
	switch policy {
	case PolicyLRU:
		return newLRU()
	case PolicyLFU:
		return newLFU()
	case PolicyFIFO:
		return newFIFO()
	case PolicyLIFO:
		return newLIFO()
	case PolicyMRU:
		return newMRU()
	case PolicyRandom:
		return newRandom()
	case PolicyTinyLFU:
		return newTinyLFU(maxBytes)
	default:
		return newLRU()
	}
}
