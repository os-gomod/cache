// Package pooling provides allocation-efficient buffer pools for cache
// encoding hot paths. The BufPool reduces GC pressure by reusing scratch
// buffers across Encode/Decode cycles.
package pooling

import "sync"

// BufPool is a typed sync.Pool for byte-slice scratch buffers. It is safe
// for concurrent use. The default buffer capacity is 64 bytes, which is
// sufficient for most codec scratch-buffer operations (int64/float64
// formatting, small JSON payloads).
//
// Usage:
//
//	pool := pooling.NewBufPool(64)
//	buf := pool.Get()
//	// ... use buf ...
//	pool.Put(buf)
type BufPool struct {
	pool sync.Pool
	cap  int
}

// NewBufPool creates a BufPool that allocates buffers with the given initial
// capacity. If cap is <= 0, a default of 64 is used.
func NewBufPool(capacity int) *BufPool {
	if capacity <= 0 {
		capacity = 64
	}
	return &BufPool{
		cap: capacity,
		pool: sync.Pool{
			New: func() any {
				buf := make([]byte, 0, capacity)
				return &buf
			},
		},
	}
}

// Get retrieves a *[]byte from the pool. The returned slice has len=0 and
// cap>=pool.cap. Callers must not assume the slice is zeroed.
func (p *BufPool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

// Put returns a *[]byte to the pool. The slice is reset to len=0 before
// storage so that subsequent Get calls return an empty buffer.
func (p *BufPool) Put(buf *[]byte) {
	*buf = (*buf)[:0]
	p.pool.Put(buf)
}
