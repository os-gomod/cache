package errors

import (
	"errors"
	"testing"
)

func TestNotFound(t *testing.T) {
	err := NotFound("cache.get", "mykey")
	if !IsNotFound(err) {
		t.Error("IsNotFound should return true for NotFound error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("should match ErrNotFound sentinel")
	}
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeNotFound {
		t.Errorf("Code = %d, want %d", ce.Code, CodeNotFound)
	}
	if ce.Operation != "cache.get" {
		t.Errorf("Operation = %q, want %q", ce.Operation, "cache.get")
	}
	if ce.Key != "mykey" {
		t.Errorf("Key = %q, want %q", ce.Key, "mykey")
	}
}

func TestClosed(t *testing.T) {
	err := Closed("cache.set")
	if !IsCacheClosed(err) {
		t.Error("IsCacheClosed should return true")
	}
}

func TestEmptyKey(t *testing.T) {
	err := EmptyKey("cache.get")
	if !Is(err, CodeEmptyKey) {
		t.Error("Is(CodeEmptyKey) should return true")
	}
}

func TestTimeoutError(t *testing.T) {
	err := TimeoutError("cache.get")
	if !IsTimeout(err) {
		t.Error("IsTimeout should return true")
	}
	if !Retryable(err) {
		t.Error("Timeout errors should be retryable")
	}
}

func TestCancelledError(t *testing.T) {
	err := CancelledError("cache.get")
	if !IsCancelled(err) {
		t.Error("IsCancelled should return true")
	}
}

func TestRateLimitedError(t *testing.T) {
	err := RateLimitedError("cache.get")
	if !IsRateLimited(err) {
		t.Error("IsRateLimited should return true")
	}
	if !Retryable(err) {
		t.Error("RateLimited errors should be retryable")
	}
}

func TestCircuitOpenError(t *testing.T) {
	err := CircuitOpenError("cache.get")
	if !IsCircuitOpen(err) {
		t.Error("IsCircuitOpen should return true")
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("connection refused")
	err := Wrap("cache.get", inner)
	if err == nil {
		t.Fatal("Wrap should return non-nil")
	}

	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Operation != "cache.get" {
		t.Errorf("Operation = %q, want %q", ce.Operation, "cache.get")
	}
}

func TestWrap_Nil(t *testing.T) {
	err := Wrap("cache.get", nil)
	if err != nil {
		t.Error("Wrap with nil cause should return nil")
	}
}

func TestWrapKey(t *testing.T) {
	inner := errors.New("timeout")
	err := WrapKey("cache.get", "mykey", inner)
	if err == nil {
		t.Fatal("WrapKey should return non-nil")
	}

	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Key != "mykey" {
		t.Errorf("Key = %q, want %q", ce.Key, "mykey")
	}
}

func TestWrapKey_Nil(t *testing.T) {
	err := WrapKey("cache.get", "key", nil)
	if err != nil {
		t.Error("WrapKey with nil cause should return nil")
	}
}

func TestWithMetadata(t *testing.T) {
	err := NotFound("cache.get", "mykey")
	err = err.WithMetadata("region", "us-east-1")

	val, ok := GetMetadata(err, "region")
	if !ok || val != "us-east-1" {
		t.Errorf("GetMetadata = %v, %v; want us-east-1, true", val, ok)
	}

	_, ok = GetMetadata(err, "missing")
	if ok {
		t.Error("GetMetadata for missing key should return false")
	}
}

func TestGetMetadata_NonCacheError(t *testing.T) {
	err := errors.New("plain error")
	_, ok := GetMetadata(err, "key")
	if ok {
		t.Error("GetMetadata on non-CacheError should return false")
	}
}

func TestCodeOf(t *testing.T) {
	err := NotFound("op", "key")
	if code := CodeOf(err); code != CodeNotFound {
		t.Errorf("CodeOf = %d, want %d", code, CodeNotFound)
	}

	plainErr := errors.New("plain")
	if code := CodeOf(plainErr); code != CodeInvalid {
		t.Errorf("CodeOf(plain) = %d, want %d", code, CodeInvalid)
	}
}

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{CodeOK, "ok"},
		{CodeNotFound, "not_found"},
		{CodeCacheClosed, "cache_closed"},
		{CodeInvalidKey, "invalid_key"},
		{CodeTimeout, "timeout"},
		{CodeCancelled, "cancelled"},
		{CodeRateLimited, "rate_limited"},
		{CodeCircuitOpen, "circuit_open"},
		{ErrorCode(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("ErrorCode(%d).String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCacheError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *CacheError
		want string
	}{
		{
			"operation only",
			&CacheError{Operation: "cache.get", Message: "not found"},
			"cache.get: not found",
		},
		{
			"with key",
			&CacheError{Operation: "cache.get", Key: "mykey", Message: "not found"},
			"cache.get key=mykey: not found",
		},
		{
			"with cause",
			&CacheError{Operation: "cache.set", Message: "failed", Cause: errors.New("timeout")},
			"cache.set: failed: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{TimeoutError("op"), true},
		{ConnectionError("op", errors.New("conn")), true},
		{RateLimitedError("op"), true},
		{NotFound("op", "key"), false},
		{Closed("op"), false},
	}
	for _, tt := range tests {
		if got := Retryable(tt.err); got != tt.want {
			t.Errorf("Retryable(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestConnectionError(t *testing.T) {
	err := ConnectionError("cache.connect", errors.New("refused"))
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeConnection {
		t.Errorf("Code = %d, want %d", ce.Code, CodeConnection)
	}
}

func TestSerializeError(t *testing.T) {
	err := SerializeError("cache.set", errors.New("json"))
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeSerialize {
		t.Errorf("Code = %d, want %d", ce.Code, CodeSerialize)
	}
}

func TestDeserializeError(t *testing.T) {
	err := DeserializeError("cache.get", errors.New("json"))
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeDeserialize {
		t.Errorf("Code = %d, want %d", ce.Code, CodeDeserialize)
	}
}

func TestInvalidConfig(t *testing.T) {
	err := InvalidConfig("cache.new", "bad config")
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeInvalid {
		t.Errorf("Code = %d, want %d", ce.Code, CodeInvalid)
	}
}

func TestInvalidKey(t *testing.T) {
	err := InvalidKey("cache.get", "key", errors.New("bad"))
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeInvalidKey {
		t.Errorf("Code = %d, want %d", ce.Code, CodeInvalidKey)
	}
}

func TestLockFailedError(t *testing.T) {
	err := LockFailedError("cache.lock", "key")
	ce, ok := AsType(err)
	if !ok {
		t.Fatal("AsType should succeed")
	}
	if ce.Code != CodeLockFailed {
		t.Errorf("Code = %d, want %d", ce.Code, CodeLockFailed)
	}
}
