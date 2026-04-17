// Package chain provides shared utilities for building observability chains
// used across all cache backend implementations (memory, redis, layer, resilience).
package chain

import "github.com/os-gomod/cache/observability"

// BuildChain creates an observability.Chain from a raw interceptors value.
// If interceptors is a []observability.Interceptor with elements, a real chain is returned;
// otherwise NopChain is returned. This is the single source of truth for chain construction
// used by memory, redis, layer, and resilience caches.
func BuildChain(interceptors any) *observability.Chain {
	var ics []observability.Interceptor
	if ic, ok := interceptors.([]observability.Interceptor); ok {
		ics = append(ics, ic...)
	}
	if len(ics) > 0 {
		return observability.NewChain(ics...)
	}
	return observability.NopChain()
}

// SetInterceptors replaces the interceptors on a cache by rebuilding the chain.
// The new chain pointer is returned.
func SetInterceptors(_ *observability.Chain, interceptors []observability.Interceptor) *observability.Chain {
	if len(interceptors) > 0 {
		return observability.NewChain(interceptors...)
	}
	return observability.NopChain()
}
