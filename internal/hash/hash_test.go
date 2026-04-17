package hash

import "testing"

func TestFNV1a32_Consistent(t *testing.T) {
	// FNV-1a is deterministic - same input must always produce same hash
	tests := []struct {
		input string
		want  uint32
	}{
		{"", 0x811c9dc5},
		{"a", 0xe40c292c},
		{"hello", 0x4f9f2cab},
		{"hello world", 0xd58b3fa7},
		{"cache:key:123", 0x1f89fffe},
	}
	for _, tt := range tests {
		got := FNV1a32(tt.input)
		if got != tt.want {
			t.Errorf("FNV1a32(%q) = 0x%08x, want 0x%08x", tt.input, got, tt.want)
		}
	}
}

func TestFNV1a32_Distributes(t *testing.T) {
	// Check that different keys produce different hashes (no trivial collisions)
	seen := make(map[uint32]bool, 1000)
	for i := 0; i < 1000; i++ {
		h := FNV1a32(string(rune('a' + i)))
		if seen[h] {
			t.Errorf("collision detected for key %d", i)
		}
		seen[h] = true
	}
}

func TestNormalizeShards(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 64},
		{-1, 64},
		{1, 1},
		{32, 32},
		{33, 64},
		{100, 128},
		{1024, 1024},
		{4096, 4096},
		{4097, 4096},
		{5000, 4096},
	}
	for _, tt := range tests {
		got := NormalizeShards(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeShards(%d) = %d, want %d", tt.input, got, tt.want)
		}
		// Verify result is always a power of two
		if !IsPowerOfTwo(got) {
			t.Errorf("NormalizeShards(%d) = %d, not a power of two", tt.input, got)
		}
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, false},
		{-1, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{16, true},
		{1024, true},
		{1023, false},
	}
	for _, tt := range tests {
		got := IsPowerOfTwo(tt.n)
		if got != tt.want {
			t.Errorf("IsPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{-5, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{9, 16},
		{16, 16},
		{17, 32},
		{100, 128},
		{1000, 1024},
	}
	for _, tt := range tests {
		got := NextPowerOfTwo(tt.input)
		if got != tt.want {
			t.Errorf("NextPowerOfTwo(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestBitmaskForPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want uint32
	}{
		{1, 0x0},
		{2, 0x1},
		{4, 0x3},
		{8, 0x7},
		{16, 0xF},
		{64, 0x3F},
		{256, 0xFF},
	}
	for _, tt := range tests {
		got := BitmaskForPowerOfTwo(tt.n)
		if got != tt.want {
			t.Errorf("BitmaskForPowerOfTwo(%d) = 0x%x, want 0x%x", tt.n, got, tt.want)
		}
	}
}
