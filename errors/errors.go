// Package errors provides structured error types for the cache library.
// All cache errors implement the CacheError type which carries an ErrorCode,
// optional metadata, key, operation, cause, and trace ID. Error classification
// is done via errors.Is and the CodeOf function.
package errors

import (
	"errors"
	"strings"
)

// ErrorCode represents a categorized error code for cache operations. Use
// CodeOf to extract the code from any error, and errors.Is with sentinel
// errors to check specific conditions.
type ErrorCode int

const (
	// CodeOK indicates no error occurred.
	CodeOK ErrorCode = iota
	// CodeNotFound indicates the requested key was not found in the cache.
	CodeNotFound
	// CodeCacheClosed indicates the cache backend has been closed.
	CodeCacheClosed
	// CodeInvalidKey indicates the provided key is malformed.
	CodeInvalidKey
	// CodeSerialize indicates a serialization (encoding) failure.
	CodeSerialize
	// CodeDeserialize indicates a deserialization (decoding) failure.
	CodeDeserialize
	// CodeTimeout indicates the operation exceeded its deadline.
	CodeTimeout
	// CodeCancelled indicates the operation was cancelled via context.
	CodeCancelled
	// CodeOverflow indicates the cache has reached its capacity limit.
	CodeOverflow
	// CodeInvalid indicates an invalid argument or configuration.
	CodeInvalid
	// CodeConnection indicates a network connection failure.
	CodeConnection
	// CodeEmptyKey indicates an empty key was provided.
	CodeEmptyKey
	// CodeExpired indicates the key has expired.
	CodeExpired
	// CodeNotSupported indicates the operation is not supported by the backend.
	CodeNotSupported
	// CodeAlreadyExists indicates the key already exists (for SetNX).
	CodeAlreadyExists
	// CodeLockFailed indicates a lock acquisition failure.
	CodeLockFailed
	// CodeRateLimited indicates the rate limit has been exceeded.
	CodeRateLimited
	// CodeCircuitOpen indicates the circuit breaker is open.
	CodeCircuitOpen
	// CodeNoAvailableNode indicates no backend node is available.
	CodeNoAvailableNode
	// CodeUnsupportedType indicates the value type is not supported.
	CodeUnsupportedType
)

var errorCodeNames = [...]string{
	CodeOK:              "ok",
	CodeNotFound:        "not_found",
	CodeCacheClosed:     "cache_closed",
	CodeInvalidKey:      "invalid_key",
	CodeSerialize:       "serialize_error",
	CodeDeserialize:     "deserialize_error",
	CodeTimeout:         "timeout",
	CodeCancelled:       "cancelled",
	CodeOverflow:        "overflow",
	CodeInvalid:         "invalid",
	CodeConnection:      "connection_error",
	CodeEmptyKey:        "empty_key",
	CodeExpired:         "expired",
	CodeNotSupported:    "not_supported",
	CodeAlreadyExists:   "already_exists",
	CodeLockFailed:      "lock_failed",
	CodeRateLimited:     "rate_limited",
	CodeCircuitOpen:     "circuit_open",
	CodeNoAvailableNode: "no_available_node",
	CodeUnsupportedType: "unsupported",
}

// String returns a human-readable string representation of the ErrorCode.
func (c ErrorCode) String() string {
	if c >= 0 && int(c) < len(errorCodeNames) {
		if name := errorCodeNames[c]; name != "" {
			return name
		}
	}
	return "unknown"
}

// errJSONUnsupportedType is an internal sentinel for the ErrUnmarshal cause.
var errJSONUnsupportedType = errors.New("json: unsupported type")

