package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestNewLoggingInterceptor_NilLogger(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for nil logger")
		}
	}()
	NewLoggingInterceptor(nil)
}

func TestNewLoggingInterceptor_Defaults(t *testing.T) {
	l := NewLoggingInterceptor(slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)))
	if l == nil {
		t.Fatal("NewLoggingInterceptor returned nil")
	}
}

func TestLoggingInterceptor_Before(t *testing.T) {
	l := NewLoggingInterceptor(slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)))
	ctx := l.Before(context.Background(), Op{Backend: "memory", Name: "get", Key: "user:1"})
	if ctx == nil {
		t.Error("Before should return non-nil context")
	}
}

func TestLoggingInterceptor_After_Hit(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewLoggingInterceptor(slog.New(h))
	l.After(context.Background(), Op{Backend: "memory", Name: "get", Key: "user:1"}, Result{Hit: true, Latency: 1})
	if buf.Len() == 0 {
		t.Error("After should log something")
	}
}

func TestLoggingInterceptor_After_Miss(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewLoggingInterceptor(slog.New(h))
	l.After(context.Background(), Op{Backend: "memory", Name: "get", Key: "user:1"}, Result{Hit: false})
	if buf.Len() == 0 {
		t.Error("After should log miss")
	}
}

func TestLoggingInterceptor_After_Error(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewLoggingInterceptor(slog.New(h))
	l.After(context.Background(), Op{Backend: "memory", Name: "get"}, Result{Err: context.DeadlineExceeded})
	output := buf.String()
	if len(output) == 0 {
		t.Error("should log error")
	}
}

func TestLoggingInterceptor_After_Slow(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoggingInterceptor(
		slog.New(slog.NewTextHandler(&buf, nil)),
		WithSlowThreshold(time.Millisecond),
		WithLoggingLevel(slog.LevelDebug),
	)
	l.After(context.Background(), Op{Backend: "memory", Name: "get"}, Result{Latency: 10 * time.Millisecond})
	// Should log at warn due to slow threshold
	output := buf.String()
	if len(output) == 0 {
		t.Error("should log slow query")
	}
}

func TestWithLoggingLevel(t *testing.T) {
	l := NewLoggingInterceptor(
		slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)),
		WithLoggingLevel(slog.LevelInfo),
	)
	if l == nil {
		t.Fatal("expected non-nil")
	}
}
