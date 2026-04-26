// Package middleware provides the interceptor chain and individual middleware
// components for cache operations. Middleware can observe, modify, retry, rate-limit,
// and trace cache operations without modifying backend implementations.
//
// The Chain manages an ordered list of middleware and provides Before/After hooks
// for operation observability, as well as a Build method for composing handlers.
package middleware

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// Handler is a function that processes a cache operation. It receives a context
// and returns an error if the operation failed.
type Handler func(ctx context.Context, op contracts.Operation) error

// Middleware is a function that wraps a Handler with additional behavior.
// Middleware are composed in a chain; each middleware can inspect the operation,
// modify the context, short-circuit execution, or perform post-processing.
type Middleware func(Handler) Handler

// nopChainInstance is the shared no-op chain singleton.
var nopChainInstance = &Chain{}

// NopChain returns a shared no-op Chain that performs no middleware processing.
// Use this when no middleware is needed but a non-nil Chain is required.
func NopChain() *Chain {
	return nopChainInstance
}

// Chain manages an ordered list of middleware for cache operations. It provides
// both the Handler composition pattern (via Build) and the Before/After hook
// pattern (via Before/After methods) for operation observability.
type Chain struct {
	middlewares []Middleware
	beforeHooks []func(ctx context.Context, op contracts.Operation) context.Context
	afterHooks  []func(ctx context.Context, op contracts.Operation, result contracts.Result)
}

// NewChain creates a new Chain with the given middleware. The middleware are
// applied in the order provided; the first middleware is the outermost wrapper.
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// BuildChain creates a Chain from a list of Interceptors, registering each
// interceptor's Before method as a before-hook and After method as an
// after-hook on the chain.
func BuildChain(interceptors ...Interceptor) *Chain {
	c := &Chain{}
	for _, i := range interceptors {
		c.beforeHooks = append(c.beforeHooks, i.Before)
		c.afterHooks = append(c.afterHooks, i.After)
	}
	return c
}

// Use adds one or more middleware to the chain. The middleware are appended to
// the existing list and applied after any previously added middleware.
// Use returns the chain for fluent chaining.
func (c *Chain) Use(m ...Middleware) *Chain {
	c.middlewares = append(c.middlewares, m...)
	return c
}

// Build composes all middleware around the final handler, returning a new
// Handler that executes each middleware in order. The final handler is the
// innermost function in the call chain.
//
// If no middleware have been registered, the final handler is returned unchanged.
func (c *Chain) Build(final Handler) Handler {
	// Apply middleware in reverse order so the first middleware is outermost
	h := final
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h
}

// Before runs all registered before hooks in order, threading the context
// through each hook. This is used for pre-operation processing such as
// setting up trace spans or validation.
func (c *Chain) Before(ctx context.Context, op contracts.Operation) context.Context {
	for _, hook := range c.beforeHooks {
		ctx = hook(ctx, op)
	}
	return ctx
}

// After runs all registered after hooks in order with the operation result.
// This is used for post-operation processing such as metrics recording,
// logging, and tracing completion.
func (c *Chain) After(ctx context.Context, op contracts.Operation, result contracts.Result) {
	for _, hook := range c.afterHooks {
		hook(ctx, op, result)
	}
}

// Interceptor is the legacy interceptor interface for Before/After hooks.
// It is retained for backward compatibility. New code should use Middleware.
type Interceptor interface {
	// Before is called before a cache operation executes. It may return a
	// modified context (e.g., with a trace span attached).
	Before(ctx context.Context, op contracts.Operation) context.Context

	// After is called after a cache operation completes, regardless of error.
	After(ctx context.Context, op contracts.Operation, result contracts.Result)
}

// NopInterceptor is a no-op interceptor that does nothing.
type NopInterceptor struct{}

// Before returns the context unchanged.
func (NopInterceptor) Before(ctx context.Context, _ contracts.Operation) context.Context {
	return ctx
}

// After is a no-op.
func (NopInterceptor) After(_ context.Context, _ contracts.Operation, _ contracts.Result) {}

// TrackedOp wraps a cache operation call with Before/After middleware hooks.
// It returns the typed result, the contracts.Result for middleware consumption,
// and any error encountered.
func TrackedOp[T any](
	ctx context.Context,
	chain *Chain,
	op contracts.Operation,
	fn func(ctx context.Context) (T, error),
) (T, contracts.Result, error) {
	start := time.Now()
	ctx = chain.Before(ctx, op)

	var resultMeta contracts.Result
	defer func() {
		resultMeta.Latency = time.Since(start)
		chain.After(ctx, op, resultMeta)
	}()

	result, err := fn(ctx)
	resultMeta.Err = err
	return result, resultMeta, err
}
