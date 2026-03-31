// Package builder_test provides tests for fluent cache builders.
package builder_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/os-gomod/cache/builder"
	"github.com/os-gomod/cache/config"
)

func TestNewBuilder(t *testing.T) {
	ctx := context.Background()
	b := builder.New(ctx)
	if b == nil {
		t.Fatal("builder is nil")
	}
}

func TestMemoryBuilder_Build(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Memory().
		MaxEntries(100).
		MaxMemoryMB(50).
		TTL(time.Hour).
		CleanupInterval(5 * time.Minute).
		ShardCount(64).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
	if cache == nil {
		t.Fatal("cache is nil")
	}
}

func TestMemoryBuilder_WithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultMemory()
	cfg.MaxEntries = 200

	cache, err := builder.New(ctx).Memory().
		WithConfig(cfg).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
	if cache == nil {
		t.Fatal("cache is nil")
	}
}

func TestMemoryBuilder_EvictionPolicies(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		build func() *builder.MemoryBuilder
	}{
		{"LRU", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().LRU() }},
		{"LFU", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().LFU() }},
		{"FIFO", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().FIFO() }},
		{"LIFO", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().LIFO() }},
		{"MRU", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().MRU() }},
		{"Random", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().Random() }},
		{"TinyLFU", func() *builder.MemoryBuilder { return builder.New(ctx).Memory().TinyLFU() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := tt.build().MaxEntries(10).Build()
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}
			t.Cleanup(func() { _ = cache.Close(ctx) })
		})
	}
}

func TestMemoryBuilder_OnEviction(t *testing.T) {
	ctx := context.Background()
	evicted := false

	cache, err := builder.New(ctx).Memory().
		MaxEntries(2).
		OnEviction(func(key, reason string) {
			evicted = true
			fmt.Printf("   🔥 Evicted: %s (reason: %s)\n", key, reason)
		}).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })

	// Add more entries than max to trigger eviction
	_ = cache.Set(ctx, "key1", []byte("value1"), 0)
	_ = cache.Set(ctx, "key2", []byte("value2"), 0)
	_ = cache.Set(ctx, "key3", []byte("value3"), 0)

	if !evicted {
		t.Error("eviction callback not called")
	}
}

func TestRedisBuilder_Build(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Redis().
		Addr("localhost:6379").
		DB(15).
		PoolSize(10).
		TTL(time.Hour).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
	if cache == nil {
		t.Fatal("cache is nil")
	}
}

func TestRedisBuilder_WithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultRedis()
	cfg.Addr = "localhost:6379"
	cfg.DB = 15

	cache, err := builder.New(ctx).Redis().
		WithConfig(cfg).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
	if cache == nil {
		t.Fatal("cache is nil")
	}
}

func TestRedisBuilder_Timeouts(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Redis().
		Addr("localhost:6379").
		DB(15).
		DialTimeout(5 * time.Second).
		ReadTimeout(3 * time.Second).
		WriteTimeout(3 * time.Second).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestRedisBuilder_StampedeProtection(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Redis().
		Addr("localhost:6379").
		DB(15).
		DistributedStampedeProtection(true).
		StampedeLockTTL(5 * time.Second).
		StampedeWaitTimeout(2 * time.Second).
		StampedeRetryInterval(100 * time.Millisecond).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestLayeredBuilder_Build(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Layered().
		L1MaxEntries(100).
		L1MaxMemoryMB(50).
		L1TTL(5 * time.Minute).
		L2Addr("localhost:6379").
		L2DB(15).
		PromoteOnHit(true).
		NegativeTTL(30 * time.Second).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
	if cache == nil {
		t.Fatal("cache is nil")
	}
}

func TestLayeredBuilder_WriteBack(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Layered().
		L1MaxEntries(100).
		L2Addr("localhost:6379").
		L2DB(15).
		WriteBack(true).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestLayeredBuilder_Sync(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.New(ctx).Layered().
		L1MaxEntries(100).
		L2Addr("localhost:6379").
		L2DB(15).
		SyncEnabled(true).
		SyncChannel("cache_invalidations").
		SyncBufferSize(100).
		Build()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestLayeredBuilder_EvictionPolicies(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		build func() *builder.LayeredBuilder
	}{
		{"L1LRU", func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1LRU() }},
		{"L1LFU", func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1LFU() }},
		{"L1FIFO", func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1FIFO() }},
		{"L1LIFO", func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1LIFO() }},
		{"L1MRU", func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1MRU() }},
		{
			"L1Random",
			func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1Random() },
		},
		{
			"L1TinyLFU",
			func() *builder.LayeredBuilder { return builder.New(ctx).Layered().L1TinyLFU() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := tt.build().
				L1MaxEntries(10).
				L2Addr("localhost:6379").
				L2DB(15).
				Build()
			if err != nil {
				t.Skip("Redis not available, skipping test")
			}
			t.Cleanup(func() { _ = cache.Close(ctx) })
		})
	}
}

