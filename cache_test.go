package cache

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/memory"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestBackend(t *testing.T) *memory.Cache {
	t.Helper()
	c, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// ---------------------------------------------------------------------------
// Get[T]
// ---------------------------------------------------------------------------

func TestGet_String(t *testing.T) {
	b := newTestBackend(t)
	cd := codec.StringCodec{}

	_ = b.Set(context.Background(), "greet", []byte("hello"), 0)

	val, err := Get[string](context.Background(), b, "greet", cd)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get = %q, want %q", val, "hello")
	}
}

func TestGet_Int64(t *testing.T) {
	b := newTestBackend(t)
	cd := codec.Int64Codec{}

	_ = b.Set(context.Background(), "num", []byte("42"), 0)

	val, err := Get[int64](context.Background(), b, "num", cd)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 42 {
		t.Errorf("Get = %d, want 42", val)
	}
}

func TestGet_Miss(t *testing.T) {
	b := newTestBackend(t)

	_, err := Get[string](context.Background(), b, "missing", codec.StringCodec{})
	if err == nil {
		t.Error("expected error on cache miss, got nil")
	}
}

// ---------------------------------------------------------------------------
// Set[T]
// ---------------------------------------------------------------------------

func TestSet_String(t *testing.T) {
	b := newTestBackend(t)

	if err := Set[string](context.Background(), b, "greet", "hello", 0, codec.StringCodec{}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	data, err := b.Get(context.Background(), "greet")
	if err != nil {
		t.Fatalf("raw Get: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("stored data = %q, want %q", data, "hello")
	}
}

func TestSet_Int64(t *testing.T) {
	b := newTestBackend(t)

	if err := Set[int64](context.Background(), b, "num", 99, 0, codec.Int64Codec{}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	data, err := b.Get(context.Background(), "num")
	if err != nil {
		t.Fatalf("raw Get: %v", err)
	}
	if string(data) != "99" {
		t.Errorf("stored data = %q, want %q", data, "99")
	}
}

// ---------------------------------------------------------------------------
// Cross-codec roundtrip
// ---------------------------------------------------------------------------

func TestRoundTrip_Float64(t *testing.T) {
	b := newTestBackend(t)
	cd := codec.Float64Codec{}

	vals := []float64{0.0, 1.0, 3.14159, -2.71828}
	for _, v := range vals {
		key := "f-test"
		if err := Set(context.Background(), b, key, v, 0, cd); err != nil {
			t.Fatalf("Set(%g): %v", v, err)
		}
		got, err := Get[float64](context.Background(), b, key, cd)
		if err != nil {
			t.Fatalf("Get(%g): %v", v, err)
		}
		if got != v {
			t.Errorf("roundtrip: got %g, want %g", got, v)
		}
	}
}

func TestRoundTrip_Raw(t *testing.T) {
	b := newTestBackend(t)
	cd := codec.RawCodec{}

	data := []byte("binary-payload\x00\x01\x02")
	if err := Set[[]byte](context.Background(), b, "bin", data, 0, cd); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get[[]byte](context.Background(), b, "bin", cd)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("roundtrip: got %q, want %q", got, data)
	}
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func TestMemory(t *testing.T) {
	c, err := Memory()
	if err != nil {
		t.Fatalf("Memory(): %v", err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	if err := c.Set(context.Background(), "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := c.Get(context.Background(), "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "v" {
		t.Errorf("Get = %q, want %q", val, "v")
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkGet_String(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	cd := codec.StringCodec{}
	_ = mc.Set(context.Background(), "bench", []byte("value"), 0)

	for b.Loop() {
		_, _ = Get[string](context.Background(), mc, "bench", cd)
	}
}

func BenchmarkSet_String(b *testing.B) {
	mc, _ := memory.New()
	defer func() { _ = mc.Close(context.Background()) }()
	cd := codec.StringCodec{}

	for b.Loop() {
		_ = Set[string](context.Background(), mc, "bench", "value", 0, cd)
	}
}
