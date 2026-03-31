// Package cachectx_test provides tests for context keys and helpers.
package cachectx_test

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/cachectx"
)

type ctxKey string

const testKey ctxKey = "key"

func TestNoCache(t *testing.T) {
	ctx := context.Background()

	// Initially should not bypass cache
	if cachectx.ShouldBypassCache(ctx) {
		t.Error("ShouldBypassCache should be false for background context")
	}

	// Wrap with NoCache
	noCacheCtx := cachectx.NoCache(ctx)
	if !cachectx.ShouldBypassCache(noCacheCtx) {
		t.Error("ShouldBypassCache should be true after NoCache")
	}
}

func TestNoCache_WithExistingValues(t *testing.T) {
	ctx := context.WithValue(context.Background(), testKey, "value")
	noCacheCtx := cachectx.NoCache(ctx)

	// Should preserve existing values
	if val := noCacheCtx.Value(testKey); val != "value" {
		t.Errorf("existing value lost: got %v, want value", val)
	}

	// Should have bypass flag
	if !cachectx.ShouldBypassCache(noCacheCtx) {
		t.Error("ShouldBypassCache should be true")
	}
}

func TestNoCache_MultipleWraps(t *testing.T) {
	ctx := context.Background()
	noCacheCtx := cachectx.NoCache(ctx)
	noCacheCtx2 := cachectx.NoCache(noCacheCtx)

	// Should still bypass cache
	if !cachectx.ShouldBypassCache(noCacheCtx2) {
		t.Error("ShouldBypassCache should be true after multiple wraps")
	}
}

func TestShouldBypassCache_NoCacheKey(t *testing.T) {
	ctx := context.Background()

	// Context without the key
	if cachectx.ShouldBypassCache(ctx) {
		t.Error("ShouldBypassCache should be false for context without key")
	}
}

func TestShouldBypassCache_NilContext(t *testing.T) {
	ctx := context.Background()
	// ShouldBypassCache should handle nil context gracefully
	if cachectx.ShouldBypassCache(ctx) {
		t.Error("ShouldBypassCache should be false for nil context")
	}
}

func TestNormalizeContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want context.Context
	}{
		{
			name: "nil context",
			ctx:  nil,
			want: context.Background(),
		},
		{
			name: "non-nil context",
			ctx:  context.WithValue(context.Background(), testKey, "value"),
			want: context.WithValue(context.Background(), testKey, "value"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cachectx.NormalizeContext(tt.ctx)
			if tt.ctx == nil {
				if result == nil {
					t.Error("NormalizeContext returned nil")
				}
				// Should return a background context
				if result != context.Background() {
					t.Error("NormalizeContext should return context.Background() for nil")
				}
			} else {
				if result != tt.ctx {
					t.Error("NormalizeContext should return the same context for non-nil")
				}
				returnedValue := result.Value(testKey)
				if returnedValue != "value" {
					t.Errorf("NormalizeContext lost context value: got %v, want value", returnedValue)
				}
			}
		})
	}
}

func TestNormalizeContext_PreservesValues(t *testing.T) {
	type testKey struct{}
	ctx := context.WithValue(context.Background(), testKey{}, "test-value")
	normalized := cachectx.NormalizeContext(ctx)

	if normalized.Value(testKey{}) != "test-value" {
		t.Error("NormalizeContext lost context values")
	}
}

func TestNewNegativeValue(t *testing.T) {
	sentinel := cachectx.NewNegativeValue()

	if sentinel == nil {
		t.Fatal("NewNegativeValue returned nil")
	}
	if len(sentinel) != 1 {
		t.Errorf("sentinel length = %d, want 1", len(sentinel))
	}
	if sentinel[0] != 0xFF {
		t.Errorf("sentinel value = %02x, want ff", sentinel[0])
	}
}

