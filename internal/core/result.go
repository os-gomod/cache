package core

import "time"

// ExecutionResult captures the outcome of a generic cache execution,
// including the typed value, any error, latency measurement, hit status,
// and size information. It is used by the Executor for type-safe operation
// results.
type ExecutionResult[T any] struct {
	// Value is the result value of the operation.
	Value T

	// Err is any error that occurred during execution. When Err is non-nil,
	// Value should be considered invalid.
	Err error

	// Latency is the wall-clock duration of the operation.
	Latency time.Duration

	// Hit indicates whether the operation resulted in a cache hit.
	Hit bool

	// Size is the approximate byte size of the result.
	Size int
}

// Success creates an ExecutionResult representing a successful operation.
func Success[T any](value T, latency time.Duration) ExecutionResult[T] {
	return ExecutionResult[T]{
		Value:   value,
		Latency: latency,
		Hit:     true,
	}
}

// Failure creates an ExecutionResult representing a failed operation.
func Failure[T any](err error, latency time.Duration) ExecutionResult[T] {
	return ExecutionResult[T]{
		Err:     err,
		Latency: latency,
		Hit:     false,
	}
}

// OK returns true if the execution completed without error.
func (r ExecutionResult[T]) OK() bool {
	return r.Err == nil
}

// Bytes returns the size of the value when T is []byte, otherwise 0.
// This is a convenience method for the common case of byte-slice results.
func (r ExecutionResult[T]) Bytes() int {
	if r.Size > 0 {
		return r.Size
	}
	if slice, ok := any(r.Value).([]byte); ok {
		return len(slice)
	}
	return 0
}
