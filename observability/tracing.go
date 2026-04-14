package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTelInterceptor creates OpenTelemetry spans for cache operations.
// Each Before starts a span; each After ends it with appropriate status.
type OTelInterceptor struct {
	tracer trace.Tracer
}

// NewOTelInterceptor creates an interceptor that uses the provided tracer.
// The tracer must not be nil.
func NewOTelInterceptor(tracer trace.Tracer) *OTelInterceptor {
	if tracer == nil {
		panic("observability: OTelInterceptor requires a non-nil tracer")
	}
	return &OTelInterceptor{tracer: tracer}
}

type otelSpanKey struct{}

// Before starts a span named "cache.<backend>.<op>" and adds key as a
// span attribute. The span is stored in the context for After to retrieve.
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

// After records the operation result on the span and ends it.
// On get operations, the cache.hit attribute is set.
// Errors set the span status to Error.
func (o *OTelInterceptor) After(ctx context.Context, op Op, result Result) {
	span, ok := ctx.Value(otelSpanKey{}).(trace.Span)
	if !ok {
		return
	}

	// Record hit attribute for read operations.
	if isReadOp(op.Name) {
		span.SetAttributes(attribute.Bool("cache.hit", result.Hit))
	}

	if result.Err != nil {
		span.SetStatus(codes.Error, result.Err.Error())
		span.RecordError(result.Err)
	}

	span.End()
}

// isReadOp returns true for operations that can produce a cache hit.
func isReadOp(name string) bool {
	switch name {
	case "get", "get_multi", "get_or_set", "getset", "exists":
		return true
	}
	return false
}
