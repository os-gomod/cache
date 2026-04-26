package core

import (
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"empty key", "", true},
		{"valid simple key", "user:123", false},
		{"valid complex key", "session:abc:def:123:token", false},
		{"key with space", "user 123", true},
		{"key with newline", "user\n123", true},
		{"key with carriage return", "user\r123", true},
		{"key with tab", "user\t123", true},
		{"key with null byte", "user\x00123", true},
		{"key at max length", strings.Repeat("a", MaxKeyLength), false},
		{"key exceeds max length", strings.Repeat("a", MaxKeyLength+1), true},
		{"key with single colon", "a:b", false},
		{"key with many colons", "a:b:c:d:e:f:g:h:i:j:k:l:m:n:o:p", true}, // 16 segments = too many
		{"key with 15 colons", strings.Repeat("a:", 15) + "b", true},      // 16 segments
		{"key with 14 colons", strings.Repeat("a:", 14) + "b", false},     // 15 segments
		{"unicode key", "user:日本語", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{"both parts", "user", "123", "user:123"},
		{"empty prefix", "", "123", "123"},
		{"empty key", "user", "", "user"},
		{"both empty", "", "", ""},
		{"nested prefix", "cache:session", "token", "cache:session:token"},
		{"numeric key", "counter", "hits", "counter:hits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildKey(tt.prefix, tt.key)
			if got != tt.expected {
				t.Errorf("BuildKey(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.expected)
			}
		})
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		fullKey  string
		expected string
	}{
		{"matching prefix", "user", "user:123", "123"},
		{"non-matching prefix", "user", "session:123", "session:123"},
		{"empty prefix", "", "user:123", "user:123"},
		{"prefix without colon", "user", "user123", "user123"},
		{"nested key", "cache", "cache:session:token", "session:token"},
		{"exact prefix match", "user", "user:", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripPrefix(tt.prefix, tt.fullKey)
			if got != tt.expected {
				t.Errorf(
					"StripPrefix(%q, %q) = %q, want %q",
					tt.prefix,
					tt.fullKey,
					got,
					tt.expected,
				)
			}
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already clean", "user:123", "user:123"},
		{"with spaces", " user:123 ", "user:123"},
		{"with multiple colons", "user::123", "user:123"},
		{"with triple colons", "user:::123", "user:123"},
		{"uppercase", "User:ABC", "user:abc"},
		{"leading colon", ":user:123", "user:123"},
		{"trailing colon", "user:123:", "user:123"},
		{"with newlines", "user\n:123", "user:123"},
		{"with tabs", "user\t:123", "user:123"},
		{"mixed bad chars", " User :: 123 \n", "user:123"},
		{"only spaces", "   ", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeKey(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStampedeLockKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{"normal", "user", "123", "lock:user:123"},
		{"empty prefix", "", "123", "lock:123"},
		{"empty key", "user", "", "lock:user"},
		{"nested", "session", "abc:token", "lock:session:abc:token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StampedeLockKey(tt.prefix, tt.key)
			if got != tt.expected {
				t.Errorf(
					"StampedeLockKey(%q, %q) = %q, want %q",
					tt.prefix,
					tt.key,
					got,
					tt.expected,
				)
			}
		})
	}
}

func TestHasControlChars(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"clean", "user:123", false},
		{"with null", "user\x00123", true},
		{"with bell", "user\a123", true},
		{"with tab", "user\t123", false}, // tab is excluded
		{"with escape", "user\x1b123", true},
		{"with space", "user 123", false}, // space is not a control char
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasControlChars(tt.key); got != tt.want {
				t.Errorf("HasControlChars(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsPrintableASCII(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"clean", "user:123", true},
		{"with space", "user 123", true},
		{"with null", "user\x00123", false},
		{"with escape", "user\x1b123", false},
		{"unicode", "user:日本語", false},
		{"with tilde", "user~123", true},
		{"with delete", "user\x7f123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPrintableASCII(tt.key); got != tt.want {
				t.Errorf("IsPrintableASCII(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestExecutionResult(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		r := Success([]byte("hello"), 5)
		if !r.OK() {
			t.Error("expected OK()")
		}
		if !r.Hit {
			t.Error("expected Hit=true")
		}
		if string(r.Value) != "hello" {
			t.Errorf("expected value=hello, got %v", r.Value)
		}
		if r.Bytes() != 5 {
			t.Errorf("expected Bytes()=5, got %d", r.Bytes())
		}
	})

	t.Run("Failure", func(t *testing.T) {
		r := Failure[[]byte](ErrTest, 10)
		if r.OK() {
			t.Error("expected not OK()")
		}
		if r.Hit {
			t.Error("expected Hit=false")
		}
		if r.Err != ErrTest {
			t.Errorf("expected ErrTest, got %v", r.Err)
		}
		if r.Bytes() != 0 {
			t.Errorf("expected Bytes()=0, got %d", r.Bytes())
		}
	})

	t.Run("Success with Size override", func(t *testing.T) {
		r := Success("val", 1)
		r.Size = 100
		if r.Bytes() != 100 {
			t.Errorf("expected Bytes()=100, got %d", r.Bytes())
		}
	})
}

// ErrTest is a test sentinel error.
var ErrTest = ErrTestType("test error")

// ErrTestType is a test error type.
type ErrTestType string

func (e ErrTestType) Error() string { return string(e) }
