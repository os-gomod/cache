// Package main demonstrates Redis cache operations with os-gomod/cache v2.
//
// This example shows:
//   - Creating a Redis cache with configuration options
//   - Set, Get, Delete operations
//   - Key prefix configuration
//   - TTL behavior
//   - Atomic operations
//   - Proper shutdown with Close
//
// NOTE: This example requires a running Redis instance.
// Adjust the connection options to match your Redis setup.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2/redis"
)

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// Create a Redis cache with options
	// ---------------------------------------------------------------------------
	fmt.Println("=== Creating Redis Cache ===")
	cache, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithPassword(""),        // set password if needed
		redis.WithDB(0),               // use database 0
		redis.WithKeyPrefix("app:v2"), // namespace all keys
		redis.WithPoolSize(10),        // connection pool size
		redis.WithDialTimeout(5*time.Second),
		redis.WithReadTimeout(3*time.Second),
		redis.WithWriteTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatalf("failed to create Redis cache: %v", err)
	}
	defer func() {
		fmt.Println("\n=== Closing Redis Cache ===")
		if err := cache.Close(ctx); err != nil {
			log.Printf("close error: %v", err)
		}
		fmt.Println("Redis cache closed successfully")
	}()

	fmt.Printf("Redis cache created: %s (addr=localhost:6379, prefix=app:v2)\n",
		cache.Name())

	// ---------------------------------------------------------------------------
	// Health Check
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Health Check ===")
	err = cache.Ping(ctx)
	if err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}
	fmt.Println("Redis PING: PONG")

	// ---------------------------------------------------------------------------
	// Basic Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Basic Operations ===")

	// Set a value
	err = cache.Set(ctx, "user:1:profile", []byte(`{"name":"Bob","age":30}`), 30*time.Minute)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Println("Set user:1:profile (stored with prefix 'app:v2:user:1:profile')")

	// Get the value
	val, err := cache.Get(ctx, "user:1:profile")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get user:1:profile = %s\n", string(val))

	// Check existence
	exists, err := cache.Exists(ctx, "user:1:profile")
	if err != nil {
		log.Fatalf("Exists failed: %v", err)
	}
	fmt.Printf("Exists user:1:profile = %v\n", exists)

	// Check TTL
	ttl, err := cache.TTL(ctx, "user:1:profile")
	if err != nil {
		log.Fatalf("TTL failed: %v", err)
	}
	fmt.Printf("TTL user:1:profile = %v\n", ttl.Round(time.Second))

	// ---------------------------------------------------------------------------
	// Batch Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Batch Operations ===")

	items := map[string][]byte{
		"session:abc": []byte("session-data-abc"),
		"session:def": []byte("session-data-def"),
		"session:ghi": []byte("session-data-ghi"),
	}
	err = cache.SetMulti(ctx, items, 1*time.Hour)
	if err != nil {
		log.Fatalf("SetMulti failed: %v", err)
	}
	fmt.Printf("SetMulti: stored %d session keys\n", len(items))

	results, err := cache.GetMulti(ctx, "session:abc", "session:def")
	if err != nil {
		log.Fatalf("GetMulti failed: %v", err)
	}
	fmt.Printf("GetMulti: retrieved %d values\n", len(results))
	for k, v := range results {
		fmt.Printf("  %s = %s\n", k, string(v))
	}

	err = cache.DeleteMulti(ctx, "session:abc", "session:ghi")
	if err != nil {
		log.Fatalf("DeleteMulti failed: %v", err)
	}
	fmt.Println("DeleteMulti: removed 2 session keys")

	// ---------------------------------------------------------------------------
	// Atomic Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Atomic Operations ===")

	// SetNX - useful for distributed locking
	lockKey := "lock:resource:1"
	ok, err := cache.SetNX(ctx, lockKey, []byte("locked"), 10*time.Second)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX %s (acquired: %v)\n", lockKey, ok)

	ok, err = cache.SetNX(ctx, lockKey, []byte("locked-again"), 10*time.Second)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX %s again (acquired: %v, expected false)\n", lockKey, ok)

	// Increment - useful for rate limiting counters
	counterKey := "rate:user:1"
	cache.Set(ctx, counterKey, []byte("0"), 1*time.Minute)
	for range 5 {
		val, err := cache.Increment(ctx, counterKey, 1)
		if err != nil {
			log.Fatalf("Increment failed: %v", err)
		}
		fmt.Printf("Increment %s = %d\n", counterKey, val)
	}

	// Decrement
	newVal, err := cache.Decrement(ctx, counterKey, 2)
	if err != nil {
		log.Fatalf("Decrement failed: %v", err)
	}
	fmt.Printf("Decrement %s by 2 = %d\n", counterKey, newVal)

	// ---------------------------------------------------------------------------
	// Key Scanning
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Key Scanning ===")

	// Set up some keys for scanning
	for i := range 5 {
		cache.Set(
			ctx,
			fmt.Sprintf("product:%d", i),
			[]byte(fmt.Sprintf("product-%d", i)),
			5*time.Minute,
		)
	}

	keys, err := cache.Keys(ctx, "product:*")
	if err != nil {
		log.Fatalf("Keys failed: %v", err)
	}
	fmt.Printf("Keys matching 'product:*': %v\n", keys)

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
	cache.Delete(ctx, "user:1:profile")
	cache.Delete(ctx, lockKey)
	cache.Delete(ctx, counterKey)
	for i := range 5 {
		cache.Delete(ctx, fmt.Sprintf("product:%d", i))
	}
	fmt.Println("All test keys cleaned up")

	fmt.Println("\n=== All Operations Completed Successfully ===")
}
