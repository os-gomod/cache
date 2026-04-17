package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestNewOTelInterceptor(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	o := NewOTelInterceptor(tracer)
	if o == nil {
		t.Fatal("NewOTelInterceptor returned nil")
	}
}

func TestNewOTelInterceptor_NilTracer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil tracer")
		}
	}()
	NewOTelInterceptor(nil)
}

func TestOTelInterceptor_Before(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	o := NewOTelInterceptor(tracer)

	tests := []struct {
		name string
		op   Op
	}{
		{"basic", Op{Backend: "memory", Name: "get", Key: "user:1"}},
		{"with key count", Op{Backend: "memory", Name: "get_multi", KeyCount: 5}},
		{"no key", Op{Backend: "memory", Name: "set"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := o.Before(context.Background(), tt.op)
			if ctx == nil {
				t.Error("Before should return non-nil context")
			}
		})
	}
}

func TestOTelInterceptor_After(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	o := NewOTelInterceptor(tracer)

	t.Run("hit", func(t *testing.T) {
		ctx := o.Before(context.Background(), Op{Backend: "memory", Name: "get"})
		o.After(ctx, Op{Backend: "memory", Name: "get"}, Result{Hit: true})
	})

	t.Run("miss", func(t *testing.T) {
		ctx := o.Before(context.Background(), Op{Backend: "memory", Name: "get"})
		o.After(ctx, Op{Backend: "memory", Name: "get"}, Result{Hit: false})
	})

	t.Run("error", func(t *testing.T) {
		ctx := o.Before(context.Background(), Op{Backend: "memory", Name: "set"})
		o.After(ctx, Op{Backend: "memory", Name: "set"}, Result{Err: context.DeadlineExceeded})
	})

	t.Run("write op no hit attr", func(t *testing.T) {
		ctx := o.Before(context.Background(), Op{Backend: "memory", Name: "set"})
		o.After(ctx, Op{Backend: "memory", Name: "set"}, Result{Hit: false})
	})
}

func TestOTelInterceptor_After_NoSpan(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	o := NewOTelInterceptor(tracer)

	// Call After without calling Before — no span in context
	o.After(context.Background(), Op{Backend: "memory", Name: "get"}, Result{Hit: true})
	// Should not panic
}

func TestOTelInterceptor_Tracer_Field(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	o := NewOTelInterceptor(tracer)
	if o.tracer != tracer {
		t.Error("tracer should be stored")
	}
}

// Compile-time check that noop tracer implements trace.Tracer
var _ trace.Tracer = noop.NewTracerProvider().Tracer("")
