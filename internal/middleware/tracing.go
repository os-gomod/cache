package middleware

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// TracingMiddleware returns a Middleware that creates OpenTelemetry spans
// for each cache operation. The span name follows the pattern "cache.{op.Name}"
// and includes the backend and key as span attributes when present.
//
// If the tracer is nil or no span context exists in the incoming context,
// the middleware passes through without creating a span.
func TracingMiddleware(tracer trace.Tracer) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			if tracer == nil {
				return next(ctx, op)
			}

			spanName := "cache." + op.Name
			ctx, span := tracer.Start(ctx, spanName)
			defer span.End()

			// Set common attributes
			if op.Backend != "" {
				span.SetAttributes(
					attribute.String("cache.backend", op.Backend),
				)
			}
			if op.Key != "" {
				span.SetAttributes(
					attribute.String("cache.key", op.Key),
				)
			}
			if op.KeyCount > 0 {
				span.SetAttributes(
					attribute.Int("cache.key_count", op.KeyCount),
				)
			}

			err := next(ctx, op)
			if err != nil {
				span.RecordError(err)
			}
			return err
		}
	}
}
