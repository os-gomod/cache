package observability

import (
	"context"
	"testing"
)

func TestNopInterceptor_Before_After(t *testing.T) {
	ni := NopInterceptor{}
	ctx := ni.Before(context.Background(), Op{Backend: "test", Name: "get"})
	ni.After(ctx, Op{}, Result{})
}

func TestNewChain_Empty(t *testing.T) {
	c := NewChain()
	if c == nil {
		t.Fatal("NewChain() returned nil")
	}
	if !c.IsEmpty() {
		t.Error("empty chain should be empty")
	}
}

func TestNewChain_WithInterceptors(t *testing.T) {
	var beforeOrder, afterOrder []string
	a := &orderInterceptor{name: "a", beforeLog: &beforeOrder, afterLog: &afterOrder}
	b := &orderInterceptor{name: "b", beforeLog: &beforeOrder, afterLog: &afterOrder}
	c := NewChain(a, b)
	if c.IsEmpty() {
		t.Fatal("chain should not be empty")
	}
	ctx := c.Before(context.Background(), Op{Backend: "test", Name: "get"})
	c.After(ctx, Op{Backend: "test", Name: "get"}, Result{})
	if len(beforeOrder) != 2 || beforeOrder[0] != "a" || beforeOrder[1] != "b" {
		t.Errorf("Before order = %v, want [a b]", beforeOrder)
	}
	if len(afterOrder) != 2 || afterOrder[0] != "b" || afterOrder[1] != "a" {
		t.Errorf("After order = %v, want [b a]", afterOrder)
	}
}

func TestChain_Append(t *testing.T) {
	var log []string
	a := &orderInterceptor{name: "a", beforeLog: &log, afterLog: &log}
	b := &orderInterceptor{name: "b", beforeLog: &log, afterLog: &log}
	c1 := NewChain(a)
	c2 := c1.Append(b)
	if c2.IsEmpty() {
		t.Fatal("appended chain should not be empty")
	}
	ctx := c2.Before(context.Background(), Op{})
	c2.After(ctx, Op{}, Result{})
	if len(log) != 4 {
		t.Errorf("expected 4 calls, got %d", len(log))
	}
}

func TestNopChain(t *testing.T) {
	c := NopChain()
	if c == nil {
		t.Fatal("NopChain() returned nil")
	}
	if !c.IsEmpty() {
		t.Error("NopChain should be empty")
	}
}

type orderInterceptor struct {
	name      string
	beforeLog *[]string
	afterLog  *[]string
}

func (o *orderInterceptor) Before(ctx context.Context, _ Op) context.Context {
	*o.beforeLog = append(*o.beforeLog, o.name)
	return ctx
}

func (o *orderInterceptor) After(_ context.Context, _ Op, _ Result) {
	*o.afterLog = append(*o.afterLog, o.name)
}
