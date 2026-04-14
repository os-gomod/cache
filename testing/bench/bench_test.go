// Package bench provides performance benchmarks for the cache library.
// These benchmarks measure hot-path performance and track overhead from
// resilience, codec, and manager layers.
//
// Run with: go test -bench=. -benchmem ./testing/bench/
package bench

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/layer"
	"github.com/os-gomod/cache/manager"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
)

// ---------------------------------------------------------------------------
// Memory Backend Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkMemory_Get_Hit(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	_ = c.Set(context.Background(), "bench-key", []byte("bench-value"), 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "bench-key")
	}
}

func BenchmarkMemory_Get_Miss(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "nonexistent-key")
	}
}

func BenchmarkMemory_Set(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	ctx := context.Background()
	val := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Set(ctx, "set-key", val, 0)
	}
}

func BenchmarkMemory_GetOrSet_NoStampede(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	_ = c.Set(context.Background(), "gos-key", []byte("present"), time.Hour)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.GetOrSet(ctx, "gos-key",
			func() ([]byte, error) { return []byte("computed"), nil },
			time.Hour)
	}
}

// ---------------------------------------------------------------------------
// Redis Backend Benchmarks (using miniredis)
// ---------------------------------------------------------------------------

func BenchmarkRedis_Get_Hit(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	c, err := redis.New(redis.WithAddress(mr.Addr()))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	_ = c.Set(context.Background(), "bench-key", []byte("bench-value"), 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(ctx, "bench-key")
	}
}

// ---------------------------------------------------------------------------
// Layered Cache Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLayered_Get_L1Hit(b *testing.B) {
	l1, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = l1.Close(context.Background()) }()

	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	l2, err := redis.New(redis.WithAddress(mr.Addr()))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = l2.Close(context.Background()) }()

	lc, err := layer.NewWithBackends(context.Background(), l1, l2)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = lc.Close(context.Background()) }()

	// Pre-populate L1 so we measure L1 hit path.
	_ = l1.Set(context.Background(), "l1-key", []byte("from-l1"), time.Hour)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lc.Get(ctx, "l1-key")
	}
}

func BenchmarkLayered_Get_L2Hit_Promote(b *testing.B) {
	l1, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = l1.Close(context.Background()) }()

	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	l2, err := redis.New(redis.WithAddress(mr.Addr()))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = l2.Close(context.Background()) }()

	cfg := config.DefaultLayered()
	cfg.PromoteOnHit = true
	lc, err := layer.NewFromConfig(cfg, l1, l2)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = lc.Close(context.Background()) }()

	// Put data in L2 only (not in L1) so Get triggers L2 hit + promotion.
	_ = l2.Set(context.Background(), "l2-key", []byte("from-l2"), time.Hour)
	// Make sure L1 does NOT have the key.
	_ = l1.Delete(context.Background(), "l2-key")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lc.Get(ctx, "l2-key")
		// Clean L1 so next iteration also triggers L2 hit + promotion.
		_ = l1.Delete(context.Background(), "l2-key")
	}
}

// ---------------------------------------------------------------------------
// Manager Overhead Benchmark
// ---------------------------------------------------------------------------

func BenchmarkManager_Get_Overhead(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	m, err := manager.New(
		manager.WithDefaultBackend(c),
		manager.WithPolicy(resilience.NoRetryPolicy()),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = m.Close(context.Background()) }()

	_ = c.Set(context.Background(), "mgr-key", []byte("mgr-val"), 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Get(ctx, "mgr-key")
	}
}

// ---------------------------------------------------------------------------
// Resilience Policy Overhead Benchmark
// ---------------------------------------------------------------------------

func BenchmarkResiliencePolicy_Execute_Success(b *testing.B) {
	c, err := memory.New()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	policy := resilience.NoRetryPolicy() // minimal overhead
	rc := resilience.NewCacheWithPolicy(c, policy)
	defer func() { _ = rc.Close(context.Background()) }()

	_ = rc.Set(context.Background(), "res-key", []byte("res-val"), 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.Get(ctx, "res-key")
	}
}

// ---------------------------------------------------------------------------
// Codec Benchmarks
// ---------------------------------------------------------------------------

type benchStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Flag  bool   `json:"flag"`
}

func BenchmarkJSONCodec_Encode_SmallStruct(b *testing.B) {
	c := codec.NewJSONCodec[benchStruct]()
	v := benchStruct{Name: "test", Value: 42, Flag: true}
	var buf []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf, _ = c.Encode(v, buf[:0])
	}
	_ = buf
}

func BenchmarkStringCodec_Decode_ZeroAlloc(b *testing.B) {
	c := codec.StringCodec{}
	data := []byte("hello, world!")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Decode(data)
	}
}

func BenchmarkInt64Codec_Roundtrip(b *testing.B) {
	c := codec.Int64Codec{}
	var buf []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := c.Encode(int64(i), buf[:0])
		_, _ = c.Decode(encoded)
	}
}

func BenchmarkFloat64Codec_Roundtrip(b *testing.B) {
	c := codec.Float64Codec{}
	var buf []byte
	v := 3.14159265358979

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := c.Encode(v, buf[:0])
		_, _ = c.Decode(encoded)
	}
}
