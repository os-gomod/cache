// Package errors provides structured error types, codes, and factory methods
// for the cache platform. All errors produced by cache operations use these
// types to enable consistent error handling, classification, and observability.
package errors

// ErrorCode is an integer enum representing all possible cache error conditions.
// Codes enable programmatic error classification, retry decisions, and
// structured logging.
type ErrorCode int

const (
	// CodeOK indicates no error.
	CodeOK ErrorCode = iota

	// CodeNotFound indicates the requested key was not found in the cache.
	CodeNotFound

	// CodeCacheClosed indicates the cache instance has been shut down.
	CodeCacheClosed

	// CodeInvalidKey indicates the provided key fails validation.
	CodeInvalidKey

	// CodeSerialize indicates a failure serializing a value for storage.
	CodeSerialize

	// CodeDeserialize indicates a failure deserializing a stored value.
	CodeDeserialize

	// CodeTimeout indicates the operation exceeded its deadline.
	CodeTimeout

	// CodeCancelled indicates the operation was cancelled via context.
	CodeCancelled

	// CodeOverflow indicates the cache has exceeded its capacity.
	CodeOverflow

	// CodeInvalid indicates a general validation or configuration error.
	CodeInvalid

	// CodeConnection indicates a failure connecting to the cache backend.
	CodeConnection

	// CodeEmptyKey indicates an empty key was provided.
	CodeEmptyKey

	// CodeExpired indicates the requested entry has expired.
	CodeExpired

	// CodeNotSupported indicates the requested operation is not supported.
	CodeNotSupported

	// CodeAlreadyExists indicates the key already exists (e.g. for SetNX).
	CodeAlreadyExists

	// CodeLockFailed indicates an atomic lock operation failed.
	CodeLockFailed

	// CodeRateLimited indicates the operation was rejected by the rate limiter.
	CodeRateLimited

	// CodeCircuitOpen indicates the circuit breaker is open.
	CodeCircuitOpen

	// CodeNoAvailableNode indicates no backend node is available.
	CodeNoAvailableNode

	// CodeUnsupportedType indicates the value type is not supported for caching.
	CodeUnsupportedType
)

// errorCodeNames maps ErrorCode values to human-readable names.
var errorCodeNames = map[ErrorCode]string{
	CodeOK:              "OK",
	CodeNotFound:        "NOT_FOUND",
	CodeCacheClosed:     "CACHE_CLOSED",
	CodeInvalidKey:      "INVALID_KEY",
	CodeSerialize:       "SERIALIZE",
	CodeDeserialize:     "DESERIALIZE",
	CodeTimeout:         "TIMEOUT",
	CodeCancelled:       "CANCELLED",
	CodeOverflow:        "OVERFLOW",
	CodeInvalid:         "INVALID",
	CodeConnection:      "CONNECTION",
	CodeEmptyKey:        "EMPTY_KEY",
	CodeExpired:         "EXPIRED",
	CodeNotSupported:    "NOT_SUPPORTED",
	CodeAlreadyExists:   "ALREADY_EXISTS",
	CodeLockFailed:      "LOCK_FAILED",
	CodeRateLimited:     "RATE_LIMITED",
	CodeCircuitOpen:     "CIRCUIT_OPEN",
	CodeNoAvailableNode: "NO_AVAILABLE_NODE",
	CodeUnsupportedType: "UNSUPPORTED_TYPE",
}

// String returns the human-readable name for the error code.
// Returns "UNKNOWN" for unrecognized codes.
func (c ErrorCode) String() string {
	if name, ok := errorCodeNames[c]; ok {
		return name
	}
	return "UNKNOWN"
}

// retryableCodes defines which error codes should trigger automatic retries.
var retryableCodes = map[ErrorCode]bool{
	CodeConnection:      true,
	CodeTimeout:         true,
	CodeCircuitOpen:     true,
	CodeNoAvailableNode: true,
	CodeLockFailed:      true,
	CodeOverflow:        true,
	CodeRateLimited:     true,
}

// IsRetryable returns true if this error code represents a transient failure
// that may succeed on retry.
func (c ErrorCode) IsRetryable() bool {
	return retryableCodes[c]
}
