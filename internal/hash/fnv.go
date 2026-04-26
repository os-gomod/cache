// Package hash provides FNV-1a hashing and power-of-two integer utilities
// used for shard selection and capacity calculations across the cache module.
package hash

const (
	// MaxShards is the maximum number of allowed cache shards.
	maxShards = 4096

	// DefaultShards is the default shard count when none is specified.
	defaultShards = 32

	// Offset32 is the FNV-1a offset basis for 32-bit hashes.
	offset32 uint32 = 2166136261

	// Prime32 is the FNV-1a prime for 32-bit hashes.
	prime32 uint32 = 16777619
)

// FNV1a32 computes the 32-bit FNV-1a hash of the given string.
// This is the standard FNV-1a algorithm suitable for hash table lookups
// and shard selection.
func FNV1a32(s string) uint32 {
	h := offset32
	for i := range len(s) {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

// NormalizeShards clamps n to [1, 4096] and rounds up to the next power of
// two. If n is already a power of two, it is returned unchanged.
func NormalizeShards(n int) int {
	if n <= 0 {
		return defaultShards
	}
	if n > maxShards {
		return maxShards
	}
	if IsPowerOfTwo(n) {
		return n
	}
	return NextPowerOfTwo(n)
}

// BitmaskForPowerOfTwo returns a bitmask for the given power-of-two value.
// For example, BitmaskForPowerOfTwo(64) returns 0x3F (63).
// The result is equivalent to (n - 1) when n is a power of two.
func BitmaskForPowerOfTwo(n int) uint32 {
	return uint32(n - 1)
}

// IsPowerOfTwo reports whether n is a power of two.
// Returns false for n <= 0.
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// NextPowerOfTwo returns the smallest power of two that is >= n.
// For n <= 0 it returns 1.
func NextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