var (
	// ErrNotFound is the sentinel error for cache misses.
	ErrNotFound = newSentinel(CodeNotFound, "cache: key not found")
	// ErrExpired is the sentinel error for expired keys.
	ErrExpired = newSentinel(CodeExpired, "cache: key expired")
	// ErrNilValue is the sentinel error for nil value submissions.
	ErrNilValue = newSentinel(CodeInvalid, "cache: nil value provided")
	// ErrInvalidConfig is the sentinel error for invalid configuration.
	ErrInvalidConfig = newSentinel(CodeInvalid, "cache: invalid configuration")
	// ErrConnectionLost is the sentinel error for lost network connections.
	ErrConnectionLost = newSentinel(CodeConnection, "cache: connection lost")
	// ErrMarshal is the sentinel error for marshaling failures.
	ErrMarshal = newSentinel(CodeSerialize, "cache: marshal failed")
	// ErrUnmarshal is the sentinel error for unmarshalling failures.
	ErrUnmarshal = newSentinelWithCause(
		CodeDeserialize,
		"cache: unmarshal failed",
		errJSONUnsupportedType,
	)
	// ErrCacheFull is the sentinel error when the cache is at capacity.
	ErrCacheFull = newSentinel(CodeOverflow, "cache: cache full")
	// ErrClosed is the sentinel error for operations on a closed cache.
	ErrClosed = newSentinel(CodeCacheClosed, "cache: cache closed")
	// ErrInvalidKey is the sentinel error for malformed keys.
	ErrInvalidKey = newSentinel(CodeInvalidKey, "cache: invalid key")
	// ErrTimeout is the sentinel error for timed-out operations.
	ErrTimeout = newSentinel(CodeTimeout, "cache: operation timeout")
	// ErrCancelled is the sentinel error for cancelled operations.
	ErrCancelled = newSentinel(CodeCancelled, "cache: operation cancelled")
	// ErrEmptyKey is the sentinel error for empty keys.
	ErrEmptyKey = newSentinel(CodeEmptyKey, "cache: empty key")
	// ErrNotSupported is the sentinel error for unsupported operations.
	ErrNotSupported = newSentinel(CodeNotSupported, "cache: operation not supported")
	// ErrAlreadyExists is the sentinel error for SetNX on existing keys.
	ErrAlreadyExists = newSentinel(CodeAlreadyExists, "cache: key already exists")
	// ErrLockFailed is the sentinel error for lock acquisition failures.
	ErrLockFailed = newSentinel(CodeLockFailed, "cache: lock acquisition failed")
	// ErrRateLimited is the sentinel error for rate-limited operations.
	ErrRateLimited = newSentinel(CodeRateLimited, "cache: rate limit exceeded")
	// ErrCircuitOpen is the sentinel error when the circuit breaker is open.
	ErrCircuitOpen = newSentinel(CodeCircuitOpen, "cache: circuit breaker is open")
	// ErrNoAvailableNode is the sentinel error when no backend node is available.
	ErrNoAvailableNode = newSentinel(CodeNoAvailableNode, "cache: no available node")
	// ErrUnsupportedType is the sentinel error for unsupported value types.
	ErrUnsupportedType = newSentinel(CodeUnsupportedType, "cache: unsupported type")
)

// CacheError is the structured error type for all cache operations. It carries
// a machine-readable ErrorCode, human-readable Message, optional Metadata,
// the Key and Operation involved, a Cause chain, and a TraceID for
// distributed tracing.
type CacheError struct {
	Code      ErrorCode
	Metadata  map[string]any
	Message   string
	Key       string
	Operation string
	Cause     error
	TraceID   string
}

// newSentinel creates a CacheError suitable for use as a sentinel value.
func newSentinel(ec ErrorCode, msg string) *CacheError {
	return &CacheError{Code: ec, Message: msg}
}

// newSentinelWithCause creates a CacheError sentinel with a pre-attached cause.
func newSentinelWithCause(ec ErrorCode, msg string, cause error) *CacheError {
	return &CacheError{Code: ec, Message: msg, Cause: cause}
}

// Error returns a human-readable error string. The format is:
// "operation key=<key>: message: cause" with optional segments omitted when
// empty.
func (e *CacheError) Error() string {
	var b strings.Builder
	if e.Operation != "" {
		b.WriteString(e.Operation)
	}
	if e.Key != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("key=")
		b.WriteString(e.Key)
	}
	if b.Len() > 0 {
		if e.Message != "" || e.Cause != nil {
			b.WriteString(": ")
		}
	}
	if e.Message != "" {
		b.WriteString(e.Message)
	}
	if e.Cause != nil {
		causeMsg := renderCause(e.Cause)
		if causeMsg != "" {
			if e.Message != "" {
				b.WriteString(": ")
			}
			b.WriteString(causeMsg)
		}
	}
	return b.String()
}

// renderCause recursively renders the cause chain, skipping intermediate
// CacheError wrappers that have no Operation or Key.
func renderCause(err error) string {
	if err == nil {
		return ""
	}
	ce, ok := err.(*CacheError)
	if !ok {
		return err.Error()
	}
	if ce.Operation == "" && ce.Key == "" && ce.Cause != nil {
		return renderCause(ce.Cause)
	}
	return ce.Error()
}

// Unwrap returns the underlying cause for errors.Unwrap compatibility.
func (e *CacheError) Unwrap() error { return e.Cause }

