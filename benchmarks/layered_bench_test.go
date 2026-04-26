// Package benchmarks provides benchmarks for the layered cache backend,
// measuring throughput across L1 hit, L2 hit, and miss scenarios.
//
// Run benchmarks:
//
//	go test -bench=. -benchmem ./benchmarks/
package benchmarks

import (
	"fmt"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/layered"
	"github.com/os-gomod/cache/v2/memory"
)

// ---------------------------------------------------------------------------
// Layered Get - L1 Hit (fast path)
// ---------------------------------------------------------------------------

func BenchmarkLayeredGet_L1Hit_256B(b *testing.B) {
	benchmarkLayeredGetL1Hit(b, 256)
}

func BenchmarkLayeredGet_L1Hit_1KB(b *testing.B) {
	benchmarkLayeredGetL1Hit(b, 1024)
}

func BenchmarkLayeredGet_L1Hit_4KB(b *testing.B) {
	benchmarkLayeredGetL1Hit(b, 4096)
}

func benchmarkLayeredGetL1Hit(b *testing.B, valueSize int) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	// Pre-populate both tiers
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
// Layered Get - L2 Hit (L1 miss, promotion from L2)
// ---------------------------------------------------------------------------

func BenchmarkLayeredGet_L2Hit_256B(b *testing.B) {
	benchmarkLayeredGetL2Hit(b, 256)
}

func BenchmarkLayeredGet_L2Hit_1KB(b *testing.B) {
	benchmarkLayeredGetL2Hit(b, 1024)
}

func BenchmarkLayeredGet_L2Hit_4KB(b *testing.B) {
	benchmarkLayeredGetL2Hit(b, 4096)
}

func benchmarkLayeredGetL2Hit(b *testing.B, valueSize int) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	// Pre-populate L2 only (simulate L1 miss scenario)
	value := make([]byte, valueSize)
	for i := 0; i < b.N+1000; i++ {
		c.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute) // nolint: errcheck
	}

	// Access once to promote to L1
	for i := 0; i < b.N+1000; i++ {
		c.Get(ctx, fmt.Sprintf("key:%d", i)) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	// All accesses should be L1 hits now (after promotion)
	for i := 0; i < b.N; i++ {
		_, err := c.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Layered Get - Full Miss (neither tier has the key)
// ---------------------------------------------------------------------------

func BenchmarkLayeredGet_Miss(b *testing.B) {
	l1, _ := memory.New(
		memory.WithMaxEntries(10000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(10000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.Get(ctx, fmt.Sprintf("nonexistent:%d", i))
		if err == nil {
			b.Fatal("expected error for nonexistent key")
		}
	}
}

// ---------------------------------------------------------------------------
// Layered Set
// ---------------------------------------------------------------------------

func BenchmarkLayeredSet_256B(b *testing.B) {
	benchmarkLayeredSet(b, 256)
}

func BenchmarkLayeredSet_1KB(b *testing.B) {
	benchmarkLayeredSet(b, 1024)
}

func BenchmarkLayeredSet_4KB(b *testing.B) {
	benchmarkLayeredSet(b, 4096)
}

func benchmarkLayeredSet(b *testing.B, valueSize int) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
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
// Layered SetMulti
// ---------------------------------------------------------------------------

func BenchmarkLayeredSetMulti_10Keys(b *testing.B) {
	benchmarkLayeredSetMulti(b, 10)
}

func BenchmarkLayeredSetMulti_50Keys(b *testing.B) {
	benchmarkLayeredSetMulti(b, 50)
}

func BenchmarkLayeredSetMulti_100Keys(b *testing.B) {
	benchmarkLayeredSetMulti(b, 100)
}

func benchmarkLayeredSetMulti(b *testing.B, keyCount int) {
	l1, _ := memory.New(
		memory.WithMaxEntries(100000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(100000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(ctx)

	value := make([]byte, 256)
	keys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		keys[i] = fmt.Sprintf("key:%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		items := make(map[string][]byte, keyCount)
		for _, k := range keys {
			items[k] = value
		}
		err := c.SetMulti(ctx, items, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Layered Delete
// ---------------------------------------------------------------------------

func BenchmarkLayeredDelete(b *testing.B) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+1000),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
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
		err := c.Delete(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Layered Exists
// ---------------------------------------------------------------------------

func BenchmarkLayeredExists_Hit(b *testing.B) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+100),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+100),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
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
		_, err := c.Exists(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLayeredExists_Miss(b *testing.B) {
	l1, _ := memory.New(
		memory.WithMaxEntries(b.N+100),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	l2, _ := memory.New(
		memory.WithMaxEntries(b.N+100),
		memory.WithShardCount(64),
		memory.WithCleanupInterval(0),
	)
	c, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
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
