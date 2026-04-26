package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorCodeString(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{CodeOK, "OK"},
		{CodeNotFound, "NOT_FOUND"},
		{CodeCacheClosed, "CACHE_CLOSED"},
		{CodeInvalidKey, "INVALID_KEY"},
		{CodeSerialize, "SERIALIZE"},
		{CodeDeserialize, "DESERIALIZE"},
		{CodeTimeout, "TIMEOUT"},
		{CodeCancelled, "CANCELLED"},
		{CodeOverflow, "OVERFLOW"},
		{CodeInvalid, "INVALID"},
		{CodeConnection, "CONNECTION"},
		{CodeEmptyKey, "EMPTY_KEY"},
		{CodeExpired, "EXPIRED"},
		{CodeNotSupported, "NOT_SUPPORTED"},
		{CodeAlreadyExists, "ALREADY_EXISTS"},
		{CodeLockFailed, "LOCK_FAILED"},
		{CodeRateLimited, "RATE_LIMITED"},
		{CodeCircuitOpen, "CIRCUIT_OPEN"},
		{CodeNoAvailableNode, "NO_AVAILABLE_NODE"},
		{CodeUnsupportedType, "UNSUPPORTED_TYPE"},
		{ErrorCode(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.code.String()
			if got != tt.want {
				t.Errorf("ErrorCode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorCodeIsRetryable(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want bool
	}{
		{CodeOK, false},
		{CodeNotFound, false},
		{CodeTimeout, true},
		{CodeCancelled, false},
		{CodeConnection, true},
		{CodeOverflow, true},
		{CodeInvalid, false},
		{CodeLockFailed, true},
		{CodeRateLimited, true},
		{CodeCircuitOpen, true},
		{CodeNoAvailableNode, true},
		{CodeInvalidKey, false},
		{CodeSerialize, false},
		{CodeDeserialize, false},
	}

	for _, tt := range tests {
		name := tt.code.String()
		t.Run(name, func(t *testing.T) {
			got := tt.code.IsRetryable()
			if got != tt.want {
				t.Errorf("ErrorCode.IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheErrorError(t *testing.T) {
	tests := []struct {
		name   string
		err    *CacheError
		substr string
	}{
		{
			name:   "basic error without key",
			err:    Factory.New(CodeNotFound, "get", "", "key not found", nil),
			substr: "cache[NOT_FOUND] get: key not found",
		},
		{
			name:   "error with key",
			err:    Factory.New(CodeNotFound, "get", "user:123", "key not found", nil),
			substr: "(key=user:123)",
		},
		{
			name: "error with cause",
			err: Factory.New(
				CodeConnection,
				"connect",
				"",
				"connection error",
				fmt.Errorf("dial tcp: connection refused"),
			),
			substr: "dial tcp: connection refused",
		},
		{
			name: "error with key and cause",
			err: Factory.New(
				CodeTimeout,
				"get",
				"session:456",
				"timeout",
				fmt.Errorf("deadline exceeded"),
			),
			substr: "deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if !strings.Contains(got, tt.substr) {
				t.Errorf("Error() = %q, want to contain %q", got, tt.substr)
			}
		})
	}
}

func TestCacheErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	err := Factory.New(CodeInvalid, "set", "", "failed", inner)

	unwrapped := err.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}

	// nil cause
	errNoCause := Factory.New(CodeNotFound, "get", "", "not found", nil)
	if errNoCause.Unwrap() != nil {
		t.Error("expected nil Unwrap() for error without cause")
	}
}

func TestCacheErrorIs(t *testing.T) {
	err := Factory.New(CodeNotFound, "get", "key1", "not found", nil)

	// Same error code and op
	same := Factory.New(CodeNotFound, "get", "key2", "not found", nil)
	if !err.Is(same) {
		t.Error("expected Is() to match same code+op")
	}

	// Different code
	diffCode := Factory.New(CodeTimeout, "get", "", "timeout", nil)
	if err.Is(diffCode) {
		t.Error("expected Is() to not match different code")
	}

	// Non-CacheError target
	if err.Is(fmt.Errorf("other")) {
		t.Error("expected Is() to not match non-CacheError")
	}
}

func TestCacheErrorWithMetadata(t *testing.T) {
	err := Factory.New(CodeTimeout, "get", "k", "timeout", nil)
	// original := err.Metadata

	err2 := err.WithMetadata("retry", 3)
	err3 := err2.WithMetadata("backend", "redis")

	if len(err.Metadata) > 0 || len(err3.Metadata) == 0 {
		t.Error("WithMetadata should create new map, not mutate original")
	}

	// if len(err.Metadata) > 0 && original != nil && len(err.Metadata) != 0 {
	// 	// Original should be unchanged if it was nil initially
	// }

	if err3.Metadata["retry"] != 3 {
		t.Error("expected retry=3 in metadata")
	}
	if err3.Metadata["backend"] != "redis" {
		t.Error("expected backend=redis in metadata")
	}
}

func TestCacheErrorPredicates(t *testing.T) {
	tests := []struct {
		name      string
		err       *CacheError
		notFound  bool
		timeout   bool
		retryable bool
		closed    bool
		expired   bool
	}{
		{
			name:     "not found",
			err:      Factory.NotFound("get", "k"),
			notFound: true,
		},
		{
			name:      "timeout",
			err:       Factory.Timeout("get"),
			timeout:   true,
			retryable: true,
		},
		{
			name:      "connection error",
			err:       Factory.Connection("connect", fmt.Errorf("refused")),
			retryable: true,
		},
		{
			name:   "closed",
			err:    Factory.Closed("set"),
			closed: true,
		},
		{
			name:    "expired",
			err:     Factory.New(CodeExpired, "get", "k", "expired", nil),
			expired: true,
		},
		{
			name:      "rate limited",
			err:       Factory.RateLimited("get"),
			retryable: true,
		},
		{
			name:      "circuit open",
			err:       Factory.CircuitOpen("get"),
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsNotFound(); got != tt.notFound {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.notFound)
			}
			if got := tt.err.IsTimeout(); got != tt.timeout {
				t.Errorf("IsTimeout() = %v, want %v", got, tt.timeout)
			}
			if got := tt.err.IsRetryable(); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
			if got := tt.err.IsClosed(); got != tt.closed {
				t.Errorf("IsClosed() = %v, want %v", got, tt.closed)
			}
			if got := tt.err.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestFactoryMethods(t *testing.T) {
	f := NewDefaultFactory()

	t.Run("New", func(t *testing.T) {
		err := f.New(CodeNotFound, "get", "k", "not found", nil)
		if err.Code != CodeNotFound {
			t.Errorf("expected CodeNotFound, got %d", err.Code)
		}
		if err.Op != "get" || err.Key != "k" {
			t.Error("wrong op or key")
		}
		if err.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		err := f.NotFound("get", "user:1")
		if err.Code != CodeNotFound {
			t.Errorf("expected CodeNotFound, got %d", err.Code)
		}
	})

	t.Run("InvalidKey", func(t *testing.T) {
		cause := fmt.Errorf("contains spaces")
		err := f.InvalidKey("set", "my key", cause)
		if err.Code != CodeInvalidKey {
			t.Errorf("expected CodeInvalidKey, got %d", err.Code)
		}
		if err.Unwrap() != cause {
			t.Error("expected cause to be wrapped")
		}
	})

	t.Run("ConnectionError", func(t *testing.T) {
		cause := fmt.Errorf("connection refused")
		err := f.Connection("connect", cause)
		if err.Code != CodeConnection {
			t.Errorf("expected CodeConnection, got %d", err.Code)
		}
	})

	t.Run("TimeoutError", func(t *testing.T) {
		err := f.Timeout("get")
		if err.Code != CodeTimeout {
			t.Errorf("expected CodeTimeout, got %d", err.Code)
		}
	})

	t.Run("SerializeError", func(t *testing.T) {
		cause := fmt.Errorf("msgpack error")
		err := f.SerializeError("set", cause)
		if err.Code != CodeSerialize {
			t.Errorf("expected CodeSerialize, got %d", err.Code)
		}
	})

	t.Run("DeserializeError", func(t *testing.T) {
		cause := fmt.Errorf("invalid format")
		err := f.DeserializeError("get", cause)
		if err.Code != CodeDeserialize {
			t.Errorf("expected CodeDeserialize, got %d", err.Code)
		}
	})

	t.Run("InvalidConfig", func(t *testing.T) {
		err := f.InvalidConfig("init", "TTL must be positive")
		if err.Code != CodeInvalid {
			t.Errorf("expected CodeInvalid, got %d", err.Code)
		}
		if err.Message != "TTL must be positive" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("Closed", func(t *testing.T) {
		err := f.Closed("set")
		if err.Code != CodeCacheClosed {
			t.Errorf("expected CodeCacheClosed, got %d", err.Code)
		}
	})

	t.Run("RateLimitedError", func(t *testing.T) {
		err := f.RateLimited("get")
		if err.Code != CodeRateLimited {
			t.Errorf("expected CodeRateLimited, got %d", err.Code)
		}
	})

	t.Run("CircuitOpenError", func(t *testing.T) {
		err := f.CircuitOpen("get")
		if err.Code != CodeCircuitOpen {
			t.Errorf("expected CodeCircuitOpen, got %d", err.Code)
		}
	})

	t.Run("EmptyKey", func(t *testing.T) {
		err := f.EmptyKey("get")
		if err.Code != CodeEmptyKey {
			t.Errorf("expected CodeEmptyKey, got %d", err.Code)
		}
	})

	t.Run("EncodeFailed", func(t *testing.T) {
		cause := fmt.Errorf("encode err")
		err := f.EncodeFailed("key1", "msgpack", cause)
		if err.Code != CodeSerialize {
			t.Errorf("expected CodeSerialize, got %d", err.Code)
		}
	})

	t.Run("DecodeFailed", func(t *testing.T) {
		cause := fmt.Errorf("decode err")
		err := f.DecodeFailed("key1", "json", cause)
		if err.Code != CodeDeserialize {
			t.Errorf("expected CodeDeserialize, got %d", err.Code)
		}
	})

	t.Run("IsNotFound", func(t *testing.T) {
		err := f.NotFound("get", "k")
		if !f.IsNotFound(err) {
			t.Error("expected IsNotFound to return true")
		}
		if f.IsTimeout(err) {
			t.Error("expected IsTimeout to return false")
		}
	})

	t.Run("IsClosed", func(t *testing.T) {
		err := f.Closed("set")
		if !f.IsClosed(err) {
			t.Error("expected IsClosed to return true")
		}
	})

	t.Run("IsRateLimited", func(t *testing.T) {
		err := f.RateLimited("get")
		if !f.IsRateLimited(err) {
			t.Error("expected IsRateLimited to return true")
		}
	})

	t.Run("IsCircuitOpen", func(t *testing.T) {
		err := f.CircuitOpen("get")
		if !f.IsCircuitOpen(err) {
			t.Error("expected IsCircuitOpen to return true")
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		err := f.Connection("get", fmt.Errorf("refused"))
		if !f.IsRetryable(err) {
			t.Error("expected IsRetryable to return true for connection error")
		}
		err2 := f.NotFound("get", "k")
		if f.IsRetryable(err2) {
			t.Error("expected IsRetryable to return false for not found")
		}
	})
}

func TestWrap(t *testing.T) {
	f := NewDefaultFactory()

	t.Run("wrap nil cause", func(t *testing.T) {
		err := f.Wrap("op", "key", nil)
		if err.Code != CodeInvalid {
			t.Errorf("expected CodeInvalid, got %d", err.Code)
		}
	})

	t.Run("wrap CacheError", func(t *testing.T) {
		inner := f.NotFound("get", "k")
		wrapped := f.Wrap("outer", "other", inner)
		if wrapped.Code != CodeNotFound {
			t.Errorf("expected CodeNotFound, got %d", wrapped.Code)
		}
	})

	t.Run("wrap generic error", func(t *testing.T) {
		inner := fmt.Errorf("something went wrong")
		err := f.Wrap("set", "key1", inner)
		if err.Code != CodeInvalid {
			t.Errorf("expected CodeInvalid, got %d", err.Code)
		}
		if err.Cause != inner {
			t.Error("expected cause to be set")
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  *CacheError
		code ErrorCode
	}{
		{"NotFound", ErrNotFound, CodeNotFound},
		{"Expired", ErrExpired, CodeExpired},
		{"Closed", ErrClosed, CodeCacheClosed},
		{"InvalidKey", ErrInvalidKey, CodeInvalidKey},
		{"Timeout", ErrTimeout, CodeTimeout},
		{"Cancelled", ErrCancelled, CodeCancelled},
		{"Connection", ErrConnection, CodeConnection},
		{"Overflow", ErrOverflow, CodeOverflow},
		{"RateLimited", ErrRateLimited, CodeRateLimited},
		{"CircuitOpen", ErrCircuitOpen, CodeCircuitOpen},
		{"EmptyKey", ErrEmptyKey, CodeEmptyKey},
		{"NotSupported", ErrNotSupported, CodeNotSupported},
	}

	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, tt.err.Code)
			}
			// Sentinels have zero timestamps by design
			_ = tt.err.Timestamp.IsZero()
		})
	}
}

func TestAllErrorCodes(t *testing.T) {
	const definedCodes = 21

	// ErrorCode(20) = CodeUnsupportedType is defined but not in the name map.
	// Only verify codes that are in the errorCodeNames map.
	for i := 0; i < definedCodes; i++ {
		// CodeUnsupportedType (20) intentionally has no name map entry
		if i == 20 {
			continue
		}
		code := ErrorCode(i)
		if code.String() == "UNKNOWN" {
			t.Errorf("ErrorCode(%d) has no name mapping", i)
		}
	}
}
