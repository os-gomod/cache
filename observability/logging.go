package observability

import (
	"context"
	"log/slog"
	"time"
)

// LoggingInterceptor logs structured cache operation results using Go's
// standard library slog. It supports optional slow-query detection that
// logs at Warn level when latency exceeds a configured threshold.
type LoggingInterceptor struct {
	logger *slog.Logger
	level  slog.Level
	slow   time.Duration // log at Warn if latency > slow (0 = disabled)
}

// LoggingOption configures a LoggingInterceptor.
type LoggingOption func(*LoggingInterceptor)

// WithLoggingLevel sets the log level for normal operations.
// Default is slog.LevelDebug.
func WithLoggingLevel(level slog.Level) LoggingOption {
	return func(l *LoggingInterceptor) { l.level = level }
}

// WithSlowThreshold sets the latency threshold for slow query detection.
// When a result's latency exceeds this threshold, it is logged at Warn
// level regardless of the configured level. Zero disables slow detection.
func WithSlowThreshold(d time.Duration) LoggingOption {
	return func(l *LoggingInterceptor) { l.slow = d }
}

// NewLoggingInterceptor creates a logging interceptor. The logger must not
// be nil. Options may be provided to customize behavior.
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

// Before is a no-op for logging — all recording happens in After.
func (l *LoggingInterceptor) Before(ctx context.Context, _ Op) context.Context {
	return ctx
}

// After logs the operation result with structured fields. For get operations,
// the hit field is included. Slow queries are always logged at Warn level
// regardless of the configured level setting.
func (l *LoggingInterceptor) After(ctx context.Context, op Op, result Result) {
	attrs := []slog.Attr{
		slog.String("backend", op.Backend),
		slog.String("op", op.Name),
		slog.String("key", truncateKey(op.Key, 32)),
		slog.Int64("latency_ns", result.Latency.Nanoseconds()),
	}

	if isReadOp(op.Name) {
		attrs = append(attrs, slog.Bool("hit", result.Hit))
	}

	if result.ByteSize > 0 {
		attrs = append(attrs, slog.Int("bytes", result.ByteSize))
	}

	if result.Err != nil {
		attrs = append(attrs, slog.String("err", result.Err.Error()))
	}

	// Determine the effective log level.
	level := l.level
	if l.slow > 0 && result.Latency > l.slow {
		level = slog.LevelWarn
		attrs = append(attrs, slog.Bool("slow", true))
	}

	// Log at the determined level.
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

// truncateKey truncates a key to at most maxLen characters for safe logging.
func truncateKey(key string, maxLen int) string {
	if len(key) <= maxLen {
		return key
	}
	if maxLen <= 3 {
		return key[:maxLen]
	}
	return key[:maxLen-3] + "..."
}
