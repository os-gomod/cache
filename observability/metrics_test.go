package observability

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewPrometheusInterceptor_NilRegisterer(t *testing.T) {
	_, err := NewPrometheusInterceptor(nil)
	if err == nil {
		t.Error("expected error for nil registerer")
	}
}

func TestNewPrometheusInterceptor(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := NewPrometheusInterceptor(reg)
	if err != nil {
		t.Fatalf("NewPrometheusInterceptor() error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestNewPrometheusInterceptor_WithNamespace(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := NewPrometheusInterceptor(reg, PrometheusConfig{Namespace: "custom"})
	if err != nil {
		t.Fatalf("NewPrometheusInterceptor() error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestMustNewPrometheusInterceptor(t *testing.T) {
	reg := prometheus.NewRegistry()
	p := MustNewPrometheusInterceptor(reg)
	if p == nil {
		t.Fatal("MustNewPrometheusInterceptor returned nil")
	}
}

func TestMustNewPrometheusInterceptor_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil registerer")
		}
	}()
	MustNewPrometheusInterceptor(nil)
}

func TestPrometheusInterceptor_Before(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	ctx := p.Before(context.Background(), Op{Backend: "test", Name: "get"})
	if ctx == nil {
		t.Error("Before should return non-nil context")
	}
}

func TestPrometheusInterceptor_After_Hit(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	p.After(context.Background(), Op{Backend: "test", Name: "get"}, Result{Hit: true, Latency: 5 * time.Millisecond})
}

func TestPrometheusInterceptor_After_Miss(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	p.After(context.Background(), Op{Backend: "test", Name: "get"}, Result{Hit: false, Latency: time.Millisecond})
}

func TestPrometheusInterceptor_After_Error(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	p.After(context.Background(), Op{Backend: "test", Name: "set"}, Result{Err: context.DeadlineExceeded, Latency: time.Millisecond})
}

func TestPrometheusInterceptor_After_WriteOp(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	// Write operations shouldn't record hit/miss counters
	p.After(context.Background(), Op{Backend: "test", Name: "set"}, Result{Hit: false, Latency: time.Millisecond, ByteSize: 100})
}

func TestPrometheusInterceptor_After_WithByteSize(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	p.After(context.Background(), Op{Backend: "test", Name: "get"}, Result{Hit: true, ByteSize: 1024, Latency: time.Millisecond})
}

func TestPrometheusInterceptor_After_ZeroLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, _ := NewPrometheusInterceptor(reg)
	p.After(context.Background(), Op{Backend: "test", Name: "get"}, Result{Hit: true, Latency: 0})
}
