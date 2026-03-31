// Package obs defines pluggable observability hooks (tracing, logging, metrics)
// that are no-op by default.  Production callers inject concrete implementations
// via obs.SetProvider; library code only calls the package-level helpers.
package obs

import (
	"context"
	"time"
)

// ----------------------------------------------------------------------------
// Span — minimal tracing abstraction
// ----------------------------------------------------------------------------

// Span represents a single unit of work.  The zero-value nopSpan is always safe.
type Span interface {
	// End marks the span as finished.
	End()
	// SetError annotates the span with an error.
	SetError(err error)
	// SetAttr attaches an arbitrary key/value attribute to the span.
	SetAttr(key string, val any)
}

// nopSpan is the default no-op span returned when no tracer is configured.
type nopSpan struct{}

func (nopSpan) End()                    {}
func (nopSpan) SetError(_ error)        {}
func (nopSpan) SetAttr(_ string, _ any) {}

// ----------------------------------------------------------------------------
// Tracer
// ----------------------------------------------------------------------------

// Tracer creates spans.
type Tracer interface {
	Start(ctx context.Context, op string) (context.Context, Span)
}

type nopTracer struct{}

func (nopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, nopSpan{}
}

// ----------------------------------------------------------------------------
// Logger
// ----------------------------------------------------------------------------

// Logger is a structured logger.  Level is caller's responsibility.
type Logger interface {
	Info(ctx context.Context, msg string, fields ...any)
	Warn(ctx context.Context, msg string, fields ...any)
	Error(ctx context.Context, msg string, fields ...any)
	Debug(ctx context.Context, msg string, fields ...any)
}

type nopLogger struct{}

func (nopLogger) Info(_ context.Context, _ string, _ ...any)  {}
func (nopLogger) Warn(_ context.Context, _ string, _ ...any)  {}
func (nopLogger) Error(_ context.Context, _ string, _ ...any) {}
func (nopLogger) Debug(_ context.Context, _ string, _ ...any) {}

// ----------------------------------------------------------------------------
// MetricsRecorder
// ----------------------------------------------------------------------------

// MetricsRecorder records cache-operation metrics.
type MetricsRecorder interface {
	// RecordHit records a cache hit with its latency.
	RecordHit(ctx context.Context, backend, op string, d time.Duration)
	// RecordMiss records a cache miss with its latency.
	RecordMiss(ctx context.Context, backend, op string, d time.Duration)
	// RecordError records an operation error.
	RecordError(ctx context.Context, backend, op string)
	// RecordEviction records a key eviction.
	RecordEviction(ctx context.Context, backend string)
}

type nopMetrics struct{}

func (nopMetrics) RecordHit(_ context.Context, _, _ string, _ time.Duration)  {}
func (nopMetrics) RecordMiss(_ context.Context, _, _ string, _ time.Duration) {}
func (nopMetrics) RecordError(_ context.Context, _, _ string)                 {}
func (nopMetrics) RecordEviction(_ context.Context, _ string)                 {}

// ----------------------------------------------------------------------------
// Provider — aggregates all observability implementations
// ----------------------------------------------------------------------------

// Provider bundles all observability backends.
type Provider struct {
	Tracer  Tracer
	Logger  Logger
	Metrics MetricsRecorder
}

// nopProvider is the default, fully no-op implementation.
var nopProvider = &Provider{
	Tracer:  nopTracer{},
	Logger:  nopLogger{},
	Metrics: nopMetrics{},
}

// global is the process-wide default provider; always non-nil.
var global = nopProvider

// SetProvider replaces the process-wide provider.  Call once during
// application startup, before any cache is constructed.
// SetProvider is NOT safe for concurrent use with package-level helpers.
func SetProvider(p *Provider) {
	if p == nil {
		global = nopProvider
		return
	}
	if p.Tracer == nil {
		p.Tracer = nopTracer{}
	}
	if p.Logger == nil {
		p.Logger = nopLogger{}
	}
	if p.Metrics == nil {
		p.Metrics = nopMetrics{}
	}
	global = p
}

// Global returns the current process-wide provider (never nil).
func Global() *Provider { return global }

// ----------------------------------------------------------------------------
// Package-level convenience wrappers
// ----------------------------------------------------------------------------

// Start begins a traced span, delegating to the global tracer.
func Start(ctx context.Context, op string) (context.Context, Span) {
	return global.Tracer.Start(ctx, op)
}

// Info logs an informational message.
func Info(ctx context.Context, msg string, fields ...any) {
	global.Logger.Info(ctx, msg, fields...)
}

// Warn logs a warning.
func Warn(ctx context.Context, msg string, fields ...any) {
	global.Logger.Warn(ctx, msg, fields...)
}

// Error logs an error.
func Error(ctx context.Context, msg string, fields ...any) {
	global.Logger.Error(ctx, msg, fields...)
}

// Debug logs a debug message.
func Debug(ctx context.Context, msg string, fields ...any) {
	global.Logger.Debug(ctx, msg, fields...)
}

// RecordHit records a cache hit.
func RecordHit(ctx context.Context, backend, op string, d time.Duration) {
	global.Metrics.RecordHit(ctx, backend, op, d)
}

// RecordMiss records a cache miss.
func RecordMiss(ctx context.Context, backend, op string, d time.Duration) {
	global.Metrics.RecordMiss(ctx, backend, op, d)
}

// RecordError records an operation error.
func RecordError(ctx context.Context, backend, op string) {
	global.Metrics.RecordError(ctx, backend, op)
}

// RecordEviction records a key eviction.
func RecordEviction(ctx context.Context, backend string) {
	global.Metrics.RecordEviction(ctx, backend)
}
