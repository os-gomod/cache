// Package benchmarks provides comprehensive benchmarks for the memory cache
// backend, measuring throughput and latency across various configurations.
//
// Run benchmarks:
//
//	go test -bench=. -benchmem ./benchmarks/
//
// Run specific benchmark:
//
//	go test -bench=BenchmarkMemoryGet -benchmem ./benchmarks/
package benchmarks

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2"
	"github.com/os-gomod/cache/v2/memory"
)

var ctx = context.Background()

// ---------------------------------------------------------------------------
// Memory Get
// ---------------------------------------------------------------------------

func BenchmarkMemoryGet_16B(b *testing.B) {
	benchmarkMemoryGet(b, 16, 64)
}

func BenchmarkMemoryGet_256B(b *testing.B) {
	benchmarkMemoryGet(b, 256, 64)
}

func BenchmarkMemoryGet_1KB(b *testing.B) {
	benchmarkMemoryGet(b, 1024, 64)
}

func BenchmarkMemoryGet_4KB(b *testing.B) {
	benchmarkMemoryGet(b, 4096, 64)
}

func BenchmarkMemoryGet_16KB(b *testing.B) {
	benchmarkMemoryGet(b, 16384, 64)
}

func benchmarkMemoryGet(b *testing.B, valueSize, shardCount int) {
	c, err := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(shardCount),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	// Pre-populate
	value := make([]byte, valueSize)
	for i := 0; i < b.N+1000; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Set
// ---------------------------------------------------------------------------

func BenchmarkMemorySet_16B(b *testing.B) {
	benchmarkMemorySet(b, 16, 64)
}

func BenchmarkMemorySet_256B(b *testing.B) {
	benchmarkMemorySet(b, 256, 64)
}

func BenchmarkMemorySet_1KB(b *testing.B) {
	benchmarkMemorySet(b, 1024, 64)
}

func BenchmarkMemorySet_4KB(b *testing.B) {
	benchmarkMemorySet(b, 4096, 64)
}

func benchmarkMemorySet(b *testing.B, valueSize, shardCount int) {
	c, err := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(shardCount),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	value := make([]byte, valueSize)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := c.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Get+Set (Parallel)
// ---------------------------------------------------------------------------

func BenchmarkMemoryGetSetParallel_1Goroutine(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 1)
}

func BenchmarkMemoryGetSetParallel_4Goroutines(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 4)
}

func BenchmarkMemoryGetSetParallel_8Goroutines(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 8)
}

func BenchmarkMemoryGetSetParallel_16Goroutines(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 16)
}

func BenchmarkMemoryGetSetParallel_32Goroutines(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 32)
}

func BenchmarkMemoryGetSetParallel_64Goroutines(b *testing.B) {
	benchmarkMemoryGetSetParallel(b, 64)
}

func benchmarkMemoryGetSetParallel(b *testing.B, goroutines int) {
	c, err := memory.New(
		memory.WithMaxEntries(100000),
		memory.WithShardCount(64),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	// Pre-populate
	value := make([]byte, 256)
	for i := 0; i < 100000; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key:%d", i%100000)
			if i%2 == 0 {
				c.Get(ctx, key) // nolint: errcheck
			} else {
				c.Set(ctx, key, value, 5*time.Minute) // nolint: errcheck
			}
			i++
		}
	})
}

// ---------------------------------------------------------------------------
// Memory GetMulti
// ---------------------------------------------------------------------------

func BenchmarkMemoryGetMulti_10Keys(b *testing.B) {
	benchmarkMemoryGetMulti(b, 10)
}

func BenchmarkMemoryGetMulti_50Keys(b *testing.B) {
	benchmarkMemoryGetMulti(b, 50)
}

func BenchmarkMemoryGetMulti_100Keys(b *testing.B) {
	benchmarkMemoryGetMulti(b, 100)
}

func BenchmarkMemoryGetMulti_500Keys(b *testing.B) {
	benchmarkMemoryGetMulti(b, 500)
}

func benchmarkMemoryGetMulti(b *testing.B, keyCount int) {
	c, err := memory.New(
		memory.WithMaxEntries(keyCount+1000),
		memory.WithShardCount(64),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	// Pre-populate
	value := make([]byte, 256)
	keys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		keys[i] = fmt.Sprintf("key:%d", i)
		c.Set(ctx, keys[i], value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.GetMulti(ctx, keys...)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory TTL
// ---------------------------------------------------------------------------

func BenchmarkMemoryTTL(b *testing.B) {
	c, err := memory.New(
		memory.WithMaxEntries(b.N+100),
		memory.WithShardCount(64),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	for i := 0; i < b.N+100; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), []byte("val"), 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.TTL(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Exists
// ---------------------------------------------------------------------------

func BenchmarkMemoryExists_Hit(b *testing.B) {
	c, err := memory.New(memory.WithMaxEntries(b.N + 100))
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	for i := 0; i < b.N+100; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), []byte("val"), 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Exists(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryExists_Miss(b *testing.B) {
	c, err := memory.New(memory.WithMaxEntries(b.N + 100))
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Exists(ctx, fmt.Sprintf("nonexistent:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Increment
// ---------------------------------------------------------------------------

func BenchmarkMemoryIncrement(b *testing.B) {
	c, err := memory.New(memory.WithMaxEntries(b.N + 100))
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	for i := 0; i < b.N+100; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), []byte("0"), 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Increment(ctx, fmt.Sprintf("key:%d", i), 1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Shard Scaling
// ---------------------------------------------------------------------------

func BenchmarkMemoryGet_1Shard(b *testing.B) {
	benchmarkMemoryGetShards(b, 1)
}

func BenchmarkMemoryGet_4Shards(b *testing.B) {
	benchmarkMemoryGetShards(b, 4)
}

func BenchmarkMemoryGet_16Shards(b *testing.B) {
	benchmarkMemoryGetShards(b, 16)
}

func BenchmarkMemoryGet_64Shards(b *testing.B) {
	benchmarkMemoryGetShards(b, 64)
}

func BenchmarkMemoryGet_128Shards(b *testing.B) {
	benchmarkMemoryGetShards(b, 128)
}

func benchmarkMemoryGetShards(b *testing.B, shardCount int) {
	c, err := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(shardCount),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	value := make([]byte, 256)
	for i := 0; i < b.N+1000; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory Key Size Variation
// ---------------------------------------------------------------------------

func BenchmarkMemoryGet_ShortKeys(b *testing.B) {
	benchmarkMemoryGetKeySize(b, 8)
}

func BenchmarkMemoryGet_MediumKeys(b *testing.B) {
	benchmarkMemoryGetKeySize(b, 64)
}

func BenchmarkMemoryGet_LongKeys(b *testing.B) {
	benchmarkMemoryGetKeySize(b, 256)
}

func benchmarkMemoryGetKeySize(b *testing.B, keyLen int) {
	c, err := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	value := make([]byte, 256)
	for i := 0; i < b.N+1000; i++ {
		key := fmt.Sprintf("%0*d", keyLen, i%10000)
		if len(key) > keyLen {
			key = key[:keyLen]
		}
		c.Set(ctx, key, value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%0*d", keyLen, i%10000)
		if len(key) > keyLen {
			key = key[:keyLen]
		}
		_, err := c.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Ensure cache import and sync are used.
var (
	_             = strconv.Itoa(0)
	_ sync.Locker = (*sync.Mutex)(nil)
	_             = cache.WithMaxEntries(0)
)
