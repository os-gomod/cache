// Package hash provides shared hashing and integer utilities used across
// the cache library, primarily for shard selection and power-of-two calculations.
package hash

const (
	// maxShards is the maximum number of allowed cache shards.
	maxShards = 4096

	// defaultShards is the default shard count when none is specified.
	defaultShards = 64

	// offset32 is the FNV-1a offset basis for 32-bit hashes.
	offset32 uint32 = 2166136261

	// prime32 is the FNV-1a prime for 32-bit hashes.
	prime32 uint32 = 16777619
)

// FNV1a32 computes the 32-bit FNV-1a hash of the given string.
// This is the standard FNV-1a algorithm suitable for hash table lookups.
func FNV1a32(s string) uint32 {
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

// NormalizeShards clamps n to [1, 4096] and rounds up to the next power of two.
// If n is already a power of two it is returned unchanged.
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

// BitmaskForPowerOfTwo returns a bitmask with the lower bits set for the given
// power-of-two value. For example, BitmaskForPowerOfTwo(64) returns 0x3F.
func BitmaskForPowerOfTwo(n int) uint32 {
	var mask uint32
	for bit := 1; bit < n; bit <<= 1 {
		mask = (mask << 1) | 1
	}
	return mask
}
