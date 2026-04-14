package cachectx

import "context"

type noCacheKey struct{}

func NoCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, noCacheKey{}, true)
}

func ShouldBypassCache(ctx context.Context) bool {
	b, _ := ctx.Value(noCacheKey{}).(bool)
	return b
}

func NormalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

const negativeSentinel = byte(0xFF)

func NewNegativeValue() []byte { return []byte{negativeSentinel} }
func IsNegativeValue(v []byte) bool {
	return len(v) == 1 && v[0] == negativeSentinel
}
