package errors

import (
	"errors"
	"testing"
)

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		c    ErrorCode
		want string
	}{
		{CodeOK, "ok"},
		{CodeNotFound, "not_found"},
		{CodeCacheClosed, "cache_closed"},
		{CodeInvalidKey, "invalid_key"},
		{CodeSerialize, "serialize_error"},
		{CodeDeserialize, "deserialize_error"},
		{CodeTimeout, "timeout"},
		{CodeCancelled, "cancelled"},
		{CodeOverflow, "overflow"},
		{CodeInvalid, "invalid"},
		{CodeConnection, "connection_error"},
		{CodeEmptyKey, "empty_key"},
		{CodeExpired, "expired"},
		{CodeNotSupported, "not_supported"},
		{CodeAlreadyExists, "already_exists"},
		{CodeLockFailed, "lock_failed"},
		{CodeRateLimited, "rate_limited"},
		{CodeCircuitOpen, "circuit_open"},
		{CodeNoAvailableNode, "no_available_node"},
		{CodeUnsupportedType, "unsupported"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("ErrorCode(%d).String() = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestErrorCode_Unknown(t *testing.T) {
	if got := ErrorCode(99).String(); got != "unknown" {
		t.Errorf("unknown code = %q, want %q", got, "unknown")
	}
}

func TestCacheError_Error(t *testing.T) {
	e := &CacheError{Code: CodeNotFound, Message: "key not found"}
	msg := e.Error()
	if msg != "key not found" {
		t.Errorf("Error() = %q, want %q", msg, "key not found")
	}
}

func TestCacheError_ErrorWithOperation(t *testing.T) {
	e := &CacheError{Code: CodeNotFound, Operation: "cache.get", Message: "key not found"}
	msg := e.Error()
	if msg != "cache.get: key not found" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestCacheError_ErrorWithKey(t *testing.T) {
	e := &CacheError{Operation: "cache.get", Key: "user:1", Message: "not found"}
	msg := e.Error()
	if msg != "cache.get key=user:1: not found" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestCacheError_Is(t *testing.T) {
	a := &CacheError{Code: CodeNotFound}
	b := &CacheError{Code: CodeNotFound}
	c := &CacheError{Code: CodeExpired}
	if !a.Is(b) {
		t.Error("same code should match")
	}
	if a.Is(c) {
		t.Error("different code should not match")
	}
}

func TestCacheError_Is_NonCacheError(t *testing.T) {
	e := &CacheError{Code: CodeNotFound}
	if e.Is(errors.New("other")) {
		t.Error("non-CacheError should not match")
	}
}

func TestCacheError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := &CacheError{Code: CodeConnection, Cause: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should return Cause")
	}
}

func TestCacheError_WithMetadata(t *testing.T) {
	e := &CacheError{Code: CodeTimeout}
	e = e.WithMetadata("retry_after", "5s")
	if e.Metadata["retry_after"] != "5s" {
		t.Error("metadata not set")
	}
}

func TestCacheError_WithMetadata_NilMap(t *testing.T) {
	e := &CacheError{Code: CodeTimeout}
	if e.Metadata != nil {
		t.Error("initial metadata should be nil")
	}
	e.WithMetadata("k", "v")
	if e.Metadata == nil {
		t.Error("metadata should be initialized")
	}
}

func TestNew(t *testing.T) {
	cause := ErrNotFound
	e := New("test.op", "mykey", cause)
	if e.Operation != "test.op" {
		t.Errorf("Operation = %q, want %q", e.Operation, "test.op")
	}
	if e.Key != "mykey" {
		t.Errorf("Key = %q, want %q", e.Key, "mykey")
	}
	if e.Cause != cause {
		t.Error("Cause should be set")
	}
	if e.Code != CodeNotFound {
		t.Errorf("Code = %d, want CodeNotFound", e.Code)
	}
}

func TestWrap(t *testing.T) {
	result := Wrap("test.op", nil)
	if result != nil {
		t.Error("Wrap(nil) should return nil")
	}
	result = Wrap("test.op", ErrTimeout)
	if result == nil {
		t.Fatal("Wrap(non-nil) should return non-nil")
	}
	if result.Operation != "test.op" {
		t.Error("Operation should be set")
	}
}

func TestWrapKey(t *testing.T) {
	result := WrapKey("test.op", "mykey", nil)
	if result != nil {
		t.Error("WrapKey(nil) should return nil")
	}
	result = WrapKey("test.op", "mykey", ErrNotFound)
	if result == nil {
		t.Fatal("WrapKey should return non-nil")
	}
	if result.Key != "mykey" {
		t.Error("Key should be set")
	}
}

func TestNotFound(t *testing.T) {
	e := NotFound("cache.get", "user:1")
	if e.Code != CodeNotFound {
		t.Errorf("Code = %d, want CodeNotFound", e.Code)
	}
	if e.Operation != "cache.get" {
		t.Errorf("Operation = %q", e.Operation)
	}
}

func TestEmptyKey(t *testing.T) {
	e := EmptyKey("cache.set")
	if e.Code != CodeEmptyKey {
		t.Errorf("Code = %d, want CodeEmptyKey", e.Code)
	}
}

func TestExpired(t *testing.T) {
	e := Expired("cache.get", "key")
	if e.Code != CodeExpired {
		t.Error("Code should be CodeExpired")
	}
}

func TestClosed(t *testing.T) {
	e := Closed("cache.set")
	if e.Code != CodeCacheClosed {
		t.Error("Code should be CodeCacheClosed")
	}
}

func TestInvalidKey(t *testing.T) {
	inner := errors.New("bad chars")
	e := InvalidKey("cache.set", "key!!", inner)
	if e.Code != CodeInvalidKey {
		t.Error("Code should be CodeInvalidKey")
	}
	if !errors.Is(e, inner) {
		t.Error("should wrap cause")
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("ErrNotFound should be IsNotFound")
	}
	if IsNotFound(ErrTimeout) {
		t.Error("ErrTimeout should not be IsNotFound")
	}
	if IsNotFound(nil) {
		t.Error("nil should not be IsNotFound")
	}
}

func TestIsExpired(t *testing.T) {
	if !IsExpired(ErrExpired) {
		t.Error("ErrExpired should be IsExpired")
	}
}

func TestIsCacheClosed(t *testing.T) {
	if !IsCacheClosed(ErrClosed) {
		t.Error("ErrClosed should be IsCacheClosed")
	}
}

func TestIsTimeout(t *testing.T) {
	if !IsTimeout(ErrTimeout) {
		t.Error("ErrTimeout should be IsTimeout")
	}
}

func TestIsCancelled(t *testing.T) {
	if !IsCancelled(ErrCancelled) {
		t.Error("ErrCancelled should be IsCancelled")
	}
}

func TestIsRateLimited(t *testing.T) {
	if !IsRateLimited(ErrRateLimited) {
		t.Error("ErrRateLimited should be IsRateLimited")
	}
}

func TestIsCircuitOpen(t *testing.T) {
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Error("ErrCircuitOpen should be IsCircuitOpen")
	}
}

func TestIsConnectionError(t *testing.T) {
	if !IsConnectionError(ErrConnectionLost) {
		t.Error("ErrConnectionLost should be IsConnectionError")
	}
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{ErrTimeout, true},
		{ErrConnectionLost, true},
		{ErrRateLimited, true},
		{ErrNotFound, false},
		{ErrClosed, false},
	}
	for _, tt := range tests {
		if got := Retryable(tt.err); got != tt.want {
			t.Errorf("Retryable(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestCodeOf(t *testing.T) {
	if got := CodeOf(ErrNotFound); got != CodeNotFound {
		t.Errorf("CodeOf(ErrNotFound) = %d", got)
	}
	if got := CodeOf(errors.New("other")); got != CodeInvalid {
		t.Errorf("CodeOf(other) = %d, want CodeInvalid", got)
	}
}

func TestSentinelErrors_NonNil(t *testing.T) {
	sentinels := []error{
		ErrNotFound, ErrExpired, ErrNilValue, ErrInvalidConfig, ErrConnectionLost,
		ErrMarshal, ErrUnmarshal, ErrCacheFull, ErrClosed, ErrInvalidKey, ErrTimeout,
		ErrCancelled, ErrEmptyKey, ErrNotSupported, ErrAlreadyExists, ErrLockFailed,
		ErrRateLimited, ErrCircuitOpen, ErrNoAvailableNode, ErrUnsupportedType,
	}
	for _, e := range sentinels {
		if e == nil {
			t.Errorf("sentinel error should not be nil: %v", e)
		}
	}
}

func TestAsType(t *testing.T) {
	e := NotFound("test", "key")
	ce, ok := AsType(e)
	if !ok {
		t.Fatal("AsType should return true for CacheError")
	}
	if ce.Code != CodeNotFound {
		t.Error("wrong code")
	}
	_, ok = AsType(errors.New("other"))
	if ok {
		t.Error("AsType should return false for non-CacheError")
	}
}

func TestIs(t *testing.T) {
	if !Is(ErrNotFound, CodeNotFound) {
		t.Error("Is(ErrNotFound, CodeNotFound) should be true")
	}
	if Is(ErrNotFound, CodeExpired) {
		t.Error("Is(ErrNotFound, CodeExpired) should be false")
	}
}

func TestGetMetadata(t *testing.T) {
	e := &CacheError{Code: CodeTimeout}
	e.WithMetadata("attempt", 3)
	val, ok := GetMetadata(e, "attempt")
	if !ok {
		t.Fatal("metadata should exist")
	}
	if val != 3 {
		t.Errorf("got %v, want 3", val)
	}
	_, ok = GetMetadata(e, "nonexistent")
	if ok {
		t.Error("nonexistent metadata should not be found")
	}
}

func TestGetMetadata_NoCacheError(t *testing.T) {
	_, ok := GetMetadata(errors.New("other"), "key")
	if ok {
		t.Error("non-CacheError should not have metadata")
	}
}
