package middleware

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

// LogEntry represents a single structured log entry for a cache operation.
type LogEntry struct {
	// Op is the operation name (e.g., "get", "set").
	Op string

	// Backend is the backend identifier.
	Backend string

	// Key is the primary cache key.
	Key string

	// Latency is the wall-clock duration of the operation.
	Latency time.Duration

	// Err is any error that occurred during the operation (nil for success).
	Err error
}

// Logger is the interface for structured logging in cache middleware.
// Implementations can route log entries to any logging backend.
type Logger interface {
	// Log processes a structured log entry for a cache operation.
	Log(entry LogEntry)
}

// LoggingMiddleware returns a Middleware that logs each cache operation
// using the provided Logger. The log entry includes the operation name,
// backend, key, latency, and any error.
func LoggingMiddleware(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, op contracts.Operation) error {
			start := time.Now()
			err := next(ctx, op)
			latency := time.Since(start)

			logger.Log(LogEntry{
				Op:      op.Name,
				Backend: op.Backend,
				Key:     op.Key,
				Latency: latency,
				Err:     err,
			})

			return err
		}
	}
}
