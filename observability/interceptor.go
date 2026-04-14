// Package observability provides a uniform middleware interceptor chain for
// cache backends. All observability — tracing, metrics, logging — flows
// through Interceptor implementations. No backend calls obs directly.
package observability

import (
	"context"
	"time"
)

// Op represents a cache operation being observed.
type Op struct {
	// Backend identifies the cache backend ("memory", "redis", "layered", "resilience").
	Backend string
	// Name is the operation name ("get", "set", "delete", "get_multi", etc.).
	Name string
	// Key is the primary key for single-key operations (empty for batch ops).
	Key string
	// KeyCount is the number of keys involved (for batch operations).
	KeyCount int
}

// Result holds the post-execution outcome of a cache operation.
type Result struct {
	// Hit indicates whether the operation resulted in a cache hit.
	// Only meaningful for read operations (get, get_multi, etc.).
	Hit bool
	// Latency is the duration of the operation.
	Latency time.Duration
	// ByteSize is the size of the value in bytes (0 if not applicable).
	ByteSize int
	// Err is the error returned by the operation, if any.
	Err error
}

// Interceptor is a middleware hook called around every cache operation.
// Implementations must be safe for concurrent use.
type Interceptor interface {
	// Before is called before the cache operation executes. It may enrich
	// the context (e.g., with a tracing span) and must return the
	// (possibly modified) context.
	Before(ctx context.Context, op Op) context.Context
	// After is called after the cache operation completes, regardless of
	// whether it succeeded or failed. Implementations must not panic.
	After(ctx context.Context, op Op, result Result)
}

// NopInterceptor is an Interceptor that does nothing.
type NopInterceptor struct{}

func (NopInterceptor) Before(ctx context.Context, _ Op) context.Context { return ctx }
func (NopInterceptor) After(_ context.Context, _ Op, _ Result)          {}

// Chain composes multiple interceptors into one. Before calls proceed in
// order (1, 2, 3); After calls proceed in reverse order (3, 2, 1) so
// that each interceptor's After runs in the same stack frame as its Before.
type Chain struct {
	interceptors []Interceptor
}

// NewChain creates a Chain from the given interceptors. If none are
// provided, the Chain behaves as a no-op.
func NewChain(interceptors ...Interceptor) *Chain {
	return &Chain{interceptors: interceptors}
}

// Before calls each interceptor's Before in order.
func (c *Chain) Before(ctx context.Context, op Op) context.Context {
	for _, ic := range c.interceptors {
		ctx = ic.Before(ctx, op)
	}
	return ctx
}

// After calls each interceptor's After in reverse order.
func (c *Chain) After(ctx context.Context, op Op, result Result) {
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		c.interceptors[i].After(ctx, op, result)
	}
}

// IsEmpty returns true if the chain has no interceptors.
func (c *Chain) IsEmpty() bool {
	return len(c.interceptors) == 0
}

// Append returns a new Chain with additional interceptors appended.
func (c *Chain) Append(interceptors ...Interceptor) *Chain {
	combined := make([]Interceptor, 0, len(c.interceptors)+len(interceptors))
	combined = append(combined, c.interceptors...)
	combined = append(combined, interceptors...)
	return &Chain{interceptors: combined}
}

// nopChain is a shared no-op chain for backends with no interceptors.
var nopChain = NewChain()

// NopChain returns a shared no-op Chain.
func NopChain() *Chain { return nopChain }
