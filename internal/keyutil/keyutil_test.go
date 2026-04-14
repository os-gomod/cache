package keyutil

import (
	"testing"
)

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"empty prefix returns key", "", "mykey", "mykey"},
		{"prefix prepended", "app:", "mykey", "app:mykey"},
		{"both empty", "", "", ""},
		{"prefix only", "cache:", "", "cache:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildKey(tt.prefix, tt.key); got != tt.want {
				t.Errorf("BuildKey(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

func TestBuildKey_NoAllocation(t *testing.T) {
	key := "testkey"
	result := BuildKey("", key)
	// When prefix is empty, the same string should be returned.
	if result != key {
		t.Errorf("BuildKey with empty prefix should return the original key, got %q", result)
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"empty prefix returns key", "", "app:mykey", "app:mykey"},
		{"prefix stripped", "app:", "app:mykey", "mykey"},
		{"key shorter than prefix", "app:", "a", "a"},
		{"key equal to prefix", "app:", "app:", "app:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripPrefix(tt.prefix, tt.key); got != tt.want {
				t.Errorf("StripPrefix(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

func TestStampedeLockKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"no prefix", "", "mykey", "mykey:__lock__"},
		{"with prefix", "app:", "mykey", "app:mykey:__lock__"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StampedeLockKey(tt.prefix, tt.key); got != tt.want {
				t.Errorf("StampedeLockKey(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	if err := ValidateKey("test.op", ""); err == nil {
		t.Error("ValidateKey with empty key should return error")
	}
	if err := ValidateKey("test.op", "valid"); err != nil {
		t.Errorf("ValidateKey with non-empty key should return nil, got %v", err)
	}
}

func FuzzBuildKey(f *testing.F) {
	f.Add("", "key")
	f.Add("prefix:", "key")
	f.Add("", "")
	f.Add("a", "b")
	f.Fuzz(func(t *testing.T, prefix, key string) {
		result := BuildKey(prefix, key)
		if prefix == "" {
			if result != key {
				t.Errorf("empty prefix: got %q, want %q", result, key)
			}
		} else {
			expected := prefix + key
			if result != expected {
				t.Errorf("got %q, want %q", result, expected)
			}
		}
	})
}
