package pooling

import (
	"testing"
)

func TestNewBufPool(t *testing.T) {
	p := NewBufPool(128)
	if p == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestNewBufPool_ZeroCapacity(t *testing.T) {
	p := NewBufPool(0)
	if p == nil {
		t.Fatal("expected non-nil pool for zero capacity")
	}
	buf := p.Get()
	if cap(*buf) != 64 {
		t.Errorf("expected default cap 64, got %d", cap(*buf))
	}
}

func TestNewBufPool_NegativeCapacity(t *testing.T) {
	p := NewBufPool(-10)
	if p == nil {
		t.Fatal("expected non-nil pool for negative capacity")
	}
	buf := p.Get()
	if cap(*buf) != 64 {
		t.Errorf("expected default cap 64, got %d", cap(*buf))
	}
}

func TestBufPool_Get(t *testing.T) {
	p := NewBufPool(256)
	buf := p.Get()
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	if cap(*buf) != 256 {
		t.Errorf("expected cap 256, got %d", cap(*buf))
	}
}

func TestBufPool_Put(t *testing.T) {
	p := NewBufPool(128)
	buf := p.Get()

	// Write some data.
	*buf = append(*buf, []byte("hello world")...)

	// Put resets the buffer.
	p.Put(buf)

	// Get again - should get a reset buffer.
	buf2 := p.Get()
	if len(*buf2) != 0 {
		t.Errorf("expected reset buffer, got %d bytes", len(*buf2))
	}
	if cap(*buf2) != 128 {
		t.Errorf("expected cap 128, got %d", cap(*buf2))
	}
}

func TestBufPool_Reuse(t *testing.T) {
	p := NewBufPool(64)
	buf1 := p.Get()
	p.Put(buf1)

	buf2 := p.Get()
	// After Put+Get, the buffer should be reset.
	if len(*buf2) != 0 {
		t.Errorf("expected reused buffer to be reset, got len=%d", len(*buf2))
	}
}

func TestBufPool_MultipleGetPut(t *testing.T) {
	p := NewBufPool(32)
	buffers := make([]*[]byte, 10)
	for i := 0; i < 10; i++ {
		buffers[i] = p.Get()
		*buffers[i] = append(*buffers[i], byte(i))
	}
	for _, buf := range buffers {
		p.Put(buf)
	}
	for i := 0; i < 10; i++ {
		buf := p.Get()
		if len(*buf) != 0 {
			t.Errorf("buffer %d: expected len=0 after Put/Get cycle, got %d", i, len(*buf))
		}
		p.Put(buf)
	}
}
