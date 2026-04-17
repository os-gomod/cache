// Package observability provides interceptors for cache operation metrics, tracing,
// and chaining. Interceptors can observe cache operations without modifying
// the cache implementations themselves.
package observability

import (
	"context"
	"time"
)

// Op describes a cache operation for observability purposes.
type Op struct {
	Backend  string
	Name     string
	Key      string
	KeyCount int
}

// Result describes the outcome of a cache operation.
type Result struct {
	Hit      bool
	Latency  time.Duration
	ByteSize int
	Err      error
}

// Interceptor observes cache operations. Implementations may record metrics,
// create trace spans, or perform other cross-cutting concerns.
type Interceptor interface {
	Before(ctx context.Context, op Op) context.Context
	After(ctx context.Context, op Op, result Result)
}

// NopInterceptor is a no-op interceptor that does nothing.
type NopInterceptor struct{}

// Before returns the context unchanged.
func (NopInterceptor) Before(ctx context.Context, _ Op) context.Context { return ctx }

// After is a no-op.
func (NopInterceptor) After(_ context.Context, _ Op, _ Result) {}

// Chain is an ordered collection of interceptors executed in sequence.
type Chain struct {
	interceptors []Interceptor
}

// NewChain creates a new interceptor chain from the given interceptors.
func NewChain(interceptors ...Interceptor) *Chain {
	return &Chain{interceptors: interceptors}
}

// Before runs all interceptors' Before hooks in order.
func (c *Chain) Before(ctx context.Context, op Op) context.Context {
	for _, ic := range c.interceptors {
		ctx = ic.Before(ctx, op)
	}
	return ctx
}

// After runs all interceptors' After hooks in reverse order.
func (c *Chain) After(ctx context.Context, op Op, result Result) {
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		c.interceptors[i].After(ctx, op, result)
	}
}

// IsEmpty reports whether the chain has no interceptors.
func (c *Chain) IsEmpty() bool {
	return len(c.interceptors) == 0
}

// Append returns a new chain with the given interceptors added to the end.
func (c *Chain) Append(interceptors ...Interceptor) *Chain {
	combined := make([]Interceptor, 0, len(c.interceptors)+len(interceptors))
	combined = append(combined, c.interceptors...)
	combined = append(combined, interceptors...)
	return &Chain{interceptors: combined}
}

var nopChain = NewChain()

// NopChain returns a shared no-op chain that performs no interception.
func NopChain() *Chain { return nopChain }
