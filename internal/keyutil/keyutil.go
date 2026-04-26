// Package keyutil provides key validation, prefixing, and formatting utilities
// used by all cache backends.
package keyutil

import (
	"strings"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// BuildKey prepends the given prefix to the key. If prefix is empty, the key
// is returned unchanged.
func BuildKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + key
}

// StripPrefix removes the given prefix from the key. If prefix is empty or the
// key doesn't start with the prefix, the key is returned unchanged.
func StripPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if len(key) <= len(prefix) {
		return key
	}
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return key
}

// ValidateKey checks that the key is non-empty. Returns an error created via
// ErrorFactory if validation fails, nil otherwise.
func ValidateKey(op, key string) error {
	if key == "" {
		return cacheerrors.Factory.EmptyKey(op)
	}
	return nil
}

// ValidateKeys checks that all keys are non-empty. Returns the first
// validation error encountered.
func ValidateKeys(op string, keys []string) error {
	for _, key := range keys {
		if key == "" {
			return cacheerrors.Factory.EmptyKey(op)
		}
	}
	return nil
}