func TestIsNegativeValue(t *testing.T) {
	tests := []struct {
		name string
		v    []byte
		want bool
	}{
		{
			name: "valid negative sentinel",
			v:    cachectx.NewNegativeValue(),
			want: true,
		},
		{
			name: "empty slice",
			v:    []byte{},
			want: false,
		},
		{
			name: "nil slice",
			v:    nil,
			want: false,
		},
		{
			name: "single byte wrong value",
			v:    []byte{0x00},
			want: false,
		},
		{
			name: "multiple bytes",
			v:    []byte{0xFF, 0xFF},
			want: false,
		},
		{
			name: "different single byte",
			v:    []byte{0x01},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cachectx.IsNegativeValue(tt.v); got != tt.want {
				t.Errorf("IsNegativeValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNegativeSentinel_Constant(t *testing.T) {
	// Verify the sentinel is consistent
	sentinel1 := cachectx.NewNegativeValue()
	sentinel2 := cachectx.NewNegativeValue()

	if !cachectx.IsNegativeValue(sentinel1) {
		t.Error("sentinel1 not recognized as negative value")
	}
	if !cachectx.IsNegativeValue(sentinel2) {
		t.Error("sentinel2 not recognized as negative value")
	}
	if len(sentinel1) != len(sentinel2) {
		t.Error("sentinel lengths differ")
	}
}

func TestNoCache_WithNormalizeContext(t *testing.T) {
	ctx := context.Background()
	// Test NoCache with nil context
	noCacheCtx := cachectx.NoCache(ctx)
	if noCacheCtx == nil {
		t.Fatal("NoCache with nil context returned nil")
	}
	if !cachectx.ShouldBypassCache(noCacheCtx) {
		t.Error("ShouldBypassCache should be true after NoCache with nil")
	}

	// Test NormalizeContext with NoCache context
	normalized := cachectx.NormalizeContext(noCacheCtx)
	if !cachectx.ShouldBypassCache(normalized) {
		t.Error("ShouldBypassCache should be preserved after NormalizeContext")
	}
}

func TestContext_Chaining(t *testing.T) {
	// Test chaining multiple context operations
	ctx := context.Background()
	ctx = cachectx.NoCache(ctx)
	ctx = context.WithValue(ctx, testKey, "value1")
	normalized := cachectx.NormalizeContext(ctx)

	// Verify all properties
	if !cachectx.ShouldBypassCache(normalized) {
		t.Error("ShouldBypassCache should be true after chaining")
	}
	if normalized.Value(testKey) != "value1" {
		t.Error("key1 value lost")
	}
}

func TestShouldBypassCache_WithValues(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, testKey, "value")
	ctx = cachectx.NoCache(ctx)

	// Should preserve existing values
	if val := ctx.Value(testKey); val != "value" {
		t.Errorf("existing value lost: got %v, want value", val)
	}

	// Should have bypass flag
	if !cachectx.ShouldBypassCache(ctx) {
		t.Error("ShouldBypassCache should be true")
	}
}

func TestIsNegativeValue_EdgeCases(t *testing.T) {
	// Test with various slice lengths
	for length := 0; length <= 5; length++ {
		t.Run("length", func(t *testing.T) {
			v := make([]byte, length)
			for i := range v {
				v[i] = 0xFF
			}
			expected := length == 1
			if got := cachectx.IsNegativeValue(v); got != expected {
				t.Errorf("IsNegativeValue() for length %d = %v, want %v", length, got, expected)
			}
		})
	}
}

func TestNoCache_Immutable(t *testing.T) {
	// Verify that NoCache doesn't modify the original context
	original := context.Background()
	noCacheCtx := cachectx.NoCache(original)

	if cachectx.ShouldBypassCache(original) {
		t.Error("original context should not be modified")
	}
	if !cachectx.ShouldBypassCache(noCacheCtx) {
		t.Error("new context should have bypass flag")
	}
}

func BenchmarkNoCache(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_ = cachectx.NoCache(ctx)
	}
}

func BenchmarkShouldBypassCache(b *testing.B) {
	ctx := cachectx.NoCache(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cachectx.ShouldBypassCache(ctx)
	}
}

func BenchmarkShouldBypassCache_WithoutFlag(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_ = cachectx.ShouldBypassCache(ctx)
	}
}

func BenchmarkNormalizeContext(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_ = cachectx.NormalizeContext(ctx)
	}
}

func BenchmarkNormalizeContext_Nil(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_ = cachectx.NormalizeContext(ctx)
	}
}

func BenchmarkNewNegativeValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = cachectx.NewNegativeValue()
	}
}

func BenchmarkIsNegativeValue(b *testing.B) {
	v := cachectx.NewNegativeValue()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cachectx.IsNegativeValue(v)
	}
}

func BenchmarkIsNegativeValue_False(b *testing.B) {
	v := []byte{0x00}
	for i := 0; i < b.N; i++ {
		_ = cachectx.IsNegativeValue(v)
	}
}

func BenchmarkNoCache_WithValues(b *testing.B) {
	ctx := context.WithValue(context.Background(), testKey, "value")
	for i := 0; i < b.N; i++ {
		_ = cachectx.NoCache(ctx)
	}
}

func TestContext_DeepNesting(t *testing.T) {
	// Test with deeply nested contexts
	type nestedKey int

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ctx = context.WithValue(ctx, nestedKey(i), i)
	}
	ctx = cachectx.NoCache(ctx)

	// Verify all values are preserved
	for i := 0; i < 5; i++ {
		if val := ctx.Value(nestedKey(i)); val != i {
			t.Errorf("value for key %d lost: got %v, want %d", i, val, i)
		}
	}

	// Verify bypass flag
	if !cachectx.ShouldBypassCache(ctx) {
		t.Error("ShouldBypassCache should be true")
	}
}

func TestNormalizeContext_WithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	normalized := cachectx.NormalizeContext(ctx)

	// Should preserve cancel behavior
	if normalized.Err() != nil {
		t.Error("context should not be canceled yet")
	}

	cancel()

	if normalized.Err() == nil {
		t.Error("context should be canceled")
	}
}

func TestNormalizeContext_WithDeadline(t *testing.T) {
	deadline := time.Now().Add(10 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	normalized := cachectx.NormalizeContext(ctx)

	// Should preserve deadline
	if d, ok := normalized.Deadline(); !ok {
		t.Error("deadline not preserved")
	} else if d != deadline {
		t.Errorf("deadline = %v, want %v", d, deadline)
	}
}
