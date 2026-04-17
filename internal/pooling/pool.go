// Package pooling provides sync.Pool-based buffer pools to reduce GC pressure.
package pooling

import "sync"

type BufPool struct {
	pool sync.Pool
	cap  int
}

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

func (p *BufPool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

func (p *BufPool) Put(buf *[]byte) {
	*buf = (*buf)[:0]
	p.pool.Put(buf)
}
