package errors

import (
	"time"

	stdliberrors "errors"
)

// stdlibAs wraps errors.As for use within the errors package.
func stdlibAs(err error, target interface{}) bool { return stdliberrors.As(err, target) }

// stdlibIs wraps errors.Is for use within the errors package.
func stdlibIs(target, err error) bool { return stdliberrors.Is(err, target) }

// ErrorFactory provides methods for creating structured cache errors.
// Implementations may add additional context, logging, or tracing
// information to each error.
//
//nolint:interfacebloat // ErrorFactory intentionally provides a comprehensive error creation API
type ErrorFactory interface {
	// New creates a CacheError with the given code, operation, key, message, and cause.
	New(code ErrorCode, op, key, msg string, cause error) *CacheError

	// Wrap creates a CacheError wrapping an existing error, automatically classifying
	// the error code based on the cause.
	Wrap(op, key string, cause error) *CacheError

	// NotFound returns a CacheError indicating the key was not found.
	NotFound(op, key string) *CacheError

	// InvalidKey returns a CacheError for invalid key input.
	InvalidKey(op, key string, cause error) *CacheError

	// EmptyKey returns a CacheError for an empty key.
	EmptyKey(op string) *CacheError

	// Connection returns a CacheError for backend connection failures.
	Connection(op string, cause error) *CacheError

	// Timeout returns a CacheError for operation timeouts.
	Timeout(op string) *CacheError

	// SerializeError returns a CacheError for serialization failures.
	SerializeError(op string, cause error) *CacheError

	// DeserializeError returns a CacheError for deserialization failures.
	DeserializeError(op string, cause error) *CacheError

	// InvalidConfig returns a CacheError for configuration validation failures.
	InvalidConfig(op, msg string) *CacheError

	// Closed returns a CacheError indicating the cache is closed.
	Closed(op string) *CacheError

	// RateLimited returns a CacheError indicating rate limit was exceeded.
	RateLimited(op string) *CacheError

	// CircuitOpen returns a CacheError indicating the circuit breaker is open.
	CircuitOpen(op string) *CacheError

	// EncodeFailed returns a CacheError for value encoding failures.
	EncodeFailed(key, codec string, cause error) *CacheError

	// DecodeFailed returns a CacheError for value decoding failures.
	DecodeFailed(key, codec string, cause error) *CacheError

	// BackendNotFound returns a CacheError when a named backend cannot be found.
	BackendNotFound(name string) *CacheError

	// AlreadyClosed returns a CacheError when attempting to close an already-closed resource.
	AlreadyClosed(name string) *CacheError

	// CloseFailed returns a CacheError when close encounters an error.
	CloseFailed(name string, cause error) *CacheError

	// WarmFailed returns a CacheError when cache warming fails for a key.
	WarmFailed(key string, cause error) *CacheError

	// WarmSourceFailed returns a CacheError when the warm source callback fails.
	WarmSourceFailed(cause error) *CacheError

	// CallbackFailed returns a CacheError when a user callback fails.
	CallbackFailed(key string, cause error) *CacheError

	// --- Classification helpers ---

	// IsNotFound reports whether err is a not-found error.
	IsNotFound(err error) bool

	// IsClosed reports whether err indicates the cache is closed.
	IsClosed(err error) bool

	// IsTimeout reports whether err is a timeout error.
	IsTimeout(err error) bool

	// IsRateLimited reports whether err is a rate-limit error.
	IsRateLimited(err error) bool

	// IsCircuitOpen reports whether err indicates the circuit breaker is open.
	IsCircuitOpen(err error) bool

	// IsRetryable reports whether err represents a transient failure.
	IsRetryable(err error) bool
}

// DefaultFactory provides a standard implementation of ErrorFactory
// that creates CacheError instances with timestamps.
type DefaultFactory struct{}

// NewDefaultFactory returns a new DefaultFactory instance.
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

