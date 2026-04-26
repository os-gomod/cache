// Package consistency provides write-mode configuration for layered cache
// stores. It defines the three standard write patterns: write-through,
// write-behind (async), and write-around.
package consistency

// WriteMode defines how writes are propagated through the cache layers.
type WriteMode int

const (
	// WriteThrough writes to both L1 and L2 synchronously. This provides
	// the strongest consistency guarantee but has the highest latency.
	WriteThrough WriteMode = iota

	// WriteBehind writes to L1 immediately and queues the L2 write for
	// asynchronous processing. This provides lower write latency at the
	// cost of temporary inconsistency between L1 and L2.
	WriteBehind

	// WriteAround bypasses L1 and writes directly to L2. Subsequent reads
	// will populate L1 on demand. This is useful for write-heavy workloads
	// where L1 would be quickly polluted by write-only data.
	WriteAround
)

// String returns the human-readable name of the write mode.
func (m WriteMode) String() string {
	switch m {
	case WriteThrough:
		return "write_through"
	case WriteBehind:
		return "write_behind"
	case WriteAround:
		return "write_around"
	default:
		return "unknown"
	}
}

// Engine manages the write mode configuration for a layered cache store.
// It determines how writes are propagated through L1 and L2.
type Engine struct {
	mode WriteMode
}

// New creates a new consistency Engine with the given write mode.
func New(mode WriteMode) *Engine {
	return &Engine{mode: mode}
}

// Mode returns the current write mode.
func (e *Engine) Mode() WriteMode {
	return e.mode
}

// SetMode changes the write mode. This may be useful for runtime
// reconfiguration of write behavior.
func (e *Engine) SetMode(mode WriteMode) {
	e.mode = mode
}

// IsWriteThrough reports whether the mode is WriteThrough.
func (e *Engine) IsWriteThrough() bool {
	return e.mode == WriteThrough
}

// IsWriteBehind reports whether the mode is WriteBehind.
func (e *Engine) IsWriteBehind() bool {
	return e.mode == WriteBehind
}

// IsWriteAround reports whether the mode is WriteAround.
func (e *Engine) IsWriteAround() bool {
	return e.mode == WriteAround
}

// ShouldWriteToL1 reports whether a Set operation should write to L1.
// Returns false only in WriteAround mode.
func (e *Engine) ShouldWriteToL1() bool {
	return e.mode != WriteAround
}

// ShouldWriteToL2 reports whether a Set operation should write to L2.
// Returns true for all modes, but the timing differs (sync vs async).
func (*Engine) ShouldWriteToL2() bool {
	return true
}

// IsAsyncL2 reports whether L2 writes should be asynchronous.
func (e *Engine) IsAsyncL2() bool {
	return e.mode == WriteBehind
}
