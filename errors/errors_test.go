// Package errors_test provides tests for the centralized error handling.
package errors_test

import (
	"errors"
	"testing"

	_errors "github.com/os-gomod/cache/errors"
)

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		name     string
		code     _errors.ErrorCode
		expected string
	}{
		{"OK", _errors.CodeOK, "ok"},
		{"NotFound", _errors.CodeNotFound, "not_found"},
		{"CacheClosed", _errors.CodeCacheClosed, "cache_closed"},
		{"InvalidKey", _errors.CodeInvalidKey, "invalid_key"},
		{"Serialize", _errors.CodeSerialize, "serialize_error"},
		{"Deserialize", _errors.CodeDeserialize, "deserialize_error"},
		{"Timeout", _errors.CodeTimeout, "timeout"},
		{"Cancelled", _errors.CodeCancelled, "cancelled"},
		{"Overflow", _errors.CodeOverflow, "overflow"},
		{"Invalid", _errors.CodeInvalid, "invalid"},
		{"Connection", _errors.CodeConnection, "connection_error"},
		{"EmptyKey", _errors.CodeEmptyKey, "empty_key"},
		{"Expired", _errors.CodeExpired, "expired"},
		{"NotSupported", _errors.CodeNotSupported, "not_supported"},
		{"AlreadyExists", _errors.CodeAlreadyExists, "already_exists"},
		{"LockFailed", _errors.CodeLockFailed, "lock_failed"},
		{"RateLimited", _errors.CodeRateLimited, "rate_limited"},
		{"CircuitOpen", _errors.CodeCircuitOpen, "circuit_open"},
		{"NoAvailableNode", _errors.CodeNoAvailableNode, "no_available_node"},
		{"Unknown", _errors.ErrorCode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("ErrorCode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCacheError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *_errors.CacheError
		expected string
	}{
		{
			name:     "minimal error",
			err:      &_errors.CacheError{Code: _errors.CodeInvalid, Message: "test error"},
			expected: "test error",
		},
		{
			name: "with operation",
			err: &_errors.CacheError{
				Code:      _errors.CodeNotFound,
				Operation: "Get",
				Message:   "key not found",
			},
			expected: "Get: key not found",
		},
		{
			name: "with key",
			err: &_errors.CacheError{
				Code:    _errors.CodeInvalidKey,
				Key:     "my-key",
				Message: "invalid key",
			},
			expected: "key=my-key: invalid key",
		},
		{
			name: "with operation and key",
			err: &_errors.CacheError{
				Code:      _errors.CodeNotFound,
				Operation: "Get",
				Key:       "my-key",
				Message:   "key not found",
			},
			expected: "Get key=my-key: key not found",
		},
		{
			name: "with cause",
			err: &_errors.CacheError{
				Code:      _errors.CodeSerialize,
				Operation: "Marshal",
				Message:   "serialization failed",
				Cause:     _errors.ErrUnmarshal,
			},
			expected: "Marshal: serialization failed: json: unsupported type",
		},
		{
			name: "full error",
			err: &_errors.CacheError{
				Code:      _errors.CodeTimeout,
				Operation: "Get",
				Key:       "my-key",
				Message:   "operation timed out",
				Cause:     errors.New("deadline exceeded"),
			},
			expected: "Get key=my-key: operation timed out: deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("CacheError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCacheError_Unwrap(t *testing.T) {
	cause := errors.New("underlying cause")
	err := &_errors.CacheError{
		Code:    _errors.CodeInvalid,
		Message: "wrapped error",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestCacheError_Is(t *testing.T) {
	err1 := &_errors.CacheError{Code: _errors.CodeNotFound, Message: "not found"}
	err2 := &_errors.CacheError{Code: _errors.CodeNotFound, Message: "different message"}
	err3 := &_errors.CacheError{Code: _errors.CodeTimeout, Message: "timeout"}

	tests := []struct {
		name   string
		err    *_errors.CacheError
		target error
		want   bool
	}{
		{"same code", err1, err2, true},
		{"different code", err1, err3, false},
		{"different type", err1, errors.New("standard error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Is(tt.target); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheError_WithMetadata(t *testing.T) {
	err := &_errors.CacheError{Code: _errors.CodeInvalid, Message: "test"}

	// Add metadata
	result := err.WithMetadata("key1", "value1")
	if result != err {
		t.Error("WithMetadata should return the same error instance")
	}

	if len(err.Metadata) != 1 {
		t.Errorf("expected 1 metadata entry, got %d", len(err.Metadata))
	}

	if val, ok := err.Metadata["key1"]; !ok || val != "value1" {
		t.Errorf("expected key1=value1, got %v", val)
	}

	// Add another metadata
	err.WithMetadata("key2", 42)
	if len(err.Metadata) != 2 {
		t.Errorf("expected 2 metadata entries, got %d", len(err.Metadata))
	}
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
		code _errors.ErrorCode
	}{
		{"ErrNotFound", _errors.ErrNotFound, _errors.CodeNotFound},
		{"ErrExpired", _errors.ErrExpired, _errors.CodeExpired},
		{"ErrNilValue", _errors.ErrNilValue, _errors.CodeInvalid},
		{"ErrInvalidConfig", _errors.ErrInvalidConfig, _errors.CodeInvalid},
		{"ErrConnectionLost", _errors.ErrConnectionLost, _errors.CodeConnection},
		{"ErrMarshal", _errors.ErrMarshal, _errors.CodeSerialize},
		{"ErrUnmarshal", _errors.ErrUnmarshal, _errors.CodeDeserialize},
		{"ErrCacheFull", _errors.ErrCacheFull, _errors.CodeOverflow},
		{"ErrClosed", _errors.ErrClosed, _errors.CodeCacheClosed},
		{"ErrInvalidKey", _errors.ErrInvalidKey, _errors.CodeInvalidKey},
		{"ErrTimeout", _errors.ErrTimeout, _errors.CodeTimeout},
		{"ErrCancelled", _errors.ErrCancelled, _errors.CodeCancelled},
		{"ErrEmptyKey", _errors.ErrEmptyKey, _errors.CodeEmptyKey},
		{"ErrNotSupported", _errors.ErrNotSupported, _errors.CodeNotSupported},
		{"ErrAlreadyExists", _errors.ErrAlreadyExists, _errors.CodeAlreadyExists},
		{"ErrLockFailed", _errors.ErrLockFailed, _errors.CodeLockFailed},
		{"ErrRateLimited", _errors.ErrRateLimited, _errors.CodeRateLimited},
		{"ErrCircuitOpen", _errors.ErrCircuitOpen, _errors.CodeCircuitOpen},
		{"ErrNoAvailableNode", _errors.ErrNoAvailableNode, _errors.CodeNoAvailableNode},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Error("sentinel error is nil")
			}

			ce, ok := s.err.(*_errors.CacheError)
			if !ok {
				t.Errorf("sentinel error is not *CacheError")
			}

			if ce.Code != s.code {
				t.Errorf("code = %v, want %v", ce.Code, s.code)
			}
		})
	}
}

func TestNew(t *testing.T) {
	cause := errors.New("underlying error")
	err := _errors.New("Get", "my-key", cause)

	ce, ok := err.(*_errors.CacheError)
	if !ok {
		t.Fatal("expected *CacheError")
	}

	if ce.Operation != "Get" {
		t.Errorf("operation = %v, want Get", ce.Operation)
	}
	if ce.Key != "my-key" {
		t.Errorf("key = %v, want my-key", ce.Key)
	}
	if ce.Cause != cause {
		t.Error("cause not set correctly")
	}
	if ce.Code != _errors.CodeInvalid {
		t.Errorf("code = %v, want CodeInvalid", ce.Code)
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		op    string
		cause error
		want  error
	}{
		{"nil cause", "Get", nil, nil},
		{"with cause", "Get", errors.New("some error"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := _errors.Wrap(tt.op, tt.cause)
			if tt.cause == nil {
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected non-nil error")
			}

			ce, ok := err.(*_errors.CacheError)
			if !ok {
				t.Fatal("expected *CacheError")
			}
			if ce.Operation != tt.op {
				t.Errorf("operation = %v, want %v", ce.Operation, tt.op)
			}
			if ce.Cause != tt.cause {
				t.Error("cause not preserved")
			}
		})
	}
}

func TestWrapKey(t *testing.T) {
	cause := errors.New("some error")
	err := _errors.WrapKey("Get", "my-key", cause)

	ce, ok := err.(*_errors.CacheError)
	if !ok {
		t.Fatal("expected *CacheError")
	}
	if ce.Operation != "Get" {
		t.Errorf("operation = %v, want Get", ce.Operation)
	}
	if ce.Key != "my-key" {
		t.Errorf("key = %v, want my-key", ce.Key)
	}
	if ce.Cause != cause {
		t.Error("cause not preserved")
	}
}

func TestConstructorFunctions(t *testing.T) {
	tests := []struct {
		name      string
		construct func() error
		code      _errors.ErrorCode
		operation string
		key       string
	}{
		{
			"InvalidConfig",
			func() error { return _errors.InvalidConfig("Init", "invalid config") },
			_errors.CodeInvalid,
			"Init",
			"",
		},
		{
			"NotFound",
			func() error { return _errors.NotFound("Get", "key123") },
			_errors.CodeNotFound,
			"Get",
			"key123",
		},
		{
			"Expired",
			func() error { return _errors.Expired("Get", "key123") },
			_errors.CodeExpired,
			"Get",
			"key123",
		},
		{
			"Closed",
			func() error { return _errors.Closed("Get") },
			_errors.CodeCacheClosed,
			"Get",
			"",
		},
		{
			"InvalidKey",
			func() error { return _errors.InvalidKey("Set", "invalid", nil) },
			_errors.CodeInvalidKey,
			"Set",
			"invalid",
		},
		{
			"EmptyKey",
			func() error { return _errors.EmptyKey("Get") },
			_errors.CodeEmptyKey,
			"Get",
			"",
		},
		{
			"SerializeError",
			func() error { return _errors.SerializeError("Marshal", nil) },
			_errors.CodeSerialize,
			"Marshal",
			"",
		},
		{
			"DeserializeError",
			func() error { return _errors.DeserializeError("Unmarshal", nil) },
			_errors.CodeDeserialize,
			"Unmarshal",
			"",
		},
		{
			"TimeoutError",
			func() error { return _errors.TimeoutError("Get") },
			_errors.CodeTimeout,
			"Get",
			"",
		},
		{
			"CancelledError",
			func() error { return _errors.CancelledError("Get") },
			_errors.CodeCancelled,
			"Get",
			"",
		},
		{
			"LockFailedError",
			func() error { return _errors.LockFailedError("Lock", "key123") },
			_errors.CodeLockFailed,
			"Lock",
			"key123",
		},
		{
			"ConnectionError",
			func() error { return _errors.ConnectionError("Connect", nil) },
			_errors.CodeConnection,
			"Connect",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.construct()
			if err == nil {
				t.Fatal("expected non-nil error")
			}

			ce, ok := err.(*_errors.CacheError)
			if !ok {
				t.Fatal("expected *CacheError")
			}
			if ce.Code != tt.code {
				t.Errorf("code = %v, want %v", ce.Code, tt.code)
			}
			if ce.Operation != tt.operation {
				t.Errorf("operation = %v, want %v", ce.Operation, tt.operation)
			}
			if ce.Key != tt.key {
				t.Errorf("key = %v, want %v", ce.Key, tt.key)
			}
		})
	}
}

func TestCodeOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want _errors.ErrorCode
	}{
		{"nil error", nil, _errors.CodeInvalid},
		{"standard error", errors.New("standard"), _errors.CodeInvalid},
		{"sentinel error", _errors.ErrNotFound, _errors.CodeNotFound},
		{"wrapped sentinel", _errors.Wrap("Get", _errors.ErrNotFound), _errors.CodeNotFound},
		{"constructed error", _errors.InvalidConfig("Init", "msg"), _errors.CodeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := _errors.CodeOf(tt.err); got != tt.want {
				t.Errorf("CodeOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFunctions(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(error) bool
		err    error
		expect bool
	}{
		{"IsNotFound", _errors.IsNotFound, _errors.ErrNotFound, true},
		{"IsNotFound", _errors.IsNotFound, _errors.NotFound("Get", "key"), true},
		{"IsNotFound", _errors.IsNotFound, errors.New("other"), false},
		{"IsExpired", _errors.IsExpired, _errors.ErrExpired, true},
		{"IsExpired", _errors.IsExpired, _errors.Expired("Get", "key"), true},
		{"IsCacheClosed", _errors.IsCacheClosed, _errors.ErrClosed, true},
		{"IsCacheClosed", _errors.IsCacheClosed, _errors.ErrClosed, true},
		{"IsTimeout", _errors.IsTimeout, _errors.ErrTimeout, true},
		{"IsTimeout", _errors.IsTimeout, _errors.TimeoutError("Get"), true},
		{"IsCancelled", _errors.IsCancelled, _errors.ErrCancelled, true},
		{"IsCancelled", _errors.IsCancelled, _errors.CancelledError("Get"), true},
		{"IsRateLimited", _errors.IsRateLimited, _errors.ErrRateLimited, true},
		{"IsRateLimited", _errors.IsRateLimited, _errors.RateLimitedError("Get"), true},
		{"IsCircuitOpen", _errors.IsCircuitOpen, _errors.ErrCircuitOpen, true},
		{"IsCircuitOpen", _errors.IsCircuitOpen, _errors.CircuitOpenError("Get"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.err); got != tt.expect {
				t.Errorf("%v = %v, want %v", tt.name, got, tt.expect)
			}
		})
	}
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"standard", errors.New("standard"), false},
		{"timeout", _errors.ErrTimeout, true},
		{"connection", _errors.ErrConnectionLost, true},
		{"rate limited", _errors.ErrRateLimited, true},
		{"not found", _errors.ErrNotFound, false},
		{"closed", _errors.ErrClosed, false},
		{"wrapped timeout", _errors.Wrap("Get", _errors.ErrTimeout), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := _errors.Retryable(tt.err); got != tt.want {
				t.Errorf("Retryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAsType(t *testing.T) {
	ce := &_errors.CacheError{Code: _errors.CodeNotFound, Message: "test"}
	tests := []struct {
		name     string
		err      error
		wantOk   bool
		wantCode _errors.ErrorCode
	}{
		{"cache error", ce, true, _errors.CodeNotFound},
		{"wrapped", _errors.Wrap("Get", ce), true, _errors.CodeNotFound},
		{"standard", errors.New("standard"), false, 0},
		{"nil", nil, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := _errors.AsType(tt.err)
			if ok != tt.wantOk {
				t.Errorf("AsType() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && got.Code != tt.wantCode {
				t.Errorf("code = %v, want %v", got.Code, tt.wantCode)
			}
		})
	}
}

func TestGetMetadata(t *testing.T) {
	err := &_errors.CacheError{
		Code: _errors.CodeInvalid,
		Metadata: map[string]any{
			"key1": "value1",
			"key2": 42,
		},
	}

	tests := []struct {
		name      string
		err       error
		key       string
		wantVal   any
		wantFound bool
	}{
		{"existing key", err, "key1", "value1", true},
		{"existing int key", err, "key2", 42, true},
		{"non-existing key", err, "key3", nil, false},
		{"nil error", nil, "key1", nil, false},
		{"error without metadata", _errors.ErrNotFound, "key1", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, found := _errors.GetMetadata(tt.err, tt.key)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if val != tt.wantVal {
				t.Errorf("val = %v, want %v", val, tt.wantVal)
			}
		})
	}
}

func TestErrorChain(t *testing.T) {
	// Create an error chain
	baseErr := _errors.ErrConnectionLost
	err := _errors.ConnectionError("Dial", baseErr)
	err = _errors.WrapKey("Get", "user:123", err)

	// Test unwrapping
	ce, ok := _errors.AsType(err)
	if !ok {
		t.Fatal("expected CacheError")
	}
	if ce.Operation != "Get" {
		t.Errorf("operation = %v, want Get", ce.Operation)
	}
	if ce.Key != "user:123" {
		t.Errorf("key = %v, want user:123", ce.Key)
	}

	// Test code extraction through chain
	if code := _errors.CodeOf(err); code != _errors.CodeConnection {
		t.Errorf("CodeOf() = %v, want CodeConnection", code)
	}

	// Test sentinel detection through chain
	if _errors.IsCacheClosed(err) {
		t.Error("IsCacheClosed should be false")
	}
	if _errors.IsTimeout(err) {
		t.Error("IsTimeout should be false")
	}
}
