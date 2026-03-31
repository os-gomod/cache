// Package cachectx holds context keys and helpers shared across cache packages.
package cachectx

import "context"

type noCacheKey struct{}

// NoCache wraps ctx so that all cache backends bypass their stores and treat
// every operation as a miss.
func NoCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, noCacheKey{}, true)
}

// ShouldBypassCache reports whether the context signals a cache bypass.
func ShouldBypassCache(ctx context.Context) bool {
	b, _ := ctx.Value(noCacheKey{}).(bool)
	return b
}

// NormalizeContext returns ctx if non-nil, otherwise context.Background().
func NormalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// negativeSentinel is the single-byte marker stored in L1 for negative cache entries.
const negativeSentinel = byte(0xFF)

// NewNegativeValue returns the sentinel value used for negative caching.
func NewNegativeValue() []byte { return []byte{negativeSentinel} }

// IsNegativeValue reports whether v is a negative-cache sentinel.
func IsNegativeValue(v []byte) bool {
	return len(v) == 1 && v[0] == negativeSentinel
}
