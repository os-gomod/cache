// Package cache defines the canonical backend interfaces that all cache
// implementations must satisfy. No other package in this module may define
// its own cache interface — they import from here or satisfy via duck typing.
package cache

import "github.com/os-gomod/cache/internal/backendiface"

type Backend = backendiface.Backend

// AtomicBackend extends Backend with compare-and-swap semantics.
type AtomicBackend = backendiface.AtomicBackend

// ScanBackend extends Backend with key enumeration.
type ScanBackend = backendiface.ScanBackend
