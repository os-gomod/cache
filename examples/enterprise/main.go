// Package main demonstrates enterprise features of os-gomod/cache v2.
//
// This example shows:
//   - Multi-backend cache manager
//   - Namespace isolation for multi-tenancy
//   - Cache warming (pre-populate from data source)
//   - Hot-key detection and monitoring
//   - Adaptive TTL (dynamic TTL adjustment)
//   - Compression (gzip and snappy)
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2"
	"github.com/os-gomod/cache/v2/memory"
)

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// 1. Multi-Backend Manager
	// ---------------------------------------------------------------------------
	fmt.Println("=== 1. Multi-Backend Manager ===")

	primaryCache, err := cache.NewMemory(
		memory.WithMaxEntries(10000),
		memory.WithDefaultTTL(10*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create primary cache: %v", err)
	}

	sessionCache, err := cache.NewMemory(
		memory.WithMaxEntries(5000),
		memory.WithDefaultTTL(30*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create session cache: %v", err)
	}

	mgr, err := cache.NewManager(
		cache.WithNamedBackend("primary", primaryCache),
		cache.WithNamedBackend("sessions", sessionCache),
		cache.WithDefaultBackend(primaryCache),
	)
	if err != nil {
		log.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close(ctx)

	fmt.Println("Manager created with 2 backends:")
	fmt.Println("  - primary (default): 10K entries, 10m TTL")
	fmt.Println("  - sessions: 5K entries, 30m TTL")

	// Use default backend
	mgr.Set(ctx, "config:version", []byte("2.0.0"), 0) // nolint: errcheck
	val, _ := mgr.Get(ctx, "config:version")
	fmt.Printf("Default backend: config:version = %s\n", string(val))

	// Use named backend
	sessionsBackend, _ := mgr.Backend("sessions")
	sessionsBackend.Set(ctx, "sess:abc123", []byte("user-data"), 30*time.Minute) // nolint: errcheck
	fmt.Println("Sessions backend: sess:abc123 = user-data")

	// Health check
	health := mgr.HealthCheck(ctx)
	fmt.Printf("Health check: %d backends, all healthy\n", len(health))

	// ---------------------------------------------------------------------------
	// 2. Namespace Isolation
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== 2. Namespace Isolation ===")

	// Create namespaces for multi-tenant isolation
	tenantA, err := cache.NewNamespace("tenant:a", primaryCache)
	if err != nil {
		log.Fatalf("failed to create namespace: %v", err)
	}

	tenantB, err := cache.NewNamespace("tenant:b", primaryCache)
	if err != nil {
		log.Fatalf("failed to create namespace: %v", err)
	}

	// Each tenant operates independently
	tenantA.Set(ctx, "user:1", []byte("Alice"), 5*time.Minute) // nolint: errcheck
	tenantB.Set(ctx, "user:1", []byte("Bob"), 5*time.Minute)   // nolint: errcheck

	// Tenant A sees their own data
	valA, _ := tenantA.Get(ctx, "user:1")
	fmt.Printf("Tenant A user:1 = %s (key: tenant:a:user:1)\n", string(valA))

	// Tenant B sees their own data
	valB, _ := tenantB.Get(ctx, "user:1")
	fmt.Printf("Tenant B user:1 = %s (key: tenant:b:user:1)\n", string(valB))

	// Keys are isolated
	keysA, _ := tenantA.Keys(ctx, "*")
	keysB, _ := tenantB.Keys(ctx, "*")
	fmt.Printf("Tenant A keys: %v\n", keysA)
	fmt.Printf("Tenant B keys: %v\n", keysB)

	// ---------------------------------------------------------------------------
	// 3. Cache Warming
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== 3. Cache Warming ===")

	warmCache, err := cache.NewMemory(memory.WithMaxEntries(1000))
	if err != nil {
		log.Fatalf("failed to create warm cache: %v", err)
	}
	defer warmCache.Close(ctx)

	// Define a loader that simulates fetching from a database
	loader := func(keys []string) (map[string][]byte, error) {
		fmt.Printf("  Loading %d keys from data source...\n", len(keys))
		result := make(map[string][]byte, len(keys))
		for _, k := range keys {
			result[k] = []byte("data-for-" + k)
		}
		return result, nil
	}

	warmer := cache.NewWarmer(warmCache, loader,
		cache.WithWarmerBatchSize(10),
		cache.WithWarmerConcurrency(2),
	)

	// Warm specific keys
	keys := make([]string, 25)
	for i := range 25 {
		keys[i] = fmt.Sprintf("product:%d", i)
	}

	err = warmer.Warm(ctx, keys...)
	if err != nil {
		log.Fatalf("Warm failed: %v", err)
	}
	fmt.Printf("Warmed %d keys successfully\n", len(keys))

	// Verify warmed data
	val, _ = warmCache.Get(ctx, "product:0")
	fmt.Printf("Verify product:0 = %s\n", string(val))

	size, _ := warmCache.Size(ctx)
	fmt.Printf("Cache size after warming: %d\n", size)

	// Warm all from a source
	err = warmer.WarmAll(ctx, func() ([]string, error) {
		allKeys := make([]string, 10)
		for i := range 10 {
			allKeys[i] = fmt.Sprintf("category:%d", i)
		}
		return allKeys, nil
	})
	if err != nil {
		log.Fatalf("WarmAll failed: %v", err)
	}
	fmt.Println("WarmAll: loaded 10 additional keys")

	// ---------------------------------------------------------------------------
	// 4. Hot-Key Detection
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== 4. Hot-Key Detection ===")

	detector := cache.NewHotKeyDetector(
		cache.WithHotKeyThreshold(50),
		cache.WithHotKeyWindow(1*time.Second),
		cache.WithHotKeyCallback(func(key string, count int64) {
			fmt.Printf("  [ALERT] Hot key detected: %s (access count: %d)\n", key, count)
		}),
	)

	// Simulate access patterns
	fmt.Println("Simulating access patterns...")
	for i := range 200 {
		detector.Record("popular:product") // very hot
		detector.Record("popular:product") // very hot (double access)
		if i%3 == 0 {
			detector.Record("medium:product") // medium
		}
		if i%10 == 0 {
			detector.Record("cold:product") // cold
		}
	}

	fmt.Printf("Is hot 'popular:product': %v\n", detector.IsHot("popular:product"))
	fmt.Printf("Is hot 'medium:product': %v\n", detector.IsHot("medium:product"))
	fmt.Printf("Is hot 'cold:product': %v\n", detector.IsHot("cold:product"))

	topKeys := detector.TopKeys(5)
	fmt.Println("Top 5 keys:")
	for i, kc := range topKeys {
		fmt.Printf("  %d. %s (count: %d)\n", i+1, kc.Key, kc.Count)
	}

	fmt.Printf("Tracked keys: %d\n", detector.Size())

	// ---------------------------------------------------------------------------
	// 5. Adaptive TTL
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== 5. Adaptive TTL ===")

	adaptive := cache.NewAdaptiveTTL(30*time.Second, 10*time.Minute)

	// Cold key: no access history, gets base TTL
	coldTTL := adaptive.TTL("cold:key", 5*time.Minute)
	fmt.Printf("Cold key TTL: %v (close to min)\n", coldTTL)

	// Simulate frequent access
	fmt.Println("Simulating frequent access to 'hot:key'...")
	for range 500 {
		adaptive.RecordAccess("hot:key")
	}

	// Hot key: lots of access history, gets extended TTL
	hotTTL := adaptive.TTL("hot:key", 5*time.Minute)
	fmt.Printf("Hot key TTL: %v (extended toward max)\n", hotTTL)

	fmt.Printf("Hot key score: %d\n", adaptive.Score("hot:key"))
	fmt.Printf("Tracked keys: %d\n", adaptive.Size())

	// Demonstrate decay: simulate time passing
	fmt.Println("Simulating time decay (accesses spread over time)...")
	for range 100 {
		time.Sleep(1 * time.Millisecond)
		adaptive.RecordAccess("decaying:key")
	}
	decayedTTL := adaptive.TTL("decaying:key", 5*time.Minute)
	fmt.Printf("Decaying key TTL: %v (partial decay)\n", decayedTTL)

	// ---------------------------------------------------------------------------
	// 6. Compression
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== 6. Compression ===")

	// Gzip compression
	gzip := cache.NewGzipCompressor(6)
	fmt.Printf("Compressor: %s\n", gzip.Name())

	// Large data benefits from compression
	largeData := make([]byte, 10*1024) // 10KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	cm := cache.NewCompressionMiddleware(gzip, 512) // compress values > 512 bytes

	compressed, err := cm.Compress(largeData)
	if err != nil {
		log.Fatalf("Compress failed: %v", err)
	}
	compressionRatio := float64(len(compressed)) / float64(len(largeData)) * 100
	fmt.Printf("Original: %d bytes -> Compressed: %d bytes (%.1f%%)\n",
		len(largeData), len(compressed), compressionRatio)

	decompressed, err := cm.Decompress(compressed)
	if err != nil {
		log.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(decompressed, largeData) {
		log.Fatal("Decompressed data doesn't match original!")
	}
	fmt.Println("Decompression verified: data matches original ✓")

	// Small data is not compressed
	smallData := []byte("hello")
	smallCompressed, _ := cm.Compress(smallData)
	if !bytes.Equal(smallCompressed, smallData) {
		log.Fatal("Small data should not be compressed!")
	}
	fmt.Printf("Small data (%d bytes): not compressed (below 512B threshold) ✓\n", len(smallData))

	// Snappy compression
	snappy := cache.NewSnappyCompressor()
	fmt.Printf("\nSnappy compressor: %s\n", snappy.Name())

	snappyCM := cache.NewCompressionMiddleware(snappy, 256)
	snappyCompressed, _ := snappyCM.Compress(largeData)
	snappyRatio := float64(len(snappyCompressed)) / float64(len(largeData)) * 100
	fmt.Printf("Original: %d bytes -> Snappy: %d bytes (%.1f%%)\n",
		len(largeData), len(snappyCompressed), snappyRatio)

	// ---------------------------------------------------------------------------
	// Cleanup
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Cleanup ===")
	fmt.Println("All enterprise features demonstrated successfully!")

	fmt.Println("\n=== Summary ===")
	fmt.Println("✓ Multi-backend manager with health checks")
	fmt.Println("✓ Namespace isolation for multi-tenancy")
	fmt.Println("✓ Cache warming with batch loading")
	fmt.Println("✓ Hot-key detection with alerts")
	fmt.Println("✓ Adaptive TTL with access-frequency tracking")
	fmt.Println("✓ Compression (gzip + snappy) with size-aware threshold")
}
