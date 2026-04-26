package runtime

import (
	"time"

	"github.com/os-gomod/cache/v2/internal/middleware"
)

// ExecutorOption is a functional option for configuring an Executor.
type ExecutorOption func(*Executor)

// WithChain sets the middleware chain for the Executor.
// A nil chain disables middleware processing.
func WithChain(chain *middleware.Chain) ExecutorOption {
	return func(e *Executor) {
		e.chain = chain
	}
}

// WithValidator sets the key validation function for the Executor.
// A nil validator disables key validation.
func WithValidator(validator KeyValidator) ExecutorOption {
	return func(e *Executor) {
		e.validator = validator
	}
}

// WithStats sets the stats collector for the Executor.
// A nil stats collector disables statistics recording.
func WithStats(stats StatsCollector) ExecutorOption {
	return func(e *Executor) {
		e.stats = stats
	}
}

// WithClock sets the clock for the Executor. This is primarily useful for
// testing with a deterministic clock.
func WithClock(clock Clock) ExecutorOption {
	return func(e *Executor) {
		e.clock = clock
	}
}

// fakeClock is a test clock that returns a fixed time and controllable durations.
type fakeClock struct {
	now     time.Time
	elapsed time.Duration
}

// NewFakeClock creates a fake clock starting at the given time.
func NewFakeClock(now time.Time, elapsed time.Duration) Clock {
	return &fakeClock{
		now:     now,
		elapsed: elapsed,
	}
}

func (c *fakeClock) Now() time.Time                  { return c.now }
func (c *fakeClock) Since(_ time.Time) time.Duration { return c.elapsed }