// New creates a CacheError with the given parameters and the current timestamp.
func (*DefaultFactory) New(code ErrorCode, op, key, msg string, cause error) *CacheError {
	return &CacheError{
		Code:      code,
		Op:        op,
		Key:       key,
		Message:   msg,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// Wrap creates a CacheError wrapping an existing error. It attempts to
// classify the error by checking if the cause is already a CacheError.
// If so, it returns the cause directly. Otherwise, it creates a new
// error with CodeInvalid and the cause attached.
func (f *DefaultFactory) Wrap(op, key string, cause error) *CacheError {
	if cause == nil {
		return f.New(CodeInvalid, op, key, "nil cause", nil)
	}
	var cacheErr *CacheError
	if As(cause, &cacheErr) {
		return cacheErr
	}
	return f.New(CodeInvalid, op, key, cause.Error(), cause)
}

// NotFound returns a CacheError for key-not-found conditions.
func (f *DefaultFactory) NotFound(op, key string) *CacheError {
	return f.New(CodeNotFound, op, key, "key not found", nil)
}

// InvalidKey returns a CacheError for invalid key validation failures.
func (f *DefaultFactory) InvalidKey(op, key string, cause error) *CacheError {
	return f.New(CodeInvalidKey, op, key, "invalid key", cause)
}

// EmptyKey returns a CacheError for an empty key input.
func (f *DefaultFactory) EmptyKey(op string) *CacheError {
	return f.New(CodeEmptyKey, op, "", "key must not be empty", nil)
}

// Connection returns a CacheError for backend connection failures.
func (f *DefaultFactory) Connection(op string, cause error) *CacheError {
	return f.New(CodeConnection, op, "", "connection error", cause)
}

// Timeout returns a CacheError for operation timeouts.
func (f *DefaultFactory) Timeout(op string) *CacheError {
	return f.New(CodeTimeout, op, "", "operation timed out", nil)
}

// SerializeError returns a CacheError for value serialization failures.
func (f *DefaultFactory) SerializeError(op string, cause error) *CacheError {
	return f.New(CodeSerialize, op, "", "serialization failed", cause)
}

// DeserializeError returns a CacheError for value deserialization failures.
func (f *DefaultFactory) DeserializeError(op string, cause error) *CacheError {
	return f.New(CodeDeserialize, op, "", "deserialization failed", cause)
}

// InvalidConfig returns a CacheError for configuration validation failures.
func (f *DefaultFactory) InvalidConfig(op, msg string) *CacheError {
	return f.New(CodeInvalid, op, "", msg, nil)
}

// Closed returns a CacheError indicating the cache has been shut down.
func (f *DefaultFactory) Closed(op string) *CacheError {
	return f.New(CodeCacheClosed, op, "", "cache is closed", nil)
}

// RateLimited returns a CacheError for rate limit exceeded conditions.
func (f *DefaultFactory) RateLimited(op string) *CacheError {
	return f.New(CodeRateLimited, op, "", "rate limit exceeded", nil)
}

// CircuitOpen returns a CacheError for circuit breaker open conditions.
func (f *DefaultFactory) CircuitOpen(op string) *CacheError {
	return f.New(CodeCircuitOpen, op, "", "circuit breaker is open", nil)
}

// EncodeFailed returns a CacheError for value encoding failures.
func (f *DefaultFactory) EncodeFailed(key, codec string, cause error) *CacheError {
	return f.New(CodeSerialize, "encode", key, "encode failed with codec "+codec, cause)
}

// DecodeFailed returns a CacheError for value decoding failures.
func (f *DefaultFactory) DecodeFailed(key, codec string, cause error) *CacheError {
	return f.New(CodeDeserialize, "decode", key, "decode failed with codec "+codec, cause)
}

// BackendNotFound returns a CacheError when a named backend cannot be found.
func (f *DefaultFactory) BackendNotFound(name string) *CacheError {
	return f.New(CodeNotFound, "backend", "", "backend not found: "+name, nil)
}

// AlreadyClosed returns a CacheError when attempting to close an already-closed resource.
func (f *DefaultFactory) AlreadyClosed(name string) *CacheError {
	return f.New(CodeCacheClosed, "close", "", name+" is already closed", nil)
}

// CloseFailed returns a CacheError when close encounters an error.
func (f *DefaultFactory) CloseFailed(name string, cause error) *CacheError {
	return f.New(CodeInvalid, "close", "", "failed to close "+name, cause)
}

// WarmFailed returns a CacheError when cache warming fails for a key.
func (f *DefaultFactory) WarmFailed(key string, cause error) *CacheError {
	return f.New(CodeInvalid, "warm", key, "cache warm failed", cause)
}

// WarmSourceFailed returns a CacheError when the warm source callback fails.
func (f *DefaultFactory) WarmSourceFailed(cause error) *CacheError {
	return f.New(CodeInvalid, "warm_source", "", "warm source callback failed", cause)
}

// CallbackFailed returns a CacheError when a user callback fails.
func (f *DefaultFactory) CallbackFailed(key string, cause error) *CacheError {
	return f.New(CodeInvalid, "callback", key, "user callback failed", cause)
}

// --- Classification helpers ---

// IsNotFound reports whether err is a not-found error.
func (*DefaultFactory) IsNotFound(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code == CodeNotFound
	}
	return stdlibIs(ErrNotFound, err)
}

// IsClosed reports whether err indicates the cache is closed.
func (*DefaultFactory) IsClosed(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code == CodeCacheClosed
	}
	return stdlibIs(ErrClosed, err)
}

// IsTimeout reports whether err is a timeout error.
func (*DefaultFactory) IsTimeout(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code == CodeTimeout
	}
	return stdlibIs(ErrTimeout, err)
}

// IsRateLimited reports whether err is a rate-limit error.
func (*DefaultFactory) IsRateLimited(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code == CodeRateLimited
	}
	return stdlibIs(ErrRateLimited, err)
}

// IsCircuitOpen reports whether err indicates the circuit breaker is open.
func (*DefaultFactory) IsCircuitOpen(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code == CodeCircuitOpen
	}
	return stdlibIs(ErrCircuitOpen, err)
}

// IsRetryable reports whether err represents a transient failure.
func (*DefaultFactory) IsRetryable(err error) bool {
	var ce *CacheError
	if As(err, &ce) {
		return ce.Code.IsRetryable()
	}
	return false
}

// As is a convenience wrapper around errors.As that works with CacheError.
// It is exported to avoid import cycles with the standard errors package
// while providing the same functionality within the cache error system.
func As(err error, target interface{}) bool {
	return stdlibAs(err, target)
}

// Factory is the package-level error factory. It is initialized with a
// DefaultFactory and can be replaced with a custom implementation.
var Factory ErrorFactory = NewDefaultFactory()
