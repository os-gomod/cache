package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// Tracer wraps an OpenTelemetry trace.Tracer to create cache-specific
// spans. Every cache operation is represented as a span named
// "cache.<operation>" with standard attributes for backend, key, and
// key count.
//
// Typical usage:
//
//	tracer := NewTracer(provider, "github.com/os-gomod/cache/v2")
//	ctx, span := tracer.Start(ctx, contracts.Operation{
//	    Name: "get", Key: "user:123", Backend: "redis",
//	})
//	// ... perform operation ...
//	tracer.End(span, err)
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a cache Tracer backed by the given TracerProvider.
// The name parameter identifies the instrumentation library and should
// match the Go module path.
func NewTracer(provider trace.TracerProvider, name string) *Tracer {
	return &Tracer{
		tracer: provider.Tracer(name,
			trace.WithInstrumentationVersion("1.0.0"),
		),
	}
}

// Start begins a new OpenTelemetry span for the given cache operation.
// The span name follows the pattern "cache.<op.Name>" (e.g. "cache.get",
// "cache.set"). Standard attributes are added:
//
//   - cache.backend: the backend name (e.g. "redis", "memory")
//   - cache.key: the primary cache key (if non-empty)
//   - cache.key_count: the number of keys involved (for multi-key ops)
//
// Start returns the updated context (with the span attached) and the
// created span. The caller MUST call End on the span when the operation
// completes.
//
//nolint:spancheck // span is intentionally returned for caller-managed lifecycle
func (t *Tracer) Start(ctx context.Context, op contracts.Operation) (context.Context, trace.Span) {
	spanName := "cache." + op.Name

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("cache.backend", op.Backend),
			attribute.Int("cache.key_count", op.KeyCount),
		),
	}

	if op.Key != "" {
		opts = append(opts, trace.WithAttributes(
			attribute.String("cache.key", op.Key),
		))
	}

	ctx, span := t.tracer.Start(ctx, spanName, opts...)
	return ctx, span
}

// End finishes the span. If err is non-nil, the span is marked as an
// error with the error message and type recorded as attributes.
func (*Tracer) End(span trace.Span, err error) {
	if span == nil {
		return
	}

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("cache.error", true),
			attribute.String("cache.error.message", err.Error()),
		)
	}

	span.End()
}
