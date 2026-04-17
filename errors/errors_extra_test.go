package errors

import (
	"errors"
	"testing"
)

func TestCacheError_Error_WithCause(t *testing.T) {
	inner := errors.New("inner error")
	e := &CacheError{Code: CodeConnection, Message: "connection failed", Cause: inner}
	msg := e.Error()
	if msg != "connection failed: inner error" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestCacheError_Error_CauseIsCacheError(t *testing.T) {
	inner := &CacheError{Code: CodeTimeout, Message: "timeout"}
	outer := &CacheError{Code: CodeConnection, Message: "wrapper", Cause: inner}
	msg := outer.Error()
	if msg != "wrapper: timeout" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestCacheError_Error_WithOperationAndCause(t *testing.T) {
	inner := errors.New("db error")
	e := &CacheError{Operation: "cache.get", Key: "user:1", Cause: inner}
	msg := e.Error()
	if msg != "cache.get key=user:1: db error" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestCacheError_Error_OnlyKey(t *testing.T) {
	e := &CacheError{Key: "user:1"}
	msg := e.Error()
	// With only key and no operation/message/cause, the output is "key=user:1"
	if msg != "key=user:1" {
		t.Errorf("Error() = %q, want %q", msg, "key=user:1")
	}
}

func TestCacheError_Error_NilCause(t *testing.T) {
	e := &CacheError{Code: CodeOK}
	msg := e.Error()
	if msg != "" {
		t.Errorf("Error() = %q, want empty", msg)
	}
}

func TestRenderCause_Nil(t *testing.T) {
	if renderCause(nil) != "" {
		t.Error("renderCause(nil) should return empty")
	}
}

func TestRenderCause_NonCacheError(t *testing.T) {
	err := errors.New("plain error")
	if renderCause(err) != "plain error" {
		t.Errorf("renderCause = %q, want %q", renderCause(err), "plain error")
	}
}

func TestRenderCause_NestedCacheErrorNoOp(t *testing.T) {
	inner := errors.New("root cause")
	middle := &CacheError{Cause: inner}
	outer := &CacheError{Cause: middle}
	msg := outer.Error()
	if msg != "root cause" {
		t.Errorf("Error() = %q, want %q", msg, "root cause")
	}
}

func TestRenderCause_NestedCacheErrorWithOp(t *testing.T) {
	inner := errors.New("root cause")
	middle := &CacheError{Operation: "inner.op", Cause: inner}
	outer := &CacheError{Operation: "outer.op", Cause: middle}
	msg := outer.Error()
	if msg != "outer.op: inner.op: root cause" {
		t.Errorf("Error() = %q", msg)
	}
}

func TestSerializeError(t *testing.T) {
	e := SerializeError("test", errors.New("json err"))
	if e.Code != CodeSerialize {
		t.Error("Code should be CodeSerialize")
	}
	if e.Operation != "test" {
		t.Error("Operation should be set")
	}
}

func TestDeserializeError(t *testing.T) {
	e := DeserializeError("test", errors.New("json err"))
	if e.Code != CodeDeserialize {
		t.Error("Code should be CodeDeserialize")
	}
}

func TestTimeoutError(t *testing.T) {
	e := TimeoutError("cache.get")
	if e.Code != CodeTimeout {
		t.Error("Code should be CodeTimeout")
	}
}

func TestCancelledError(t *testing.T) {
	e := CancelledError("cache.get")
	if e.Code != CodeCancelled {
		t.Error("Code should be CodeCancelled")
	}
}

func TestRateLimitedError(t *testing.T) {
	e := RateLimitedError("cache.get")
	if e.Code != CodeRateLimited {
		t.Error("Code should be CodeRateLimited")
	}
}

func TestCircuitOpenError(t *testing.T) {
	e := CircuitOpenError("cache.get")
	if e.Code != CodeCircuitOpen {
		t.Error("Code should be CodeCircuitOpen")
	}
}

func TestLockFailedError(t *testing.T) {
	e := LockFailedError("cache.set", "user:1")
	if e.Code != CodeLockFailed {
		t.Error("Code should be CodeLockFailed")
	}
	if e.Key != "user:1" {
		t.Error("Key should be set")
	}
}

func TestConnectionError(t *testing.T) {
	e := ConnectionError("cache.get", errors.New("conn err"))
	if e.Code != CodeConnection {
		t.Error("Code should be CodeConnection")
	}
}

func TestConnectionError_NilCause(t *testing.T) {
	e := ConnectionError("test", nil)
	if e == nil {
		t.Error("ConnectionError should return non-nil even with nil cause")
	}
	if e.Code != CodeConnection {
		t.Error("Code should be CodeConnection")
	}
}

func TestInvalidConfig(t *testing.T) {
	e := InvalidConfig("test", "bad config")
	if e.Code != CodeInvalid {
		t.Error("Code should be CodeInvalid")
	}
	if e.Message != "bad config" {
		t.Errorf("Message = %q, want %q", e.Message, "bad config")
	}
}

func TestNotFound_Factory(t *testing.T) {
	e := NotFound("cache.get", "user:1")
	if e.Code != CodeNotFound {
		t.Error("Code should be CodeNotFound")
	}
	if e.Key != "user:1" {
		t.Error("Key should be set")
	}
}

func TestExpired_Factory(t *testing.T) {
	e := Expired("cache.get", "user:1")
	if e.Code != CodeExpired {
		t.Error("Code should be CodeExpired")
	}
}

func TestClosed_Factory(t *testing.T) {
	e := Closed("cache.set")
	if e.Code != CodeCacheClosed {
		t.Error("Code should be CodeCacheClosed")
	}
}

func TestEmptyKey_Factory(t *testing.T) {
	e := EmptyKey("cache.set")
	if e.Code != CodeEmptyKey {
		t.Error("Code should be CodeEmptyKey")
	}
}

func TestNew_WithNonCacheError(t *testing.T) {
	cause := errors.New("plain error")
	e := New("test.op", "mykey", cause)
	if e.Code != CodeInvalid {
		t.Errorf("Code = %d, want CodeInvalid (for non-CacheError)", e.Code)
	}
}

func TestIs_NonCacheError(t *testing.T) {
	if Is(errors.New("other"), CodeNotFound) {
		t.Error("Is should return false for non-CacheError")
	}
	if Is(nil, CodeNotFound) {
		t.Error("Is should return false for nil")
	}
}

func TestGetMetadata_NilMetadata(t *testing.T) {
	e := &CacheError{Code: CodeTimeout}
	_, ok := GetMetadata(e, "key")
	if ok {
		t.Error("should return false when metadata is nil")
	}
}

func TestWithMetadata_Chain(t *testing.T) {
	e := &CacheError{Code: CodeTimeout}
	e = e.WithMetadata("retry", 3)
	e = e.WithMetadata("backoff", "100ms")
	if e.Metadata["retry"] != 3 {
		t.Error("retry metadata not set")
	}
	if e.Metadata["backoff"] != "100ms" {
		t.Error("backoff metadata not set")
	}
}

func TestUnmarshalSentinel(t *testing.T) {
	if ErrUnmarshal == nil {
		t.Error("ErrUnmarshal should not be nil")
	}
	if ErrUnmarshal.Cause == nil {
		t.Error("ErrUnmarshal should have a cause")
	}
	if !errors.Is(ErrUnmarshal, ErrUnmarshal.Cause) {
		t.Error("ErrUnmarshal should wrap its cause")
	}
}

func TestErrorCode_Negative(t *testing.T) {
	if got := ErrorCode(-1).String(); got != "unknown" {
		t.Errorf("negative code = %q, want %q", got, "unknown")
	}
}
