package builder

import (
	"strings"
	"unicode"

	_errors "github.com/os-gomod/cache/errors"
)

const (
	maxKeyLen    = 512
	maxPrefixLen = 256
)

// KeyBuilder produces hierarchical, colon-delimited cache keys from a prefix
// and zero or more path segments.  Every method returns a new KeyBuilder so
// the receiver is safe to reuse and fork.
type KeyBuilder struct {
	prefix string
	parts  []string
}

// NewKey returns a KeyBuilder rooted at prefix.
// Returns an error when prefix is empty, exceeds maxPrefixLen, or contains
// control characters or whitespace.
func NewKey(prefix string) (*KeyBuilder, error) {
	if err := validateSegment("prefix", prefix, maxPrefixLen); err != nil {
		return nil, err
	}
	return &KeyBuilder{
		prefix: prefix,
		parts:  make([]string, 0, 4),
	}, nil
}

// MustNewKey is like NewKey but panics on validation error.
// Use only in program initialisation or tests.
func MustNewKey(prefix string) *KeyBuilder {
	kb, err := NewKey(prefix)
	if err != nil {
		panic("cache: MustNewKey: " + err.Error())
	}
	return kb
}

// Add appends a path segment and returns a new KeyBuilder.
// Returns an error when part is empty, would push the total key length over
// maxKeyLen, or contains control characters or whitespace.
func (b *KeyBuilder) Add(part string) (*KeyBuilder, error) {
	if err := validateSegment("segment", part, maxKeyLen); err != nil {
		return nil, err
	}
	// Pre-flight total-length check.
	total := len(b.prefix)
	for _, p := range b.parts {
		total += 1 + len(p) // ":" + segment
	}
	total += 1 + len(part) // ":" + new segment
	if total > maxKeyLen {
		return nil, _errors.InvalidConfig("key_builder.add",
			"resulting key would exceed maximum length")
	}

	newParts := make([]string, len(b.parts)+1)
	copy(newParts, b.parts)
	newParts[len(b.parts)] = part
	return &KeyBuilder{prefix: b.prefix, parts: newParts}, nil
}

// MustAdd is like Add but panics on validation error.
func (b *KeyBuilder) MustAdd(part string) *KeyBuilder {
	nb, err := b.Add(part)
	if err != nil {
		panic("cache: KeyBuilder.MustAdd: " + err.Error())
	}
	return nb
}

// Build assembles all segments into a single colon-delimited key string.
func (b *KeyBuilder) Build() string {
	if b == nil {
		return ""
	}
	total := len(b.prefix)
	for _, p := range b.parts {
		total += 1 + len(p)
	}
	var sb strings.Builder
	sb.Grow(total)
	sb.WriteString(b.prefix)
	for _, p := range b.parts {
		sb.WriteByte(':')
		sb.WriteString(p)
	}
	return sb.String()
}

// TryAdd is an alias for Add (error-returning variant).
func (b *KeyBuilder) TryAdd(part string) (*KeyBuilder, error) { return b.Add(part) }

// TryBuild assembles the key and returns an error when the resulting length
// exceeds maxKeyLen.
func (b *KeyBuilder) TryBuild() (string, error) {
	if b == nil {
		return "", _errors.InvalidConfig("key_builder.build", "builder is nil")
	}
	total := len(b.prefix)
	for _, p := range b.parts {
		total += 1 + len(p)
	}
	if total > maxKeyLen {
		return "", _errors.InvalidConfig("key_builder.build",
			"resulting key exceeds maximum length")
	}
	return b.Build(), nil
}

// Prefix returns the root prefix of this builder.
func (b *KeyBuilder) Prefix() string {
	if b == nil {
		return ""
	}
	return b.prefix
}

// Depth returns the number of segments added beyond the prefix.
func (b *KeyBuilder) Depth() int {
	if b == nil {
		return 0
	}
	return len(b.parts)
}

// validateSegment rejects empty strings, strings over maxLen, and strings
// containing control characters or whitespace.
func validateSegment(field, s string, maxLen int) error {
	if s == "" {
		return _errors.InvalidConfig("key_builder",
			field+" must not be empty")
	}
	if len(s) > maxLen {
		return _errors.InvalidConfig("key_builder",
			field+" exceeds maximum length")
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return _errors.InvalidConfig("key_builder",
				field+" must not contain control characters or whitespace")
		}
	}
	return nil
}
