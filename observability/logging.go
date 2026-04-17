package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/os-gomod/cache/internal/stringutil"
)

// LoggingInterceptor logs cache operations using slog.
// It supports configurable log levels and slow-query thresholds.
type LoggingInterceptor struct {
	logger *slog.Logger
	level  slog.Level
	slow   time.Duration
}

// LoggingOption is a functional option for LoggingInterceptor.
type LoggingOption func(*LoggingInterceptor)

// WithLoggingLevel sets the minimum log level for cache operations.
func WithLoggingLevel(level slog.Level) LoggingOption {
	return func(l *LoggingInterceptor) { l.level = level }
}

// WithSlowThreshold sets the latency threshold above which operations
// are logged at WARN level regardless of the configured log level.
func WithSlowThreshold(d time.Duration) LoggingOption {
	return func(l *LoggingInterceptor) { l.slow = d }
}

// NewLoggingInterceptor creates a new logging interceptor.
// Panics if logger is nil.
func NewLoggingInterceptor(logger *slog.Logger, opts ...LoggingOption) *LoggingInterceptor {
	if logger == nil {
		panic("observability: LoggingInterceptor requires a non-nil logger")
	}
	l := &LoggingInterceptor{
		logger: logger,
		level:  slog.LevelDebug,
		slow:   0,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Before returns the context unchanged. The logging happens in After.
func (l *LoggingInterceptor) Before(ctx context.Context, _ Op) context.Context {
	return ctx
}

// After logs the result of a cache operation.
func (l *LoggingInterceptor) After(ctx context.Context, op Op, result Result) {
	attrs := []slog.Attr{
		slog.String("backend", op.Backend),
		slog.String("op", op.Name),
		slog.String("key", stringutil.TruncateKey(op.Key, 32)),
		slog.Int64("latency_ns", result.Latency.Nanoseconds()),
	}
	if stringutil.IsReadOp(op.Name) {
		attrs = append(attrs, slog.Bool("hit", result.Hit))
	}
	if result.ByteSize > 0 {
		attrs = append(attrs, slog.Int("bytes", result.ByteSize))
	}
	if result.Err != nil {
		attrs = append(attrs, slog.String("err", result.Err.Error()))
	}
	level := l.level
	if l.slow > 0 && result.Latency > l.slow {
		level = slog.LevelWarn
		attrs = append(attrs, slog.Bool("slow", true))
	}
	msg := "cache." + op.Backend + "." + op.Name
	switch {
	case level >= slog.LevelError && result.Err != nil:
		l.logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
	case level >= slog.LevelWarn:
		l.logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
	case level >= slog.LevelInfo:
		l.logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
	default:
		l.logger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
	}
}
