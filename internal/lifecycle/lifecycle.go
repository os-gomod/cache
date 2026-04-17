// Package lifecycle provides start/close state management with guard patterns.
package lifecycle

import (
	"sync/atomic"

	cacheerrors "github.com/os-gomod/cache/errors"
)

type Guard struct {
	closed atomic.Bool
}

func (g *Guard) CheckClosed(op string) error {
	if g.closed.Load() {
		return cacheerrors.Closed(op)
	}
	return nil
}

func (g *Guard) Close() (alreadyClosed bool) {
	return g.closed.Swap(true)
}

func (g *Guard) IsClosed() bool {
	return g.closed.Load()
}
