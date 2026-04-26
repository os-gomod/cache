// Package layered implements a two-tier (L1+L2) cache store that combines
// a fast in-memory cache (L1) with a shared distributed cache (L2). It
// supports automatic promotion of L2 hits to L1 and optional async
// write-back for improved write performance.
package layered

import (
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/middleware"
)

// Option is a functional option for configuring the layered Store.
type Option func(*config)

// WithL1 sets the L1 (fast, in-memory) cache backend. This is required.
func WithL1(l1 contracts.Cache) Option {
	return func(c *config) { c.l1 = l1 }
}

// WithL2 sets the L2 (slow, distributed) cache backend. This is required.
func WithL2(l2 contracts.Cache) Option {
	return func(c *config) { c.l2 = l2 }
}

// WithPromoteOnHit enables or disables automatic promotion of L2 hits to L1.
// Default: true.
func WithPromoteOnHit(b bool) Option {
	return func(c *config) { c.promoteOnHit = b }
}

// WithWriteBack enables write-behind mode where L2 writes are queued and
// processed asynchronously. Default: false.
func WithWriteBack(b bool) Option {
	return func(c *config) { c.writeBack = b }
}

// WithWriteBackQueueSize sets the capacity of the async write-back queue.
// Default: 10000.
func WithWriteBackQueueSize(n int) Option {
	return func(c *config) { c.wbQueueSize = n }
}

// WithWriteBackWorkers sets the number of worker goroutines for the
// write-back queue. Default: 4.
func WithWriteBackWorkers(n int) Option {
	return func(c *config) { c.wbWorkers = n }
}

// WithNegativeTTL sets the TTL for negative cache entries (keys that were
// not found in L2). This prevents thundering herd on missing keys.
// A zero value disables negative caching. Default: 30 seconds.
func WithNegativeTTL(d time.Duration) Option {
	return func(c *config) { c.negativeTTL = d }
}

// WithInterceptors sets the observability interceptors for the layered store.
func WithInterceptors(i ...middleware.Interceptor) Option {
	return func(c *config) { c.interceptors = i }
}

// config holds the validated configuration for the layered Store.
type config struct {
	l1           contracts.Cache
	l2           contracts.Cache
	promoteOnHit bool
	writeBack    bool
	wbQueueSize  int
	wbWorkers    int
	negativeTTL  time.Duration
	interceptors []middleware.Interceptor
}

// defaultConfig returns the default layered configuration.
func defaultConfig() config {
	return config{
		promoteOnHit: true,
		writeBack:    false,
		wbQueueSize:  10000,
		wbWorkers:    4,
		negativeTTL:  30 * time.Second,
	}
}
