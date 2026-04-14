// Package keyutil provides canonical key-building utilities for all cache
// backends. Every key-prefix operation in the codebase must use these
// functions to ensure consistent behavior and zero duplication.
package keyutil

import (
	"strings"

	_errors "github.com/os-gomod/cache/errors"
)

const lockSuffix = ":__lock__"

// BuildKey prepends prefix to key when prefix is non-empty.
// When prefix is empty the original key is returned directly,
// avoiding any allocation on the hot path.
func BuildKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + key
}

// StripPrefix removes prefix from key when prefix is non-empty
// and key is longer than prefix. Otherwise key is returned unchanged.
func StripPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if len(key) <= len(prefix) {
		return key
	}
	return key[len(prefix):]
}

// StampedeLockKey returns the distributed-lock key for a given cache key.
// The result is BuildKey(prefix, key) + ":__lock__".
func StampedeLockKey(prefix, key string) string {
	if prefix == "" {
		return key + lockSuffix
	}
	var b strings.Builder
	b.Grow(len(prefix) + len(key) + len(lockSuffix))
	b.WriteString(prefix)
	b.WriteString(key)
	b.WriteString(lockSuffix)
	return b.String()
}

// ValidateKey returns an EmptyKey error when key is blank.
// Every public cache method should call this before any other work.
func ValidateKey(op, key string) error {
	if key == "" {
		return _errors.EmptyKey(op)
	}
	return nil
}
