package serialization

import "sync"

// BufPool provides a pool of byte-slice buffers for zero-allocation
// encoding. High-throughput cache paths can borrow a buffer from the
// pool, encode into it, and return it when done. This dramatically
// reduces GC pressure compared to allocating a new []byte per encode.
//
// The pool grows on demand up to a soft limit (see sync.Pool semantics)
// and is drained automatically during GC.
//
// Typical usage:
//
//	buf := pool.Get()
//	encoded, err := codec.Encode(value, *buf)
//	pool.Put(buf)
type BufPool struct {
	pool sync.Pool
	size int
}

// NewBufPool creates a buffer pool where each buffer has an initial
// capacity of size bytes. Callers should choose a size that accommodates
// the majority of their encoded values to avoid re-slicing.
func NewBufPool(size int) *BufPool {
	if size <= 0 {
		size = 4096 // sensible default: 4 KiB
	}
	p := &BufPool{size: size}
	p.pool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, size)
			return &b
		},
	}
	return p
}

// Get returns a pointer to a byte slice from the pool. The slice has
// length 0 and capacity ≥ the pool's configured size. The caller owns
// the returned buffer until Put is called.
func (p *BufPool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

// Put returns a buffer to the pool. The buffer is reset to length 0
// before being placed back in the pool so that subsequent callers
// receive a clean slice.
func (p *BufPool) Put(b *[]byte) {
	if b == nil {
		return
	}
	// Reset to zero length but preserve capacity.
	*b = (*b)[:0]
	p.pool.Put(b)
}

// Size returns the configured initial capacity of each buffer in the pool.
func (p *BufPool) Size() int {
	return p.size
}
