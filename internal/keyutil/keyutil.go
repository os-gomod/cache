// Package keyutil provides key validation, prefixing, and formatting utilities.
package keyutil

import (
	"strings"

	cacheerrors "github.com/os-gomod/cache/errors"
)

const lockSuffix = ":__lock__"

func BuildKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + key
}

func StripPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if len(key) <= len(prefix) {
		return key
	}
	return key[len(prefix):]
}

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

func ValidateKey(op, key string) error {
	if key == "" {
		return cacheerrors.EmptyKey(op)
	}
	return nil
}