// Is supports errors.Is by comparing ErrorCode values. Two CacheErrors match
// if they share the same code, regardless of other fields.
func (e *CacheError) Is(target error) bool {
	t, ok := target.(*CacheError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithMetadata adds a key-value pair to the error's metadata map and returns
// the same CacheError for chaining.
func (e *CacheError) WithMetadata(key string, value any) *CacheError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// newTyped creates a CacheError with full field control.
func newTyped(ec ErrorCode, op, key, msg string, cause error) *CacheError {
	return &CacheError{Code: ec, Operation: op, Key: key, Message: msg, Cause: cause}
}

// New creates a CacheError with the given operation, key, and cause. The
// ErrorCode is derived from the cause via CodeOf.
func New(op, key string, cause error) *CacheError {
	return &CacheError{Code: CodeOf(cause), Operation: op, Key: key, Cause: cause}
}

// Wrap wraps a cause error with an operation name. Returns nil if cause is nil.
func Wrap(op string, cause error) *CacheError {
	if cause == nil {
		return nil
	}
	return New(op, "", cause)
}

// WrapKey wraps a cause error with an operation name and key. Returns nil if
// cause is nil.
func WrapKey(op, key string, cause error) *CacheError {
	if cause == nil {
		return nil
	}
	return New(op, key, cause)
}

// InvalidConfig creates a CodeInvalid error for configuration problems.
func InvalidConfig(op, msg string) *CacheError {
	return newTyped(CodeInvalid, op, "", msg, nil)
}

// NotFound creates a CodeNotFound error for the given operation and key.
func NotFound(op, key string) *CacheError {
	return newTyped(CodeNotFound, op, key, "key not found", nil)
}

// Expired creates a CodeExpired error for the given operation and key.
func Expired(op, key string) *CacheError {
	return newTyped(CodeExpired, op, key, "key expired", nil)
}

// Closed creates a CodeCacheClosed error for the given operation.
func Closed(op string) *CacheError {
	return newTyped(CodeCacheClosed, op, "", "cache is closed", nil)
}

// InvalidKey creates a CodeInvalidKey error for the given operation, key, and
// optional cause.
func InvalidKey(op, key string, cause error) *CacheError {
	return newTyped(CodeInvalidKey, op, key, "invalid key", cause)
}

// EmptyKey creates a CodeEmptyKey error for the given operation.
func EmptyKey(op string) *CacheError {
	return newTyped(CodeEmptyKey, op, "", "key must not be empty", nil)
}

// SerializeError creates a CodeSerialize error for encoding failures.
func SerializeError(op string, cause error) *CacheError {
	return newTyped(CodeSerialize, op, "", "serialization failed", cause)
}

// DeserializeError creates a CodeDeserialize error for decoding failures.
func DeserializeError(op string, cause error) *CacheError {
	return newTyped(CodeDeserialize, op, "", "deserialization failed", cause)
}

// TimeoutError creates a CodeTimeout error for the given operation.
func TimeoutError(op string) *CacheError {
	return newTyped(CodeTimeout, op, "", "operation timed out", nil)
}

// CancelledError creates a CodeCancelled error for the given operation.
func CancelledError(op string) *CacheError {
	return newTyped(CodeCancelled, op, "", "operation cancelled", nil)
}

// RateLimitedError creates a CodeRateLimited error for the given operation.
func RateLimitedError(op string) *CacheError {
	return newTyped(CodeRateLimited, op, "", "rate limit exceeded", nil)
}

// CircuitOpenError creates a CodeCircuitOpen error for the given operation.
func CircuitOpenError(op string) *CacheError {
	return newTyped(CodeCircuitOpen, op, "", "circuit breaker is open", nil)
}

// LockFailedError creates a CodeLockFailed error for the given operation and
// key.
func LockFailedError(op, key string) *CacheError {
	return newTyped(CodeLockFailed, op, key, "lock acquisition failed", nil)
}

// ConnectionError creates a CodeConnection error for network failures.
func ConnectionError(op string, cause error) *CacheError {
	return newTyped(CodeConnection, op, "", "connection error", cause)
}

// CodeOf extracts the ErrorCode from any error. If the error is not a
// CacheError, CodeInvalid is returned.
func CodeOf(err error) ErrorCode {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return CodeInvalid
}

// IsNotFound returns true if the error is a cache miss (CodeNotFound).
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsExpired returns true if the error indicates an expired key.
func IsExpired(err error) bool { return errors.Is(err, ErrExpired) }

// IsCacheClosed returns true if the error indicates the cache is closed.
func IsCacheClosed(err error) bool { return errors.Is(err, ErrClosed) }

// IsTimeout returns true if the error indicates a timeout.
func IsTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// IsCancelled returns true if the error indicates cancellation.
func IsCancelled(err error) bool { return errors.Is(err, ErrCancelled) }

// IsRateLimited returns true if the error indicates rate limiting.
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsCircuitOpen returns true if the error indicates an open circuit breaker.
func IsCircuitOpen(err error) bool { return errors.Is(err, ErrCircuitOpen) }

// IsConnectionError returns true if the error indicates a connection failure.
func IsConnectionError(err error) bool { return errors.Is(err, ErrConnectionLost) }

// Retryable returns true if the error is transient and the operation can be
// safely retried. Retryable error codes are CodeTimeout, CodeConnection, and
// CodeRateLimited.
func Retryable(err error) bool {
	code := CodeOf(err)
	return code == CodeTimeout || code == CodeConnection || code == CodeRateLimited
}

// AsType attempts to cast err to *CacheError. Returns the CacheError and true
// if successful, or nil and false otherwise.
func AsType(err error) (*CacheError, bool) {
	var ce *CacheError
	ok := errors.As(err, &ce)
	return ce, ok
}

// Is returns true if the error's code matches the given ErrorCode.
func Is(err error, code ErrorCode) bool {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// GetMetadata retrieves a metadata value from the error by key. Returns the
// value and true if found, or nil and false otherwise.
func GetMetadata(err error, key string) (any, bool) {
	ce, ok := AsType(err)
	if !ok || ce.Metadata == nil {
		return nil, false
	}
	val, found := ce.Metadata[key]
	return val, found
}
