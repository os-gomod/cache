package keyutil

import "testing"

func TestBuildKey(t *testing.T) {
	tests := []struct {
		prefix, key string
		want        string
	}{
		{"", "user:1", "user:1"},
		{"cache:", "user:1", "cache:user:1"},
		{"ns", "key", "nskey"},
	}
	for _, tt := range tests {
		got := BuildKey(tt.prefix, tt.key)
		if got != tt.want {
			t.Errorf("BuildKey(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
		}
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		prefix, full string
		want         string
	}{
		{"", "user:1", "user:1"},
		{"cache:", "cache:user:1", "user:1"},
		{"ns", "ns:key", ":key"},
	}
	for _, tt := range tests {
		got := StripPrefix(tt.prefix, tt.full)
		if got != tt.want {
			t.Errorf("StripPrefix(%q, %q) = %q, want %q", tt.prefix, tt.full, got, tt.want)
		}
	}
}

func TestStampedeLockKey(t *testing.T) {
	got := StampedeLockKey("cache:", "user:1")
	if got == "" {
		t.Error("StampedeLockKey returned empty")
	}
	if got == "user:1" {
		t.Error("StampedeLockKey should differ from the original key")
	}
}
