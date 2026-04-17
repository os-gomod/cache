package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/os-gomod/cache/internal/stringutil"
)

// OTelInterceptor integrates cache operations with OpenTelemetry distributed tracing.
// It creates spans for each cache operation and records hit/miss/error attributes.
type OTelInterceptor struct {
	tracer trace.Tracer
}

// NewOTelInterceptor creates a new OpenTelemetry tracing interceptor.
// Panics if tracer is nil.
func NewOTelInterceptor(tracer trace.Tracer) *OTelInterceptor {
	if tracer == nil {
		panic("observability: OTelInterceptor requires a non-nil tracer")
	}
	return &OTelInterceptor{tracer: tracer}
}

type otelSpanKey struct{}

// Before starts a new OpenTelemetry span for the cache operation and
// attaches it to the context.
func (o *OTelInterceptor) Before(ctx context.Context, op Op) context.Context {
	spanName := fmt.Sprintf("cache.%s.%s", op.Backend, op.Name)
	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("db.system", "cache"),
			attribute.String("db.operation", op.Name),
			attribute.String("cache.backend", op.Backend),
		),
	}
	if op.Key != "" {
		opts = append(opts, trace.WithAttributes(
			attribute.String("cache.key", op.Key),
		))
	}
	if op.KeyCount > 0 {
		opts = append(opts, trace.WithAttributes(
			attribute.Int("cache.key_count", op.KeyCount),
		))
	}
	ctx, span := o.tracer.Start(ctx, spanName, opts...)
	return context.WithValue(ctx, otelSpanKey{}, span)
}

// After ends the OpenTelemetry span and records the operation result.
func (o *OTelInterceptor) After(ctx context.Context, op Op, result Result) {
	span, ok := ctx.Value(otelSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	if stringutil.IsReadOp(op.Name) {
		span.SetAttributes(attribute.Bool("cache.hit", result.Hit))
	}
	if result.Err != nil {
		span.SetStatus(codes.Error, result.Err.Error())
		span.RecordError(result.Err)
	}
	span.End()
}
