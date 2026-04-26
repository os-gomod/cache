// Package main demonstrates type-safe cache operations using TypedCache[T].
//
// This example shows:
//   - Creating typed caches with JSON codec for arbitrary structs
//   - String-optimized cache (no JSON overhead)
//   - Int64-optimized cache (no JSON overhead)
//   - Type-safe Get/Set/Delete operations
//   - GetOrSet pattern (cache-aside)
//   - Batch operations with type safety
//   - Atomic operations with type safety
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2"
	"github.com/os-gomod/cache/v2/internal/serialization"
	"github.com/os-gomod/cache/v2/memory"
)

// User is a custom struct we'll cache with JSON encoding.
type User struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// JSON Typed Cache (for structs)
	// ---------------------------------------------------------------------------
	fmt.Println("=== JSON Typed Cache (struct) ===")

	userCache, err := cache.NewMemoryJSON[User](
		memory.WithMaxEntries(1000),
	)
	if err != nil {
		log.Fatalf("failed to create JSON typed cache: %v", err)
	}
	defer userCache.Close(ctx)

	// Store a struct
	alice := User{ID: 1, Name: "Alice", Email: "alice@example.com", CreatedAt: "2024-01-15"}
	err = userCache.Set(ctx, "user:1", alice, 10*time.Minute)
	if err != nil {
		log.Fatalf("Set failed: %v", err)
	}
	fmt.Printf("Set user:1 = %+v\n", alice)

	// Retrieve as typed struct (no manual unmarshaling!)
	retrieved, err := userCache.Get(ctx, "user:1")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get user:1 = %+v\n", retrieved)

	if retrieved.Name != "Alice" {
		log.Fatalf("name mismatch: got %q, want %q", retrieved.Name, "Alice")
	}

	// Exists check
	exists, err := userCache.Exists(ctx, "user:1")
	if err != nil {
		log.Fatalf("Exists failed: %v", err)
	}
	fmt.Printf("Exists user:1 = %v\n", exists)

	// Delete
	err = userCache.Delete(ctx, "user:1")
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("Deleted user:1")

	// ---------------------------------------------------------------------------
	// String Typed Cache (optimized, no JSON overhead)
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== String Typed Cache ===")

	stringCache, err := cache.NewMemoryString()
	if err != nil {
		log.Fatalf("failed to create string typed cache: %v", err)
	}
	defer stringCache.Close(ctx)

	stringCache.Set(ctx, "greeting", "Hello, World!", 5*time.Minute) // nolint: errcheck
	greeting, err := stringCache.Get(ctx, "greeting")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get greeting = %q\n", greeting)

	stringCache.Set(ctx, "config:theme", "dark", 0) // nolint: errcheck (no expiry)
	theme, _ := stringCache.Get(ctx, "config:theme")
	fmt.Printf("Get config:theme = %q\n", theme)

	// ---------------------------------------------------------------------------
	// Int64 Typed Cache (optimized for counters)
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Int64 Typed Cache ===")

	intCache, err := cache.NewMemoryInt64()
	if err != nil {
		log.Fatalf("failed to create int64 typed cache: %v", err)
	}
	defer intCache.Close(ctx)

	// Use as a counter
	intCache.Set(ctx, "page:home:views", 0, 0) // nolint: errcheck

	// Atomically increment
	newCount, err := intCache.Increment(ctx, "page:home:views", 1)
	if err != nil {
		log.Fatalf("Increment failed: %v", err)
	}
	fmt.Printf("Increment page:home:views = %d\n", newCount)

	newCount, err = intCache.Increment(ctx, "page:home:views", 1)
	if err != nil {
		log.Fatalf("Increment failed: %v", err)
	}
	fmt.Printf("Increment page:home:views = %d\n", newCount)

	// Read the counter
	currentCount, err := intCache.Get(ctx, "page:home:views")
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("Get page:home:views = %d\n", currentCount)

	// Decrement
	newCount, err = intCache.Decrement(ctx, "page:home:views", 1)
	if err != nil {
		log.Fatalf("Decrement failed: %v", err)
	}
	fmt.Printf("Decrement page:home:views = %d\n", newCount)

	// ---------------------------------------------------------------------------
	// GetOrSet Pattern (cache-aside)
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== GetOrSet Pattern ===")

	// First call: cache miss, fn is called to load the value
	callCount := 0
	user, err := userCache.GetOrSet(ctx, "user:2", func() (User, error) {
		callCount++
		// Simulate a database lookup
		return User{ID: 2, Name: "Bob", Email: "bob@example.com", CreatedAt: "2024-02-20"}, nil
	}, 10*time.Minute)
	if err != nil {
		log.Fatalf("GetOrSet failed: %v", err)
	}
	fmt.Printf("GetOrSet user:2 = %+v (fn called %d time)\n", user, callCount)

	// Second call: cache hit, fn is NOT called
	user, err = userCache.GetOrSet(ctx, "user:2", func() (User, error) {
		callCount++
		return User{}, errors.New("should not be called")
	}, 10*time.Minute)
	if err != nil {
		log.Fatalf("GetOrSet failed: %v", err)
	}
	fmt.Printf("GetOrSet user:2 = %+v (fn still called %d time)\n", user, callCount)

	// ---------------------------------------------------------------------------
	// SetNX with Typed Values
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== SetNX Pattern ===")

	ok, err := userCache.SetNX(ctx, "user:3", User{ID: 3, Name: "Charlie"}, 5*time.Minute)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX user:3 (first time, created: %v)\n", ok)

	ok, err = userCache.SetNX(ctx, "user:3", User{ID: 3, Name: "Charlie Updated"}, 5*time.Minute)
	if err != nil {
		log.Fatalf("SetNX failed: %v", err)
	}
	fmt.Printf("SetNX user:3 (second time, created: %v)\n", ok)

	user3, _ := userCache.Get(ctx, "user:3")
	fmt.Printf("Get user:3 = %+v (name should be 'Charlie', not 'Charlie Updated')\n", user3)

	// ---------------------------------------------------------------------------
	// Batch Operations
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Batch Operations ===")

	users := map[string]User{
		"user:10": {ID: 10, Name: "User10"},
		"user:11": {ID: 11, Name: "User11"},
		"user:12": {ID: 12, Name: "User12"},
	}
	err = userCache.SetMulti(ctx, users, 5*time.Minute)
	if err != nil {
		log.Fatalf("SetMulti failed: %v", err)
	}
	fmt.Printf("SetMulti: stored %d users\n", len(users))

	retrievedUsers, err := userCache.GetMulti(ctx, "user:10", "user:11", "user:99")
	if err != nil {
		log.Fatalf("GetMulti failed: %v", err)
	}
	fmt.Printf("GetMulti: retrieved %d users\n", len(retrievedUsers))
	for k, u := range retrievedUsers {
		fmt.Printf("  %s = %+v\n", k, u)
	}

	// ---------------------------------------------------------------------------
	// GetSet (swap and return old)
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== GetSet Pattern ===")

	intCache.Set(ctx, "swap:test", 100, 0) // nolint: errcheck
	oldVal, err := intCache.GetSet(ctx, "swap:test", 200, 0)
	if err != nil {
		log.Fatalf("GetSet failed: %v", err)
	}
	fmt.Printf("GetSet swap:test: old=%d, new=200\n", oldVal)

	// ---------------------------------------------------------------------------
	// Statistics
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Statistics ===")
	stats := userCache.Stats()
	fmt.Printf("Stats: Hits=%d, Misses=%d, Evictions=%d\n",
		stats.Hits, stats.Misses, stats.Evictions)

	fmt.Println("\n=== All Operations Completed Successfully ===")
}

// Ensure unused import is referenced.
var _ = serialization.BufPool{}
