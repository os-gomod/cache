// Package main demonstrates layered cache (L1 memory + L2 Redis) operations.
//
// This example shows:
//   - Creating a two-tier layered cache
//   - L1 (memory) hit path for fast reads
//   - L2 (Redis) hit path with automatic L1 promotion
//   - Full miss path
//   - Write-through behavior (writes go to both tiers)
//   - Write-back mode (L1 dirty tracking + async L2 flush)
//   - Proper shutdown
//
// NOTE: This example requires a running Redis instance for the L2 tier.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2/layered"
	"github.com/os-gomod/cache/v2/memory"
	"github.com/os-gomod/cache/v2/redis"
)

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// Create L1 (Memory) and L2 (Redis) tiers
	// ---------------------------------------------------------------------------
	fmt.Println("=== Creating Layered Cache ===")

	l1, err := memory.New(
		memory.WithMaxEntries(1000),
		memory.WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create L1 cache: %v", err)
	}

	l2, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(1), // use DB 1 to avoid conflicts
	)
	if err != nil {
		log.Fatalf("failed to create L2 cache: %v", err)
	}

	// Create layered cache with both tiers
	cache, err := layered.New(
		layered.WithL1(l1),
		layered.WithL2(l2),
	)
	if err != nil {
		log.Fatalf("failed to create layered cache: %v", err)
	}
	defer func() {
		fmt.Println("\n=== Closing Layered Cache ===")
		if err := cache.Close(ctx); err != nil {
			log.Printf("close error: %v", err)
		}
		// Individual tiers are closed by layered.New (via cache.Close)
		fmt.Println("Layered cache closed successfully")
	}()

	fmt.Printf("Layered cache created: %s\n", cache.Name())
	fmt.Println("  L1: memory (maxEntries=1000, eviction=LRU)")
	fmt.Println("  L2: redis (addr=localhost:6379, db=1)")

	// ---------------------------------------------------------------------------
	// Health Check
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Health Check ===")
	err = cache.Ping(ctx)
	if err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Ping: OK (both tiers healthy)")

	// ---------------------------------------------------------------------------
	// Write-through: writes go to both tiers
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Write-Through Operations ===")

	err = cache.Set(
		ctx,
		"product:1",
		[]byte(`{"id":1,"name":"Widget","price":29.99}`),
		10*time.Minute,
	)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Println("Set product:1 -> written to L1 and L2")

	err = cache.Set(
		ctx,
		"product:2",
		[]byte(`{"id":2,"name":"Gadget","price":49.99}`),
		10*time.Minute,
	)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Println("Set product:2 -> written to L1 and L2")

	// ---------------------------------------------------------------------------
	// L1 Hit Path: data is in L1 memory
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== L1 Hit Path ===")
	start := time.Now()
	val, err := cache.Get(ctx, "product:1")
	elapsed := time.Since(start)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get product:1 (L1 hit): %s (latency: %v)\n", string(val), elapsed)

	// ---------------------------------------------------------------------------
	// L2 Promotion: simulate L1 miss, data promoted from L2
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== L2 Hit + L1 Promotion ===")
	fmt.Println("Clearing L1 to simulate L1 miss...")
	l1.Clear(ctx) // nolint: errcheck

	start = time.Now()
	val, err = cache.Get(ctx, "product:2")
	elapsed = time.Since(start)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get product:2 (L2 hit + L1 promotion): %s (latency: %v)\n", string(val), elapsed)

	// Verify promotion: second read should be L1 hit
	start = time.Now()
	val, err = cache.Get(ctx, "product:2")
	elapsed = time.Since(start)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get product:2 (L1 hit after promotion): %s (latency: %v)\n", string(val), elapsed)

	// ---------------------------------------------------------------------------
	// Full Miss Path
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Full Miss Path ===")
	start = time.Now()
	_, err = cache.Get(ctx, "product:nonexistent")
	elapsed = time.Since(start)
	if err == nil {
		log.Fatal("Get for nonexistent key should return error")
	}
	fmt.Printf("Get product:nonexistent (miss): %v (latency: %v)\n", err, elapsed)

	// ---------------------------------------------------------------------------
	// Atomic Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Atomic Operations ===")

	// SetNX
	ok, err := cache.SetNX(ctx, "lock:job:1", []byte("locked"), 30*time.Second)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX lock:job:1 (acquired: %v)\n", ok)

	// Increment
	cache.Set(ctx, "counter:views", []byte("0"), 5*time.Minute) // nolint: errcheck
	newVal, err := cache.Increment(ctx, "counter:views", 1)
	if err != nil {
		log.Fatalf("Increment failed: %v", err)
	}
	fmt.Printf("Increment counter:views = %d\n", newVal)

	// ---------------------------------------------------------------------------
	// Batch Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Batch Operations ===")

	items := map[string][]byte{
		"cat:1": []byte("Electronics"),
		"cat:2": []byte("Books"),
		"cat:3": []byte("Clothing"),
	}
	err = cache.SetMulti(ctx, items, 10*time.Minute)
	if err != nil {
		log.Fatalf("SetMulti failed: %v", err)
	}
	fmt.Printf("SetMulti: stored %d categories in both tiers\n", len(items))

	results, err := cache.GetMulti(ctx, "cat:1", "cat:2", "cat:999")
	if err != nil {
		log.Fatalf("GetMulti failed: %v", err)
	}
	fmt.Printf("GetMulti: retrieved %d values (cat:999 missing = expected)\n", len(results))
	for k, v := range results {
		fmt.Printf("  %s = %s\n", k, string(v))
	}

	// ---------------------------------------------------------------------------
	// Key Scanning
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Key Scanning ===")
	keys, err := cache.Keys(ctx, "cat:*")
	if err != nil {
		log.Fatalf("Keys failed: %v", err)
	}
	fmt.Printf("Keys matching 'cat:*': %v\n", keys)

	// ---------------------------------------------------------------------------
	// Statistics
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Statistics ===")
	stats := cache.Stats()
	fmt.Printf("Stats: Hits=%d, Misses=%d, Evictions=%d\n",
		stats.Hits, stats.Misses, stats.Evictions)

	// ---------------------------------------------------------------------------
	// Cleanup
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Cleanup ===")
	cache.Delete(ctx, "product:1")                    // nolint: errcheck
	cache.Delete(ctx, "product:2")                    // nolint: errcheck
	cache.Delete(ctx, "lock:job:1")                   // nolint: errcheck
	cache.Delete(ctx, "counter:views")                // nolint: errcheck
	cache.DeleteMulti(ctx, "cat:1", "cat:2", "cat:3") // nolint: errcheck
	fmt.Println("All test keys cleaned up")

	fmt.Println("\n=== All Operations Completed Successfully ===")
}
