// Package main demonstrates basic memory cache operations with os-gomod/cache v2.
//
// This example shows:
//   - Creating a memory cache with configuration options
//   - Set, Get, Delete operations
//   - TTL (time-to-live) behavior
//   - Batch operations (GetMulti, SetMulti, DeleteMulti)
//   - Atomic operations (CompareAndSwap, SetNX, Increment, Decrement, GetSet)
//   - Scanning (Keys, Size, Clear)
//   - Proper shutdown with Close
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2/memory"
)

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// Create a memory cache with options
	// ---------------------------------------------------------------------------
	fmt.Println("=== Creating Memory Cache ===")
	cache, err := memory.New(
		memory.WithMaxEntries(10000),
		memory.WithDefaultTTL(5*time.Minute),
		memory.WithShardCount(64),
	)
	if err != nil {
		log.Fatalf("failed to create memory cache: %v", err)
	}
	defer func() {
		fmt.Println("\n=== Closing Cache ===")
		if err := cache.Close(ctx); err != nil {
			log.Printf("close error: %v", err)
		}
		fmt.Println("Cache closed successfully")
	}()

	fmt.Printf("Cache created: %s (shards=%d, maxEntries=%d)\n",
		cache.Name(), 64, 10000)

	// ---------------------------------------------------------------------------
	// Basic Set / Get / Delete
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Basic Operations ===")

	// Set a value with TTL
	err = cache.Set(ctx, "user:1:name", []byte("Alice"), 10*time.Minute)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Println("Set user:1:name = Alice (TTL=10m)")

	// Get the value
	val, err := cache.Get(ctx, "user:1:name")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get user:1:name = %s\n", string(val))

	// Check existence
	exists, err := cache.Exists(ctx, "user:1:name")
	if err != nil {
		log.Fatalf("Exists failed: %v", err)
	}
	fmt.Printf("Exists user:1:name = %v\n", exists)

	// Check TTL
	ttl, err := cache.TTL(ctx, "user:1:name")
	if err != nil {
		log.Fatalf("TTL failed: %v", err)
	}
	fmt.Printf("TTL user:1:name = %v\n", ttl.Round(time.Second))

	// Delete
	err = cache.Delete(ctx, "user:1:name")
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("Deleted user:1:name")

	// Verify deletion
	_, err = cache.Get(ctx, "user:1:name")
	if err != nil {
		fmt.Printf("Get after delete: key not found (expected)\n")
	}

	// ---------------------------------------------------------------------------
	// Batch Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Batch Operations ===")

	// SetMultiple
	items := map[string][]byte{
		"batch:1": []byte("value-one"),
		"batch:2": []byte("value-two"),
		"batch:3": []byte("value-three"),
	}
	err = cache.SetMulti(ctx, items, 5*time.Minute)
	if err != nil {
		log.Fatalf("SetMulti failed: %v", err)
	}
	fmt.Printf("SetMulti: stored %d items\n", len(items))

	// GetMultiple
	results, err := cache.GetMulti(ctx, "batch:1", "batch:2", "batch:999")
	if err != nil {
		log.Fatalf("GetMulti failed: %v", err)
	}
	fmt.Printf("GetMulti: got %d results (batch:999 missing = expected)\n", len(results))
	for k, v := range results {
		fmt.Printf("  %s = %s\n", k, string(v))
	}

	// DeleteMultiple
	err = cache.DeleteMulti(ctx, "batch:1", "batch:3")
	if err != nil {
		log.Fatalf("DeleteMulti failed: %v", err)
	}
	fmt.Println("DeleteMulti: removed batch:1 and batch:3")

	// ---------------------------------------------------------------------------
	// Atomic Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Atomic Operations ===")

	// SetNX (set if not exists)
	ok, err := cache.SetNX(ctx, "atomic:counter", []byte("1"), 5*time.Minute)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX atomic:counter = 1 (was set: %v)\n", ok)

	ok, err = cache.SetNX(ctx, "atomic:counter", []byte("999"), 5*time.Minute)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX atomic:counter = 999 (was set: %v, expected false)\n", ok)

	// Increment
	newVal, err := cache.Increment(ctx, "atomic:counter", 5)
	if err != nil {
		log.Fatalf("Increment failed: %v", err)
	}
	fmt.Printf("Increment atomic:counter by 5 = %d\n", newVal)

	// Decrement
	newVal, err = cache.Decrement(ctx, "atomic:counter", 2)
	if err != nil {
		log.Fatalf("Decrement failed: %v", err)
	}
	fmt.Printf("Decrement atomic:counter by 2 = %d\n", newVal)

	// CompareAndSwap on a non-existent key returns an error
	ok, err = cache.CompareAndSwap(ctx, "atomic:cas", []byte("old"), []byte("new"), 5*time.Minute)
	fmt.Printf("CompareAndSwap atomic:cas (key not found, err=%v): %v\n", err, ok)

	// Set first, then CAS
	cache.Set(ctx, "atomic:cas", []byte("old"), 5*time.Minute)
	ok, err = cache.CompareAndSwap(ctx, "atomic:cas", []byte("old"), []byte("new"), 5*time.Minute)
	if err != nil {
		log.Fatalf("CompareAndSwap failed: %v", err)
	}
	fmt.Printf("CompareAndSwap atomic:cas (hit): %v\n", ok)

	val, _ = cache.Get(ctx, "atomic:cas")
	fmt.Printf("Value after CAS: %s\n", string(val))

	// GetSet
	oldVal, err := cache.GetSet(ctx, "atomic:getset", []byte("replaced"), 5*time.Minute)
	if err != nil {
		log.Fatalf("GetSet failed: %v", err)
	}
	fmt.Printf("GetSet atomic:getset (old value): %q (new: \"replaced\")\n", string(oldVal))

	// ---------------------------------------------------------------------------
	// Scanning Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Scanning Operations ===")

	// Add some prefixed keys
	for i := range 5 {
		cache.Set(
			ctx,
			fmt.Sprintf("scan:items:%d", i),
			[]byte(fmt.Sprintf("item-%d", i)),
			5*time.Minute,
		)
	}

	keys, err := cache.Keys(ctx, "scan:items:*")
	if err != nil {
		log.Fatalf("Keys failed: %v", err)
	}
	fmt.Printf("Keys matching 'scan:items:*': %v\n", keys)

	size, err := cache.Size(ctx)
	if err != nil {
		log.Fatalf("Size failed: %v", err)
	}
	fmt.Printf("Cache size: %d entries\n", size)

	err = cache.Clear(ctx)
	if err != nil {
		log.Fatalf("Clear failed: %v", err)
	}
	fmt.Println("Cache cleared")

	size, _ = cache.Size(ctx)
	fmt.Printf("Cache size after clear: %d entries\n", size)

	// ---------------------------------------------------------------------------
	// Statistics
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Statistics ===")
	// Re-populate for stats demonstration
	for i := range 10 {
		cache.Set(ctx, fmt.Sprintf("stat:key:%d", i), []byte("val"), 5*time.Minute)
	}
	for i := range 10 {
		cache.Get(ctx, fmt.Sprintf("stat:key:%d", i)) // hits
	}
	cache.Get(ctx, "stat:nonexistent") // miss

	stats := cache.Stats()
	fmt.Printf("Stats: %+v\n", stats)

	// ---------------------------------------------------------------------------
	// Health Check
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Health Check ===")
	err = cache.Ping(ctx)
	if err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Ping: OK")

	fmt.Println("\n=== All Operations Completed Successfully ===")
}
