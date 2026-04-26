// Package promotion manages L1 promotion control for layered cache stores.
// It tracks keys that should not be promoted from L2 to L1, which is useful
// for write-heavy patterns where promotion would waste L1 capacity on
// frequently-updated keys.
package promotion

import "sync"

// Engine manages promotion control for the layered cache. It maintains a
// thread-safe set of keys that should be excluded from L1 promotion.
type Engine struct {
	noPromote sync.Map // map[string]struct{}
}

// New creates a new promotion Engine.
func New() *Engine {
	return &Engine{}
}

// ShouldPromote reports whether the given key should be promoted from L2
// to L1. Returns false if the key is in the do-not-promote set.
func (e *Engine) ShouldPromote(key string) bool {
	_, blocked := e.noPromote.Load(key)
	return !blocked
}

// DisablePromotion adds the key to the do-not-promote set. Future calls
// to ShouldPromote(key) will return false until EnablePromotion is called.
func (e *Engine) DisablePromotion(key string) {
	e.noPromote.Store(key, struct{}{})
}

// EnablePromotion removes the key from the do-not-promote set, allowing
// it to be promoted again.
func (e *Engine) EnablePromotion(key string) {
	e.noPromote.Delete(key)
}

// IsPromotionDisabled reports whether promotion is currently disabled for
// the given key.
func (e *Engine) IsPromotionDisabled(key string) bool {
	_, blocked := e.noPromote.Load(key)
	return blocked
}

// Clear removes all keys from the do-not-promote set.
func (e *Engine) Clear() {
	e.noPromote.Range(func(key, _ interface{}) bool {
		e.noPromote.Delete(key)
		return true
	})
}

// Size returns the number of keys in the do-not-promote set.
func (e *Engine) Size() int {
	count := 0
	e.noPromote.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
