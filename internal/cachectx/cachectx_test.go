package cachectx

import (
	"context"
	"testing"
)

func TestShouldBypassCache_Default(t *testing.T) {
	ctx := context.Background()
	if ShouldBypassCache(ctx) {
		t.Error("expected default context to not bypass cache")
	}
}

func TestShouldBypassCache_NoCache(t *testing.T) {
	ctx := context.Background()
	ctx = NoCache(ctx)
	if !ShouldBypassCache(ctx) {
		t.Error("expected NoCache context to bypass cache")
	}
}

func TestNormalizeContext_Nil(t *testing.T) {
	got := NormalizeContext(nil)
	if got == nil {
		t.Error("expected non-nil context from nil input")
	}
}

func TestNormalizeContext_NonNil(t *testing.T) {
	ctx := context.Background()
	got := NormalizeContext(ctx)
	if got != ctx {
		t.Error("expected same context returned for non-nil input")
	}
}

func TestNewNegativeValue(t *testing.T) {
	v := NewNegativeValue()
	if len(v) != 1 || v[0] != 0xFF {
		t.Errorf("expected [0xFF], got %v", v)
	}
}

func TestIsNegativeValue_True(t *testing.T) {
	v := NewNegativeValue()
	if !IsNegativeValue(v) {
		t.Error("expected true for negative sentinel value")
	}
}

func TestIsNegativeValue_False(t *testing.T) {
	if IsNegativeValue([]byte("hello")) {
		t.Error("expected false for regular value")
	}
	if IsNegativeValue([]byte{}) {
		t.Error("expected false for empty value")
	}
	if IsNegativeValue([]byte{0xFE}) {
		t.Error("expected false for different byte")
	}
}

func TestNoCache_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), "other-key", "other-val")
	ctx = NoCache(ctx)
	if !ShouldBypassCache(ctx) {
		t.Error("expected bypass after NoCache on context with existing values")
	}
	// Original value should still be accessible.
	if ctx.Value("other-key") != "other-val" {
		t.Error("expected original context value to be preserved")
	}
}