func TestDefaultMemory(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.DefaultMemory(ctx)
	if err != nil {
		t.Fatalf("DefaultMemory failed: %v", err)
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestMemoryWithSize(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.MemoryWithSize(ctx, 100)
	if err != nil {
		t.Fatalf("MemoryWithSize failed: %v", err)
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestMemoryWithTTL(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.MemoryWithTTL(ctx, time.Hour)
	if err != nil {
		t.Fatalf("MemoryWithTTL failed: %v", err)
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestDefaultRedis(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.DefaultRedis(ctx)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestRedisWithAddress(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.RedisWithAddress(ctx, "localhost:6379")
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestRedisWithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultRedis()
	cfg.Addr = "localhost:6379"
	cfg.DB = 15

	cache, err := builder.RedisWithConfig(ctx, cfg)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestDefaultLayered(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.DefaultLayered(ctx)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestLayeredWithRedis(t *testing.T) {
	ctx := context.Background()

	cache, err := builder.LayeredWithRedis(ctx, "localhost:6379")
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestLayeredWithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultLayered()
	cfg.L2Config.Addr = "localhost:6379"

	cache, err := builder.LayeredWithConfig(ctx, cfg)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if cache == nil {
		t.Fatal("cache is nil")
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestGenericBuilder_Immutable(t *testing.T) {
	ctx := context.Background()

	b1 := builder.New(ctx).Memory().MaxEntries(100)
	b2 := b1.MaxEntries(200)

	if b1 == b2 {
		t.Error("builders should be immutable")
	}

	c1, err := b1.Build()
	if err != nil {
		t.Fatalf("b1 build failed: %v", err)
	}
	t.Cleanup(func() { _ = c1.Close(ctx) })

	c2, err := b2.Build()
	if err != nil {
		t.Fatalf("b2 build failed: %v", err)
	}
	t.Cleanup(func() { _ = c2.Close(ctx) })
}

func TestGenericBuilder_WithConfigNil(t *testing.T) {
	ctx := context.Background()

	b := builder.New(ctx).Memory().WithConfig(nil)
	cache, err := b.Build()
	if err != nil {
		t.Fatalf("Build with nil config failed: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func TestGenericBuilder_MustBuild(t *testing.T) {
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustBuild should panic on error")
		}
	}()

	// This should panic because validation fails before any cache is built.
	_ = builder.New(ctx).Redis().DB(-1).MustBuild()
}

func TestKeyBuilder_NewKey(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr bool
	}{
		{"valid prefix", "cache", false},
		{"empty prefix", "", true},
		{"prefix with spaces", "cache ", true},
		{"prefix with control char", "cache\n", true},
		{"long prefix", string(make([]byte, 300)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb, err := builder.NewKey(tt.prefix)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kb == nil {
				t.Fatal("key builder is nil")
			}
			if kb.Prefix() != tt.prefix {
				t.Errorf("prefix = %s, want %s", kb.Prefix(), tt.prefix)
			}
		})
	}
}

func TestKeyBuilder_MustNewKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustNewKey should panic on invalid prefix")
		}
	}()

	_ = builder.MustNewKey("")
}

func TestKeyBuilder_Add(t *testing.T) {
	kb, err := builder.NewKey("cache")
	if err != nil {
		t.Fatalf("NewKey failed: %v", err)
	}

	// Add valid segment
	kb2, err := kb.Add("user")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if kb2.Depth() != 1 {
		t.Errorf("depth = %d, want 1", kb2.Depth())
	}

	// Add another segment
	kb3, err := kb2.Add("123")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if kb3.Depth() != 2 {
		t.Errorf("depth = %d, want 2", kb3.Depth())
	}

	// Check final key
	key := kb3.Build()
	expected := "cache:user:123"
	if key != expected {
		t.Errorf("key = %s, want %s", key, expected)
	}
}

func TestKeyBuilder_AddInvalid(t *testing.T) {
	kb, err := builder.NewKey("cache")
	if err != nil {
		t.Fatalf("NewKey failed: %v", err)
	}

	tests := []struct {
		name string
		part string
	}{
		{"empty", ""},
		{"spaces", "user "},
		{"control char", "user\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errAdd := kb.Add(tt.part)
			if errAdd == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestKeyBuilder_MustAdd(t *testing.T) {
	kb, err := builder.NewKey("cache")
	if err != nil {
		t.Fatalf("NewKey failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustAdd should panic on invalid segment")
		}
	}()

	_ = kb.MustAdd("")
}

func TestKeyBuilder_Build(t *testing.T) {
	kb, _ := builder.NewKey("cache")
	kb, _ = kb.Add("user")
	kb, _ = kb.Add("123")

	key := kb.Build()
	expected := "cache:user:123"
	if key != expected {
		t.Errorf("key = %s, want %s", key, expected)
	}
}

func TestKeyBuilder_TryBuild(t *testing.T) {
	kb, _ := builder.NewKey("cache")

	// Build valid key
	key, err := kb.TryBuild()
	if err != nil {
		t.Fatalf("TryBuild failed: %v", err)
	}
	if key != "cache" {
		t.Errorf("key = %s, want cache", key)
	}

	// Build very long key
	longPrefix := string(make([]byte, 500))
	kb, _ = builder.NewKey(longPrefix)

	_, err = kb.TryBuild()
	if err == nil {
		t.Error("expected error for long key, got nil")
	}
	if err != nil {
		t.Logf("got error: %v", err)
	}
}

func TestKeyBuilder_Chain(t *testing.T) {
	kb, _ := builder.NewKey("api")
	kb, _ = kb.Add("v1")
	kb, _ = kb.Add("users")
	kb, _ = kb.Add("123")

	key := kb.Build()
	expected := "api:v1:users:123"
	if key != expected {
		t.Errorf("key = %s, want %s", key, expected)
	}

	if kb.Prefix() != "api" {
		t.Errorf("prefix = %s, want api", kb.Prefix())
	}
	if kb.Depth() != 3 {
		t.Errorf("depth = %d, want 3", kb.Depth())
	}
}

func TestKeyBuilder_LengthLimit(t *testing.T) {
	// Create a valid prefix that's close to the prefix-length limit.
	prefix := strings.Repeat("p", 250)
	kb, err := builder.NewKey(prefix)
	if err != nil {
		t.Fatalf("NewKey failed: %v", err)
	}

	// Adding a long but valid segment should fail due to the overall key limit.
	_, err = kb.Add(strings.Repeat("s", 270))
	if err == nil {
		t.Error("expected error for exceeding length limit")
	}
}

func TestBuilder_Chaining(t *testing.T) {
	ctx := context.Background()

	// Test that builders can be chained
	cache, err := builder.New(ctx).
		Memory().
		MaxEntries(100).
		TTL(time.Hour).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close(ctx) })
}

func BenchmarkMemoryBuilder_Build(b *testing.B) {
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		cache, err := builder.New(ctx).Memory().
			MaxEntries(1000).
			MaxMemoryMB(100).
			TTL(time.Hour).
			Build()
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
		cache.Close(ctx)
	}
}

func BenchmarkKeyBuilder_Build(b *testing.B) {
	kb, _ := builder.NewKey("cache")
	kb, _ = kb.Add("user")
	kb, _ = kb.Add("123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = kb.Build()
	}
}

func BenchmarkKeyBuilder_Chain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		kb, _ := builder.NewKey("api")
		kb, _ = kb.Add("v1")
		kb, _ = kb.Add("users")
		kb, _ = kb.Add("123")
		_ = kb.Build()
	}
}
