// Package core provides fundamental types and utilities for cache key management
// and result handling. These are the building blocks used by higher-level
// runtime, backend, and adapter components.
package core

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// MaxKeyLength is the maximum allowed length for a cache key.
const MaxKeyLength = 256

// MaxKeySegments is the maximum allowed number of colon-separated segments in
// a key.
const MaxKeySegments = 15

// forbiddenChars contains characters that are not allowed in cache keys.
var forbiddenChars = map[rune]bool{
	' ':    true, // space
	'\n':   true, // newline
	'\r':   true, // carriage return
	'\t':   true, // tab
	'\x00': true, // null
}

// ValidateKey checks that a cache key meets all validation requirements:
//   - non-empty
//   - does not exceed MaxKeyLength
//   - does not contain forbidden characters (spaces, control chars)
//   - does not have excessive colon-separated segments
//
// Returns an error describing the first validation failure encountered.
func ValidateKey(key string) error {
	if key == "" {
		return errors.New("cache key must not be empty")
	}
	if len(key) > MaxKeyLength {
		return fmt.Errorf("cache key length %d exceeds maximum of %d", len(key), MaxKeyLength)
	}
	for _, ch := range key {
		if forbiddenChars[ch] {
			return fmt.Errorf("cache key contains forbidden character: %q", ch)
		}
	}
	segments := strings.Count(key, ":") + 1
	if segments > MaxKeySegments {
		return fmt.Errorf("cache key has too many segments (%d, max %d)",
			segments, MaxKeySegments)
	}
	return nil
}

// BuildKey constructs a namespaced cache key by joining the prefix and key
// with a colon separator. If either prefix or key is empty, the non-empty
// value is returned directly. If both are empty, an empty string is returned.
//
// Example:
//
//	BuildKey("user", "123")  => "user:123"
//	BuildKey("", "123")      => "123"
//	BuildKey("user", "")     => "user"
func BuildKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + ":" + key
}

// StripPrefix removes the given prefix (and colon separator) from a full key.
// If the full key does not start with the prefix followed by a colon, the
// original fullKey is returned unchanged.
//
// Example:
//
//	StripPrefix("user", "user:123") => "123"
//	StripPrefix("user", "other:123") => "other:123"
func StripPrefix(prefix, fullKey string) string {
	if prefix == "" {
		return fullKey
	}
	prefixWithSep := prefix + ":"
	if strings.HasPrefix(fullKey, prefixWithSep) {
		return fullKey[len(prefixWithSep):]
	}
	return fullKey
}

// SanitizeKey cleans a cache key by:
//   - trimming leading and trailing whitespace
//   - collapsing multiple consecutive colons into one
//   - removing any forbidden characters
//   - lowercasing the result
//
// Returns the sanitized key. If the key becomes empty after sanitization,
// it is returned as-is (the caller should validate separately).
func SanitizeKey(key string) string {
	// Trim whitespace
	key = strings.TrimSpace(key)

	// Remove forbidden characters
	var b strings.Builder
	b.Grow(len(key))
	for _, ch := range key {
		if !forbiddenChars[ch] {
			b.WriteRune(ch)
		}
	}
	key = b.String()

	// Collapse multiple colons
	for strings.Contains(key, "::") {
		key = strings.ReplaceAll(key, "::", ":")
	}

	// Trim leading/trailing colons
	key = strings.Trim(key, ":")

	// Lowercase
	key = strings.ToLower(key)

	return key
}

// StampedeLockKey builds a cache stampede protection (mutex) key from the
// given prefix and key. The resulting key is prefixed with "lock:" to
// namespace it separately from data keys.
//
// Example:
//
//	StampedeLockKey("user", "123") => "lock:user:123"
func StampedeLockKey(prefix, key string) string {
	return "lock:" + BuildKey(prefix, key)
}

// HasControlChars returns true if the key contains any ASCII control characters
// (characters below 0x20, excluding tab).
func HasControlChars(key string) bool {
	for _, ch := range key {
		if ch < 0x20 && ch != '\t' {
			return true
		}
	}
	return false
}

// IsPrintableASCII returns true if the key consists entirely of printable ASCII
// characters (0x20-0x7E).
func IsPrintableASCII(key string) bool {
	for _, ch := range key {
		if ch > unicode.MaxASCII || ch < 0x20 || ch == 0x7f {
			return false
		}
	}
	return true
}
