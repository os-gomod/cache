// Package runtime provides the unified execution runtime for cache operations.
// The Executor is the central component that coordinates middleware execution,
// key validation, statistics collection, and timing for all cache operations.
package runtime

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/middleware"
)

// KeyValidator is a function that validates a cache key for a given operation.
// Returns nil if the key is valid, or an error describing the validation failure.
type KeyValidator func(op, key string) error

// StatsCollector records cache operation statistics.
type StatsCollector interface {
	Hit()
	Miss()
	Set()
	Delete()
	Error()
	Eviction()
}

// Clock provides time-related functionality, allowing injection of a fake clock
// for testing.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// realClock is the default Clock implementation using the standard time package.
type realClock struct{}

func (*realClock) Now() time.Time                  { return time.Now() }
func (*realClock) Since(t time.Time) time.Duration { return time.Since(t) }

// noOpStats is a no-op StatsCollector that discards all statistics.
type noOpStats struct{}

func (*noOpStats) Hit()      {}
func (*noOpStats) Miss()     {}
func (*noOpStats) Set()      {}
func (*noOpStats) Delete()   {}
func (*noOpStats) Error()    {}
func (*noOpStats) Eviction() {}

// defaultValidator is the default key validator that rejects empty keys.
func defaultValidator(_op, key string) error {
	if key == "" {
		return &cacheErr{msg: "key must not be empty"}
	}
	return nil
}

// cacheErr is a simple error type for runtime validation errors.
type cacheErr struct {
	msg string
}

func (e *cacheErr) Error() string { return e.msg }

// Executor is the unified execution runtime for cache operations. It coordinates
// middleware execution, key validation, statistics collection, and timing.
//
// The Executor provides three execution methods:
//   - Execute: for void operations (set, delete, clear)
//   - ExecuteResult: for operations returning contracts.Result (get)
//   - ExecuteTyped: for operations returning an arbitrary typed result
//
// All methods run the middleware chain before and after the actual operation,
// capture latency, and handle errors consistently.
type Executor struct {
	chain     *middleware.Chain
	validator KeyValidator
	stats     StatsCollector
	clock     Clock
}

// New creates a new Executor with the given options. If no options are provided,
// sensible defaults are used (nil chain, default validator, no-op stats, real clock).
func New(opts ...ExecutorOption) *Executor {
	e := &Executor{
		validator: defaultValidator,
		stats:     &noOpStats{},
		clock:     &realClock{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs a void cache operation through the middleware chain. The
// middleware Before hooks are called with the operation, the function fn
// is executed, and middleware After hooks are called with the result.
//
// If a key validator is configured and the operation has a non-empty key,
// the key is validated before execution. If validation fails, the error is
// returned immediately without calling fn.
func (e *Executor) Execute(
	ctx context.Context,
	op contracts.Operation,
	fn func(ctx context.Context) error,
) error {
	if e.chain != nil {
		ctx = e.chain.Before(ctx, op)
	}

	if err := e.validateKey(op); err != nil {
		e.recordError(op, err)
		return err
	}

	start := e.clock.Now()
	err := fn(ctx)
	latency := e.clock.Since(start)

	result := contracts.Result{
		Latency: latency,
		Err:     err,
	}

	if e.chain != nil {
		e.chain.After(ctx, op, result)
	}

	if err != nil {
		e.recordError(op, err)
		return err
	}

	e.recordStats(op)
	return nil
}

// ExecuteResult runs a cache operation that returns a contracts.Result through
// the middleware chain. The middleware Before hooks are called, the function fn
// is executed, and middleware After hooks are called with the result.
func (e *Executor) ExecuteResult(
	ctx context.Context,
	op contracts.Operation,
	fn func(ctx context.Context) (contracts.Result, error),
) (contracts.Result, error) {
	if e.chain != nil {
		ctx = e.chain.Before(ctx, op)
	}

	if err := e.validateKey(op); err != nil {
		e.recordError(op, err)
		return contracts.Result{Err: err}, err
	}

	start := e.clock.Now()
	result, err := fn(ctx)
	if err != nil {
		result.Latency = e.clock.Since(start)
		result.Err = err
	} else {
		result.Latency = e.clock.Since(start)
	}

	if e.chain != nil {
		e.chain.After(ctx, op, result)
	}

	if err != nil {
		e.recordError(op, err)
		return result, err
	}

	e.recordStats(op)
	return result, nil
}

// ExecuteTyped runs a cache operation that returns an arbitrary typed result
// through the middleware chain. The result is wrapped in a contracts.Result
// for middleware processing.
//
//nolint:revive // Executor is intentionally the first parameter for method-style chaining
func ExecuteTyped[T any](
	e *Executor,
	ctx context.Context,
	op contracts.Operation,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	if e.chain != nil {
		ctx = e.chain.Before(ctx, op)
	}

	if err := e.validateKey(op); err != nil {
		e.recordError(op, err)
		var zero T
		return zero, err
	}

	start := e.clock.Now()
	value, err := fn(ctx)
	latency := e.clock.Since(start)

	result := contracts.Result{
		Latency: latency,
		Err:     err,
	}

	if e.chain != nil {
		e.chain.After(ctx, op, result)
	}

	if err != nil {
		e.recordError(op, err)
		var zero T
		return zero, err
	}

	e.recordStats(op)
	return value, nil
}

// validateKey checks the operation key using the configured validator.
func (e *Executor) validateKey(op contracts.Operation) error {
	if e.validator == nil || op.Key == "" {
		return nil
	}
	return e.validator(op.Name, op.Key)
}

// recordStats updates the stats collector based on the operation type.
func (e *Executor) recordStats(op contracts.Operation) {
	if e.stats == nil {
		return
	}
	switch op.Name {
	case "get", "getmulti", "exists":
		// Stats updated by caller based on hit/miss result
	case "set", "setmulti", "setnx", "cas":
		e.stats.Set()
	case "delete", "deletemulti":
		e.stats.Delete()
	}
}

// recordError increments the error counter in the stats collector.
func (e *Executor) recordError(_op contracts.Operation, _err error) {
	if e.stats == nil {
		return
	}
	e.stats.Error()
}
