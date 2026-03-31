// Package errors provides centralized, structured error handling with typed sentinel errors.
package errors

import (
	"errors"
	"strings"
)

// ErrorCode defines the type for error codes.
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

//nolint:gocyclo // ErrorCode.String covers all sentinel codes.
func (c ErrorCode) String() string {
	switch c {
	case CodeOK:
		return "ok"
	case CodeNotFound:
		return "not_found"
	case CodeCacheClosed:
		return "cache_closed"
	case CodeInvalidKey:
		return "invalid_key"
	case CodeSerialize:
		return "serialize_error"
	case CodeDeserialize:
		return "deserialize_error"
	case CodeTimeout:
		return "timeout"
	case CodeCancelled:
		return "cancelled"
	case CodeOverflow:
		return "overflow"
	case CodeInvalid:
		return "invalid"
	case CodeConnection:
		return "connection_error"
	case CodeEmptyKey:
		return "empty_key"
	case CodeExpired:
		return "expired"
	case CodeNotSupported:
		return "not_supported"
	case CodeAlreadyExists:
		return "already_exists"
	case CodeLockFailed:
		return "lock_failed"
	case CodeRateLimited:
		return "rate_limited"
	case CodeCircuitOpen:
		return "circuit_open"
	case CodeNoAvailableNode:
		return "no_available_node"
	case CodeUnsupportedType:
		return "unsupported"
	default:
		return "unknown"
	}
}

// Sentinel errors for comparison via errors.Is.
var (
	errJSONUnsupportedType = errors.New("json: unsupported type")

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

// CacheError is a structured, context-rich cache error.
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

// Error implements the error interface.
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

// Unwrap returns the underlying cause for errors.Is / errors.As chain walking.
func (e *CacheError) Unwrap() error { return e.Cause }

// Is matches sentinels by error code.
func (e *CacheError) Is(target error) bool {
	t, ok := target.(*CacheError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithMetadata attaches an arbitrary key/value pair to the error.
func (e *CacheError) WithMetadata(key string, value any) *CacheError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// --- constructors ---

func newTyped(ec ErrorCode, op, key, msg string, cause error) error {
	return &CacheError{Code: ec, Operation: op, Key: key, Message: msg, Cause: cause}
}

// New creates a new CacheError deriving the code from cause.
func New(op, key string, cause error) error {
	return &CacheError{Code: CodeOf(cause), Operation: op, Key: key, Cause: cause}
}

// Wrap wraps an error with operation context.
func Wrap(op string, cause error) error {
	if cause == nil {
		return nil
	}
	return New(op, "", cause)
}

// WrapKey wraps an error with operation and key context.
func WrapKey(op, key string, cause error) error {
	if cause == nil {
		return nil
	}
	return New(op, key, cause)
}

// InvalidConfig creates an invalid-configuration error.
func InvalidConfig(op, msg string) error {
	return newTyped(CodeInvalid, op, "", msg, nil)
}

// NotFound creates a not-found error.
func NotFound(op, key string) error {
	return newTyped(CodeNotFound, op, key, "key not found", nil)
}

// Expired creates an expired error.
func Expired(op, key string) error {
	return newTyped(CodeExpired, op, key, "key expired", nil)
}

// Closed creates a cache-closed error.
func Closed(op string) error {
	return newTyped(CodeCacheClosed, op, "", "cache is closed", nil)
}

// InvalidKey creates an invalid-key error.
func InvalidKey(op, key string, cause error) error {
	return newTyped(CodeInvalidKey, op, key, "invalid key", cause)
}

// EmptyKey creates an empty-key error.
func EmptyKey(op string) error {
	return newTyped(CodeEmptyKey, op, "", "key must not be empty", nil)
}

// SerializeError creates a serialization error.
func SerializeError(op string, cause error) error {
	return newTyped(CodeSerialize, op, "", "serialization failed", cause)
}

// DeserializeError creates a deserialization error.
func DeserializeError(op string, cause error) error {
	return newTyped(CodeDeserialize, op, "", "deserialization failed", cause)
}

// TimeoutError creates a timeout error.
func TimeoutError(op string) error {
	return newTyped(CodeTimeout, op, "", "operation timed out", nil)
}

// CancelledError creates a cancelled error.
func CancelledError(op string) error {
	return newTyped(CodeCancelled, op, "", "operation cancelled", nil)
}

// RateLimitedError creates a rate-limited error.
func RateLimitedError(op string) error {
	return newTyped(CodeRateLimited, op, "", "rate limit exceeded", nil)
}

// CircuitOpenError creates a circuit-open error.
func CircuitOpenError(op string) error {
	return newTyped(CodeCircuitOpen, op, "", "circuit breaker is open", nil)
}

// LockFailedError creates a lock-failed error.
func LockFailedError(op, key string) error {
	return newTyped(CodeLockFailed, op, key, "lock acquisition failed", nil)
}

// ConnectionError creates a connection error.
func ConnectionError(op string, cause error) error {
	return newTyped(CodeConnection, op, "", "connection error", cause)
}

// --- introspection helpers ---

// CodeOf extracts the ErrorCode from err. Returns CodeInvalid for unknown types.
func CodeOf(err error) ErrorCode {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return CodeInvalid
}

// IsNotFound reports whether err is a not-found error.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsExpired reports whether err is an expired error.
func IsExpired(err error) bool { return errors.Is(err, ErrExpired) }

// IsCacheClosed reports whether err is a cache-closed error.
func IsCacheClosed(err error) bool { return errors.Is(err, ErrClosed) }

// IsTimeout reports whether err is a timeout error.
func IsTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// IsCancelled reports whether err is a cancelled error.
func IsCancelled(err error) bool { return errors.Is(err, ErrCancelled) }

// IsRateLimited reports whether err signals rate limiting.
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsCircuitOpen reports whether err signals an open circuit breaker.
func IsCircuitOpen(err error) bool { return errors.Is(err, ErrCircuitOpen) }

// Retryable reports whether the error is safe to retry.
func Retryable(err error) bool {
	code := CodeOf(err)
	return code == CodeTimeout || code == CodeConnection || code == CodeRateLimited
}

// AsType attempts to extract the underlying *CacheError.
func AsType(err error) (*CacheError, bool) {
	var ce *CacheError
	ok := errors.As(err, &ce)
	return ce, ok
}

// Is checks if err matches the sentinel error with the given code.
func Is(err error, code ErrorCode) bool {
	var ce *CacheError
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// GetMetadata retrieves metadata from a CacheError if present.
func GetMetadata(err error, key string) (any, bool) {
	ce, ok := AsType(err)
	if !ok || ce.Metadata == nil {
		return nil, false
	}
	val, found := ce.Metadata[key]
	return val, found
}
