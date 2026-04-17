// Package cache provides a unified, high-performance caching library with support
// for in-memory, Redis, and layered (L1+L2) cache backends.
package cache

import "github.com/os-gomod/cache/internal/backendiface"

// Backend defines the core read/write interface for all cache implementations.
type Backend = backendiface.Backend

// AtomicBackend extends Backend with atomic operations such as CompareAndSwap,
// SetNX, Increment, and GetSet.
type AtomicBackend = backendiface.AtomicBackend

// ScanBackend extends Backend with key scanning, size counting, and clearing.
type ScanBackend = backendiface.ScanBackend
