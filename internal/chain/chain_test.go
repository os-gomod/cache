package chain

import (
	"context"
	"testing"

	"github.com/os-gomod/cache/observability"
)

func TestBuildChain_Nil(t *testing.T) {
	c := BuildChain(nil)
	if c == nil {
		t.Fatal("BuildChain(nil) returned nil")
	}
	if !c.IsEmpty() {
		t.Error("BuildChain(nil) should return empty chain")
	}
}

func TestBuildChain_EmptySlice(t *testing.T) {
	c := BuildChain([]observability.Interceptor{})
	if !c.IsEmpty() {
		t.Error("BuildChain([]) should return empty chain")
	}
}

func TestBuildChain_WithInterceptors(t *testing.T) {
	called := 0
	ic := &callCountingInterceptor{fn: func() { called++ }}
	c := BuildChain([]observability.Interceptor{ic})
	if c.IsEmpty() {
		t.Fatal("BuildChain with interceptor should not be empty")
	}
	c.After(context.Background(), observability.Op{}, observability.Result{})
	if called != 1 {
		t.Errorf("expected 1 After call, got %d", called)
	}
}

func TestBuildChain_NonInterceptorSlice(t *testing.T) {
	c := BuildChain("not a slice")
	if c == nil {
		t.Fatal("BuildChain with wrong type should return NopChain, not nil")
	}
	if !c.IsEmpty() {
		t.Error("BuildChain with wrong type should return empty chain")
	}
}

func TestBuildChain_NonInterceptorAnyValue(t *testing.T) {
	c := BuildChain(42)
	if c == nil {
		t.Fatal("BuildChain with int should return NopChain, not nil")
	}
	if !c.IsEmpty() {
		t.Error("BuildChain with int should return empty chain")
	}
}

func TestSetInterceptors_Empty(t *testing.T) {
	existing := observability.NewChain(&callCountingInterceptor{})
	result := SetInterceptors(existing, nil)
	if result == nil {
		t.Fatal("SetInterceptors returned nil")
	}
	if !result.IsEmpty() {
		t.Error("SetInterceptors with nil should return NopChain")
	}
}

func TestSetInterceptors_EmptySlice(t *testing.T) {
	existing := observability.NewChain(&callCountingInterceptor{})
	result := SetInterceptors(existing, []observability.Interceptor{})
	if result == nil {
		t.Fatal("SetInterceptors returned nil")
	}
	if !result.IsEmpty() {
		t.Error("SetInterceptors with empty slice should return NopChain")
	}
}

func TestSetInterceptors_WithInterceptors(t *testing.T) {
	result := SetInterceptors(nil, []observability.Interceptor{
		&callCountingInterceptor{},
	})
	if result == nil {
		t.Fatal("SetInterceptors returned nil")
	}
	if result.IsEmpty() {
		t.Error("SetInterceptors with interceptors should not be empty")
	}
}

func TestSetInterceptors_NilExisting(t *testing.T) {
	result := SetInterceptors(nil, []observability.Interceptor{
		&callCountingInterceptor{},
	})
	if result == nil {
		t.Fatal("SetInterceptors with nil existing should still return a chain")
	}
}

// callCountingInterceptor is a test interceptor that counts Before/After calls.
type callCountingInterceptor struct {
	fn func()
}

func (c *callCountingInterceptor) Before(ctx context.Context, op observability.Op) context.Context {
	if c.fn != nil {
		c.fn()
	}
	return ctx
}

func (c *callCountingInterceptor) After(_ context.Context, _ observability.Op, _ observability.Result) {
	if c.fn != nil {
		c.fn()
	}
}
