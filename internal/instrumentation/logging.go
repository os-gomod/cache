package instrumentation

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// LogEntry carries structured data for a single log line. All fields are
// serialized to JSON when the entry is written, making the output
// machine-parseable by log aggregation systems (ELK, Loki, CloudWatch).
//
// Use pointer receivers when passing LogEntry to Logger methods to avoid
// expensive copies of this 136-byte struct.
type LogEntry struct {
	// Timestamp is the time at which the log entry was created.
	Timestamp time.Time `json:"timestamp"`

	// Level is the log severity: "debug", "info", "warn", or "error".
	Level string `json:"level"`

	// Backend identifies the cache backend (e.g. "redis", "memory").
	Backend string `json:"backend,omitempty"`

	// Operation is the cache operation name (e.g. "get", "set").
	Operation string `json:"operation,omitempty"`

	// Key is the primary cache key involved in the operation.
	Key string `json:"key,omitempty"`

	// Latency is the operation duration, if applicable.
	Latency time.Duration `json:"latency_ns,omitempty"`

	// Error holds the error, if any. Nil entries are omitted from JSON.
	Error error `json:"-"`

	// Metadata contains arbitrary key-value pairs for additional context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON implements custom JSON marshaling that converts the Error
// field to a string.
func (e *LogEntry) MarshalJSON() ([]byte, error) {
	type Alias LogEntry
	payload := struct {
		*Alias
		Error string `json:"error,omitempty"`
	}{
		Alias: (*Alias)(e),
	}
	if e.Error != nil {
		payload.Error = e.Error.Error()
	}
	//nolint:wrapcheck // error is already wrapped by internal packages
	return json.Marshal(payload)
}

// ---------------------------------------------------------------------------
// Logger interface
// ---------------------------------------------------------------------------

// Logger defines the contract for structured log emitters. Implementations
// may write to stdout, files, syslog, or external log aggregation services.
type Logger interface {
	Debug(entry *LogEntry)
	Info(entry *LogEntry)
	Warn(entry *LogEntry)
	Error(entry *LogEntry)
}

// ---------------------------------------------------------------------------

const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

// logLevelPriority assigns a numeric priority to each log level for
// filtering. Higher values are more severe.
var logLevelPriority = map[string]int{
	levelDebug: 0,
	levelInfo:  1,
	levelWarn:  2,
	levelError: 3,
}

// DefaultLogger implements Logger by writing JSON-formatted log entries
// to an io.Writer. It supports configurable log-level filtering so that
// verbose debug entries can be suppressed in production.
type DefaultLogger struct {
	writer io.Writer
	level  string
	mu     sync.Mutex

	// encoder is created once and reused to avoid per-call allocation.
	encoder *json.Encoder
}

// NewDefaultLogger creates a structured logger that writes JSON entries
// to w. The level parameter controls the minimum severity: entries with
// a lower priority are silently discarded.
//
// Accepted levels: "debug", "info", "warn", "error".
func NewDefaultLogger(w io.Writer, level string) *DefaultLogger {
	if w == nil {
		w = os.Stdout
	}

	if _, ok := logLevelPriority[level]; !ok {
		level = levelInfo
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) // avoid mangling cache keys containing < > &

	return &DefaultLogger{
		writer:  w,
		level:   level,
		encoder: enc,
	}
}

// shouldLog returns true if the given log level meets or exceeds the
// configured minimum level.
func (l *DefaultLogger) shouldLog(level string) bool {
	return logLevelPriority[level] >= logLevelPriority[l.level]
}

// setTimestamp ensures the entry has a timestamp.
func (*DefaultLogger) setTimestamp(entry *LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
}

// Debug logs a debug-level entry if the configured level allows it.
func (l *DefaultLogger) Debug(entry *LogEntry) {
	if !l.shouldLog(levelDebug) {
		return
	}
	entry.Level = levelDebug
	l.log(entry)
}

// Info logs an info-level entry if the configured level allows it.
func (l *DefaultLogger) Info(entry *LogEntry) {
	if !l.shouldLog(levelInfo) {
		return
	}
	entry.Level = levelInfo
	l.log(entry)
}

// Warn logs a warn-level entry if the configured level allows it.
func (l *DefaultLogger) Warn(entry *LogEntry) {
	if !l.shouldLog(levelWarn) {
		return
	}
	entry.Level = levelWarn
	l.log(entry)
}

// Error logs an error-level entry. Error-level entries are never filtered.
func (l *DefaultLogger) Error(entry *LogEntry) {
	if !l.shouldLog(levelError) {
		return
	}
	entry.Level = levelError
	l.log(entry)
}

// log serializes the entry as JSON and writes it to the configured writer.
// The mutex ensures that concurrent writes do not interleave.
func (l *DefaultLogger) log(entry *LogEntry) {
	l.setTimestamp(entry)

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.encoder.Encode(entry); err != nil {
		// Fallback: write a plain text line to avoid silent data loss.
		//nolint:errcheck // best-effort fallback
		io.WriteString(
			l.writer,
			"{\"level\":\"error\",\"message\":\"log encoding failed: "+err.Error()+"\"}\n",
		)
	}
}

// SetLevel changes the minimum log level at runtime.
func (l *DefaultLogger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := logLevelPriority[level]; ok {
		l.level = level
	}
}

// Level returns the current minimum log level.
func (l *DefaultLogger) Level() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}
