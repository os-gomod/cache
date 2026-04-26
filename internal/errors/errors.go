package errors

import (
	"fmt"
	"time"

	stdliberrors "errors"
)

// CacheError is the primary error type for the cache platform. It carries
// structured information including an error code, the operation that failed,
// the affected key, a human-readable message, the underlying cause, optional
// metadata, and the timestamp of the error.
type CacheError struct {
	Code      ErrorCode      `json:"code"`
	Op        string         `json:"op"`
	Key       string         `json:"key,omitempty"`
	Message   string         `json:"message"`
	Cause     error          `json:"cause,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Error returns a human-readable representation of the cache error including
// the code, operation, key, and message. If a cause is present, it is
// appended after ": ".
func (e *CacheError) Error() string {
	base := fmt.Sprintf("cache[%s] %s: %s", e.Code, e.Op, e.Message)
	if e.Key != "" {
		base += fmt.Sprintf(" (key=%s)", e.Key)
	}
	if e.Cause != nil {
		base += ": " + e.Cause.Error()
	}
	return base
}

// Unwrap returns the underlying cause error, enabling errors.Is and errors.As
// chains from the standard library.
func (e *CacheError) Unwrap() error {
	return e.Cause
}

// Is reports whether target matches this CacheError. It checks for both
// exact CacheError matches and sentinel error values.
func (e *CacheError) Is(target error) bool {
	t, ok := target.(*CacheError)
	if !ok {
		// Check against sentinel errors
		return e.matchesSentinel(target)
	}
	return t.Code == e.Code && t.Op == e.Op
}

func (e *CacheError) matchesSentinel(target error) bool {
	for _, s := range sentinels {
		if stdliberrors.Is(target, s) {
			var sentinel *CacheError
			if stdliberrors.As(s, &sentinel) && sentinel.Code == e.Code {
				return true
			}
		}
	}
	return false
}

// WithMetadata returns a copy of the CacheError with the given key-value pair
// added to the metadata map. This method is non-mutating; it returns a new
// CacheError with the additional metadata.
func (e *CacheError) WithMetadata(k string, v any) *CacheError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	clone := *e
	clone.Metadata = make(map[string]any, len(e.Metadata)+1)
	for mk, mv := range e.Metadata {
		clone.Metadata[mk] = mv
	}
	clone.Metadata[k] = v
	return &clone
}

// IsNotFound returns true if this error represents a cache miss (key not found).
func (e *CacheError) IsNotFound() bool {
	return e.Code == CodeNotFound
}

// IsTimeout returns true if this error represents a timeout condition.
func (e *CacheError) IsTimeout() bool {
	return e.Code == CodeTimeout
}

// IsRetryable returns true if this error represents a transient failure that
// may succeed on retry.
func (e *CacheError) IsRetryable() bool {
	return e.Code.IsRetryable()
}

// IsClosed returns true if this error indicates the cache has been shut down.
func (e *CacheError) IsClosed() bool {
	return e.Code == CodeCacheClosed
}

// IsExpired returns true if this error indicates the entry has expired.
func (e *CacheError) IsExpired() bool {
	return e.Code == CodeExpired
}

// Sentinels are pre-allocated error values for common error conditions.
// Use errors.Is(err, errors.ErrNotFound) to check for these.
var (
	// ErrNotFound is returned when a requested key does not exist in the cache.
	ErrNotFound = &CacheError{
		Code:      CodeNotFound,
		Op:        "get",
		Message:   "key not found",
		Timestamp: time.Time{},
	}

	// ErrExpired is returned when a requested entry has expired.
	ErrExpired = &CacheError{
		Code:      CodeExpired,
		Op:        "get",
		Message:   "entry expired",
		Timestamp: time.Time{},
	}

	// ErrClosed is returned when an operation is attempted on a closed cache.
	ErrClosed = &CacheError{
		Code:      CodeCacheClosed,
		Op:        "operation",
		Message:   "cache is closed",
		Timestamp: time.Time{},
	}

	// ErrInvalidKey is returned when a key fails validation.
	ErrInvalidKey = &CacheError{
		Code:      CodeInvalidKey,
		Op:        "validate",
		Message:   "invalid key",
		Timestamp: time.Time{},
	}

	// ErrTimeout is returned when an operation exceeds its deadline.
	ErrTimeout = &CacheError{
		Code:      CodeTimeout,
		Op:        "operation",
		Message:   "operation timed out",
		Timestamp: time.Time{},
	}

	// ErrCancelled is returned when an operation is cancelled via context.
	ErrCancelled = &CacheError{
		Code:      CodeCancelled,
		Op:        "operation",
		Message:   "operation cancelled",
		Timestamp: time.Time{},
	}

	// ErrConnection is returned when a connection to the cache backend fails.
	ErrConnection = &CacheError{
		Code:      CodeConnection,
		Op:        "connect",
		Message:   "connection failed",
		Timestamp: time.Time{},
	}

	// ErrOverflow is returned when the cache has exceeded its capacity.
	ErrOverflow = &CacheError{
		Code:      CodeOverflow,
		Op:        "set",
		Message:   "cache capacity exceeded",
		Timestamp: time.Time{},
	}

	// ErrRateLimited is returned when the rate limiter rejects an operation.
	ErrRateLimited = &CacheError{
		Code:      CodeRateLimited,
		Op:        "operation",
		Message:   "rate limit exceeded",
		Timestamp: time.Time{},
	}

	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = &CacheError{
		Code:      CodeCircuitOpen,
		Op:        "operation",
		Message:   "circuit breaker is open",
		Timestamp: time.Time{},
	}

	// ErrEmptyKey is returned when an empty key is provided.
	ErrEmptyKey = &CacheError{
		Code:      CodeEmptyKey,
		Op:        "validate",
		Message:   "key must not be empty",
		Timestamp: time.Time{},
	}

	// ErrNotSupported is returned when an operation is not supported by the backend.
	ErrNotSupported = &CacheError{
		Code:      CodeNotSupported,
		Op:        "operation",
		Message:   "operation not supported",
		Timestamp: time.Time{},
	}
)

// sentinels collects all sentinel errors for iteration in Is() checks.
var sentinels = []error{
	ErrNotFound, ErrExpired, ErrClosed, ErrInvalidKey, ErrTimeout,
	ErrCancelled, ErrConnection, ErrOverflow, ErrRateLimited, ErrCircuitOpen,
	ErrEmptyKey, ErrNotSupported,
}
