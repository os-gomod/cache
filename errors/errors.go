// Package errors provides typed cache errors with error codes, structured
// metadata, and helper functions for error classification.
package errors

import (
	"errors"
	"strings"
)

// ErrorCode is a numeric identifier for a category of cache error.
type ErrorCode int

const (
	CodeOK ErrorCode = iota
	CodeNotFound
	CodeCacheClosed
	CodeInvalidKey
	CodeSerialize
	CodeDeserialize
	CodeTimeout
	CodeCancelled
	CodeOverflow
	CodeInvalid
	CodeConnection
	CodeEmptyKey
	CodeExpired
	CodeNotSupported
	CodeAlreadyExists
	CodeLockFailed
	CodeRateLimited
	CodeCircuitOpen
	CodeNoAvailableNode
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

// String returns the human-readable name for the error code.
func (c ErrorCode) String() string {
	if c >= 0 && int(c) < len(errorCodeNames) {
		if name := errorCodeNames[c]; name != "" {
			return name
		}
	}
	return "unknown"
}

var errJSONUnsupportedType = errors.New("json: unsupported type")
var (
	ErrNotFound       = newSentinel(CodeNotFound, "cache: key not found")
	ErrExpired        = newSentinel(CodeExpired, "cache: key expired")
	ErrNilValue       = newSentinel(CodeInvalid, "cache: nil value provided")
	ErrInvalidConfig  = newSentinel(CodeInvalid, "cache: invalid configuration")
	ErrConnectionLost = newSentinel(CodeConnection, "cache: connection lost")
	ErrMarshal        = newSentinel(CodeSerialize, "cache: marshal failed")
	ErrUnmarshal      = newSentinelWithCause(
		CodeDeserialize,
		"cache: unmarshal failed",
		errJSONUnsupportedType,
	)
	ErrCacheFull       = newSentinel(CodeOverflow, "cache: cache full")
	ErrClosed          = newSentinel(CodeCacheClosed, "cache: cache closed")
	ErrInvalidKey      = newSentinel(CodeInvalidKey, "cache: invalid key")
	ErrTimeout         = newSentinel(CodeTimeout, "cache: operation timeout")
	ErrCancelled       = newSentinel(CodeCancelled, "cache: operation cancelled")
	ErrEmptyKey        = newSentinel(CodeEmptyKey, "cache: empty key")
	ErrNotSupported    = newSentinel(CodeNotSupported, "cache: operation not supported")
	ErrAlreadyExists   = newSentinel(CodeAlreadyExists, "cache: key already exists")
	ErrLockFailed      = newSentinel(CodeLockFailed, "cache: lock acquisition failed")
	ErrRateLimited     = newSentinel(CodeRateLimited, "cache: rate limit exceeded")
	ErrCircuitOpen     = newSentinel(CodeCircuitOpen, "cache: circuit breaker is open")
	ErrNoAvailableNode = newSentinel(CodeNoAvailableNode, "cache: no available node")
	ErrUnsupportedType = newSentinel(CodeUnsupportedType, "cache: unsupported type")
)

// CacheError is a structured error with a code, metadata, and optional cause chain.
type CacheError struct {
	Code      ErrorCode
	Metadata  map[string]any
	Message   string
	Key       string
	Operation string
	Cause     error
	TraceID   string
}

func newSentinel(ec ErrorCode, msg string) *CacheError {
	return &CacheError{Code: ec, Message: msg}
}

func newSentinelWithCause(ec ErrorCode, msg string, cause error) *CacheError {
	return &CacheError{Code: ec, Message: msg, Cause: cause}
}

// Error returns a formatted error message including operation, key, and cause.
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

// Unwrap returns the underlying cause for errors.Is/errors.As chaining.
func (e *CacheError) Unwrap() error { return e.Cause }

// Is reports whether the target error has the same error code.
func (e *CacheError) Is(target error) bool {
	t, ok := target.(*CacheError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithMetadata adds a key-value pair to the error's metadata and returns the error for chaining.
func (e *CacheError) WithMetadata(key string, value any) *CacheError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

func newTyped(ec ErrorCode, op, key, msg string, cause error) *CacheError {
	return &CacheError{Code: ec, Operation: op, Key: key, Message: msg, Cause: cause}
}

// New creates a new CacheError with an inferred error code from the cause.
func New(op, key string, cause error) *CacheError {
	return &CacheError{Code: CodeOf(cause), Operation: op, Key: key, Cause: cause}
}

// Wrap creates a new CacheError that wraps a cause.
func Wrap(op string, cause error) *CacheError {
	if cause == nil {
		return nil
	}
	return New(op, "", cause)
}

// WrapKey creates a new CacheError with an operation and key that wraps a cause.
func WrapKey(op, key string, cause error) *CacheError {
	if cause == nil {
		return nil
	}
	return New(op, key, cause)
}

// InvalidConfig creates an error indicating invalid configuration.
func InvalidConfig(op, msg string) *CacheError {
	return newTyped(CodeInvalid, op, "", msg, nil)
}

// NotFound creates an error indicating the key was not found.
func NotFound(op, key string) *CacheError {
	return newTyped(CodeNotFound, op, key, "key not found", nil)
}

// Expired creates an error indicating the key has expired.
func Expired(op, key string) *CacheError {
	return newTyped(CodeExpired, op, key, "key expired", nil)
}

// Closed creates an error indicating the cache is closed.
func Closed(op string) *CacheError {
	return newTyped(CodeCacheClosed, op, "", "cache is closed", nil)
}

// InvalidKey creates an error indicating the key is invalid.
func InvalidKey(op, key string, cause error) *CacheError {
	return newTyped(CodeInvalidKey, op, key, "invalid key", cause)
}

// EmptyKey creates an error indicating the key must not be empty.
func EmptyKey(op string) *CacheError {
	return newTyped(CodeEmptyKey, op, "", "key must not be empty", nil)
}

// SerializeError creates an error indicating a serialization failure.
func SerializeError(op string, cause error) *CacheError {
	return newTyped(CodeSerialize, op, "", "serialization failed", cause)
}

// DeserializeError creates an error indicating a deserialization failure.
func DeserializeError(op string, cause error) *CacheError {
	return newTyped(CodeDeserialize, op, "", "deserialization failed", cause)
}

// TimeoutError creates an error indicating an operation timed out.
func TimeoutError(op string) *CacheError {
	return newTyped(CodeTimeout, op, "", "operation timed out", nil)
}

// CancelledError creates an error indicating the operation was cancelled.
func CancelledError(op string) *CacheError {
	return newTyped(CodeCancelled, op, "", "operation cancelled", nil)
}

// RateLimitedError creates an error indicating the rate limit was exceeded.
func RateLimitedError(op string) *CacheError {
	return newTyped(CodeRateLimited, op, "", "rate limit exceeded", nil)
}

// CircuitOpenError creates an error indicating the circuit breaker is open.
func CircuitOpenError(op string) *CacheError {
	return newTyped(CodeCircuitOpen, op, "", "circuit breaker is open", nil)
}

// LockFailedError creates an error indicating a lock could not be acquired.
func LockFailedError(op, key string) *CacheError {
	return newTyped(CodeLockFailed, op, key, "lock acquisition failed", nil)
}

// ConnectionError creates an error indicating a connection failure.
func ConnectionError(op string, cause error) *CacheError {
	return newTyped(CodeConnection, op, "", "connection error", cause)
}

// CodeOf extracts the ErrorCode from an error if it is a CacheError.
func CodeOf(err error) ErrorCode {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return CodeInvalid
}

// IsNotFound reports whether err is a not-found cache error.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsExpired reports whether err is a cache expiration error.
func IsExpired(err error) bool { return errors.Is(err, ErrExpired) }

// IsCacheClosed reports whether err indicates the cache is closed.
func IsCacheClosed(err error) bool { return errors.Is(err, ErrClosed) }

// IsTimeout reports whether err is a cache timeout error.
func IsTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// IsCancelled reports whether err is a cache cancellation error.
func IsCancelled(err error) bool { return errors.Is(err, ErrCancelled) }

// IsRateLimited reports whether err indicates rate limiting.
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsCircuitOpen reports whether err indicates the circuit breaker is open.
func IsCircuitOpen(err error) bool { return errors.Is(err, ErrCircuitOpen) }

// IsConnectionError reports whether err is a connection failure.
func IsConnectionError(err error) bool { return errors.Is(err, ErrConnectionLost) }

// Retryable reports whether the error is transient and worth retrying.
func Retryable(err error) bool {
	code := CodeOf(err)
	return code == CodeTimeout || code == CodeConnection || code == CodeRateLimited
}

// AsType extracts the CacheError from err if present.
func AsType(err error) (*CacheError, bool) {
	var ce *CacheError
	ok := errors.As(err, &ce)
	return ce, ok
}

// Is reports whether err has the specified error code.
func Is(err error, code ErrorCode) bool {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// GetMetadata retrieves a metadata value from a CacheError by key.
func GetMetadata(err error, key string) (any, bool) {
	ce, ok := AsType(err)
	if !ok || ce.Metadata == nil {
		return nil, false
	}
	val, found := ce.Metadata[key]
	return val, found
}
