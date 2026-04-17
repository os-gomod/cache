package stringutil

import "testing"

func TestTruncateKey(t *testing.T) {
	tests := []struct {
		key    string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this_is_longer", 10, "this_is..."},
		{"a", 3, "a"},
		{"abcd", 3, "abc"},
		{"abcdef", 1, "a"},
		{"", 10, ""},
	}
	for _, tt := range tests {
		got := TruncateKey(tt.key, tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateKey(%q, %d) = %q, want %q", tt.key, tt.maxLen, got, tt.want)
		}
	}
}

func TestIsReadOp(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"get", true},
		{"get_multi", true},
		{"get_or_set", true},
		{"getset", true},
		{"exists", true},
		{"set", false},
		{"delete", false},
		{"ping", false},
		{"", false},
		{"get_or_set_with_lock", false},
	}
	for _, tt := range tests {
		got := IsReadOp(tt.op)
		if got != tt.want {
			t.Errorf("IsReadOp(%q) = %v, want %v", tt.op, got, tt.want)
		}
	}
}
