// Package benchmarks provides benchmarks for TypedCache with various codecs,
// measuring the overhead of type-safe wrappers compared to raw byte access.
//
// Run benchmarks:
//
//	go test -bench=. -benchmem ./benchmarks/
package benchmarks

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2"
	"github.com/os-gomod/cache/v2/internal/serialization"
	"github.com/os-gomod/cache/v2/memory"
)

// ---------------------------------------------------------------------------
// Typed Get - JSON Codec
// ---------------------------------------------------------------------------

// TestStruct is a representative cache value for JSON benchmarks.
type TestStruct struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Score  float64  `json:"score"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

func BenchmarkTypedGet_JSON_16B(b *testing.B) {
	benchmarkTypedGetJSON(b, 16)
}

func BenchmarkTypedGet_JSON_128B(b *testing.B) {
	benchmarkTypedGetJSON(b, 128)
}

func BenchmarkTypedGet_JSON_1KB(b *testing.B) {
	benchmarkTypedGetJSON(b, 1024)
}

func BenchmarkTypedGet_JSON_4KB(b *testing.B) {
	benchmarkTypedGetJSON(b, 4096)
}

func benchmarkTypedGetJSON(b *testing.B, valueSize int) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	// Create a typed cache with JSON codec
	codec := serialization.NewJSONCodec[TestStruct]()
	tc := cache.NewTyped[TestStruct](backend, codec)

	// Pre-populate
	for i := 0; i < b.N+1000; i++ {
		s := TestStruct{
			ID:     int64(i),
			Name:   string(make([]byte, valueSize)),
			Email:  fmt.Sprintf("user%d@test.com", i),
			Score:  float64(i) * 0.1,
			Active: true,
			Tags:   []string{"tag1", "tag2"},
		}
		tc.Set(ctx, fmt.Sprintf("key:%d", i), s, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed Set - JSON Codec
// ---------------------------------------------------------------------------

func BenchmarkTypedSet_JSON_16B(b *testing.B) {
	benchmarkTypedSetJSON(b, 16)
}

func BenchmarkTypedSet_JSON_128B(b *testing.B) {
	benchmarkTypedSetJSON(b, 128)
}

func BenchmarkTypedSet_JSON_1KB(b *testing.B) {
	benchmarkTypedSetJSON(b, 1024)
}

func benchmarkTypedSetJSON(b *testing.B, valueSize int) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	codec := serialization.NewJSONCodec[TestStruct]()
	tc := cache.NewTyped[TestStruct](backend, codec)

	val := TestStruct{
		ID:     1,
		Name:   string(make([]byte, valueSize)),
		Email:  "user@test.com",
		Score:  99.5,
		Active: true,
		Tags:   []string{"tag1", "tag2", "tag3"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := tc.Set(ctx, fmt.Sprintf("key:%d", i), val, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed Get - Raw Codec (baseline)
// ---------------------------------------------------------------------------

func BenchmarkTypedGet_Raw(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[[]byte](backend, &serialization.RawCodec{})

	value := make([]byte, 256)
	for i := 0; i < b.N+1000; i++ {
		tc.Set(ctx, fmt.Sprintf("key:%d", i), value, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed Get - String Codec
// ---------------------------------------------------------------------------

func BenchmarkTypedGet_String_16B(b *testing.B) {
	benchmarkTypedGetString(b, 16)
}

func BenchmarkTypedGet_String_128B(b *testing.B) {
	benchmarkTypedGetString(b, 128)
}

func BenchmarkTypedGet_String_1KB(b *testing.B) {
	benchmarkTypedGetString(b, 1024)
}

func benchmarkTypedGetString(b *testing.B, valueSize int) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[string](backend, &serialization.StringCodec{})

	val := string(make([]byte, valueSize))
	for i := 0; i < b.N+1000; i++ {
		tc.Set(ctx, fmt.Sprintf("key:%d", i), val, 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed Get - Int64 Codec
// ---------------------------------------------------------------------------

func BenchmarkTypedGet_Int64(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[int64](backend, &serialization.Int64Codec{})

	for i := 0; i < b.N+1000; i++ {
		tc.Set(ctx, fmt.Sprintf("key:%d", i), int64(i), 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.Get(ctx, fmt.Sprintf("key:%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed GetOrSet Pattern
// ---------------------------------------------------------------------------

func BenchmarkTypedGetOrSet_Hit(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[string](backend, &serialization.StringCodec{})

	for i := 0; i < b.N+1000; i++ {
		tc.Set(ctx, fmt.Sprintf("key:%d", i), "cached", 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.GetOrSet(ctx, fmt.Sprintf("key:%d", i), func() (string, error) {
			return "loaded", nil
		}, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTypedGetOrSet_Miss(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[string](backend, &serialization.StringCodec{})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.GetOrSet(ctx, fmt.Sprintf("miss:%d", i), func() (string, error) {
			return "loaded", nil
		}, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed SetNX
// ---------------------------------------------------------------------------

func BenchmarkTypedSetNX_Success(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[string](backend, &serialization.StringCodec{})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.SetNX(ctx, fmt.Sprintf("key:%d", i), "value", 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTypedSetNX_Existing(b *testing.B) {
	backend, err := cache.NewMemory(memory.WithMaxEntries(b.N + 1000))
	if err != nil {
		b.Fatal(err)
	}
	defer backend.Close(ctx)

	tc := cache.NewTyped[string](backend, &serialization.StringCodec{})

	for i := 0; i < b.N+1000; i++ {
		tc.Set(ctx, fmt.Sprintf("key:%d", i), "value", 5*time.Minute) // nolint: errcheck
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := tc.SetNX(ctx, fmt.Sprintf("key:%d", i), "new-value", 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Codec Comparison (no cache overhead)
// ---------------------------------------------------------------------------

func BenchmarkCodec_JSON_Encode(b *testing.B) {
	codec := serialization.NewJSONCodec[TestStruct]()
	val := TestStruct{
		ID:     42,
		Name:   "benchmark-user",
		Email:  "bench@test.com",
		Score:  95.5,
		Active: true,
		Tags:   []string{"a", "b", "c", "d", "e"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Encode(val, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodec_JSON_Decode(b *testing.B) {
	codec := serialization.NewJSONCodec[TestStruct]()
	val := TestStruct{
		ID:     42,
		Name:   "benchmark-user",
		Email:  "bench@test.com",
		Score:  95.5,
		Active: true,
		Tags:   []string{"a", "b", "c", "d", "e"},
	}
	encoded, _ := codec.Encode(val, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Decode(encoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodec_String_Encode(b *testing.B) {
	codec := &serialization.StringCodec{}
	val := "benchmark-string-value-128-bytes-padding-padding"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Encode(val, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodec_String_Decode(b *testing.B) {
	codec := &serialization.StringCodec{}
	encoded, _ := codec.Encode("benchmark-string-value-128-bytes-padding-padding", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Decode(encoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodec_Int64_Encode(b *testing.B) {
	codec := &serialization.Int64Codec{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Encode(int64(i), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodec_Int64_Decode(b *testing.B) {
	codec := &serialization.Int64Codec{}
	encoded, _ := codec.Encode(int64(42), nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := codec.Decode(encoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Ensure imports used.
var (
	_ sync.Locker = (*sync.Mutex)(nil)
	_             = memory.Option(nil)
)
