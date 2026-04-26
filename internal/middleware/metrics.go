package middleware

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// MetricsRecorder records metrics for cache operations.
type MetricsRecorder interface {
	// Record is called after each cache operation completes.
	// The op is the operation name, latency is the wall-clock duration,
	// and err is any error that occurred (nil for success).
	Record(op string, latency time.Duration, err error)
}

// MetricsMiddleware returns a Middleware that records operation metrics
// (operation count, latency, errors) using the provided MetricsRecorder.
//
// This middleware measures the wall-clock time of the entire operation
// including any downstream middleware, and records it via the recorder.
func MetricsMiddleware(recorder MetricsRecorder) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			start := time.Now()
			err := next(ctx, op)
			latency := time.Since(start)
			recorder.Record(op.Name, latency, err)
			return err
		}
	}
}
