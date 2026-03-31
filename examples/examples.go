// Package main demonstrates all features of the cache library.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/os-gomod/cache"
	"github.com/os-gomod/cache/builder"
	_errors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/layered"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
)

func main() {
	ctx := context.Background()

	// Demonstrate all features
	demoMemoryCache(ctx)
	demoRedisCache(ctx)
	demoLayeredCache(ctx)
	demoBuilderPattern(ctx)
	demoTypedCaches(ctx)
	demoEvictionPolicies(ctx)
	demoResilienceWrappers(ctx)
	demoKeyBuilder(ctx)
	demoErrorHandling(ctx)
	demoStats(ctx)

	printFooter()
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func printSection(title string) {
	fmt.Printf("\n%s\n", strings.Repeat("━", 80))
	fmt.Printf("  %s\n", title)
	fmt.Printf("%s\n", strings.Repeat("━", 80))
}

func printSubsection(title string) {
	fmt.Printf("\n📁 %s:\n", title)
}

func printKeyValue(key string, value interface{}) {
	fmt.Printf("   %s: %v\n", key, value)
}

func printSuccess(message string) {
	fmt.Printf("\n   ✅ %s\n", message)
}

func printError(message string) {
	fmt.Printf("   ❌ %s\n", message)
}

func printInfo(message string) {
	fmt.Printf("   ℹ️  %s\n", message)
}

func printWarning(message string) {
	fmt.Printf("   ⚠️  %s\n", message)
}

func printMetric(metric string, value interface{}) {
	fmt.Printf("   📊 %s: %v\n", metric, value)
}

// -----------------------------------------------------------------------------
// 1. Memory Cache Demonstration
// -----------------------------------------------------------------------------

func demoMemoryCache(ctx context.Context) {
	printSection("MEMORY CACHE")

	// Basic operations
	printSubsection("Basic Operations")
	mem, err := memory.New(
		memory.WithMaxEntries(100),
		memory.WithMaxMemoryMB(10),
		memory.WithTTL(5*time.Minute),
		memory.WithCleanupInterval(30*time.Second),
		memory.WithOnEvictionPolicy(func(key, reason string) {
			fmt.Printf("   📢 Evicted: %s (reason: %s)\n", key, reason)
		}),
	)
	if err != nil {
		printError("Failed to create memory cache: " + err.Error())
		return
	}
	defer mem.Close(ctx)

	// Set and Get
	err = mem.Set(ctx, "user:1", []byte(`{"id":1,"name":"Alice"}`), 0)
	if err != nil {
		printError("Set failed: " + err.Error())
	} else {
		printSuccess("Set user:1")
	}

	val, err := mem.Get(ctx, "user:1")
	if err != nil {
		printError("Get failed: " + err.Error())
	} else {
		printKeyValue("user:1", string(val))
	}

	// Exists
	exists, _ := mem.Exists(ctx, "user:1")
	printKeyValue("Exists", exists)

	// TTL
	ttl, _ := mem.TTL(ctx, "user:1")
	printKeyValue("TTL remaining", ttl)

	// Delete
	err = mem.Delete(ctx, "user:1")
	if err != nil {
		printError("Delete failed: " + err.Error())
	} else {
		printSuccess("Deleted user:1")
	}

	// GetOrSet with singleflight
	printSubsection("GetOrSet (Singleflight)")
	var counter int
	val, err = mem.GetOrSet(ctx, "expensive:key", func() ([]byte, error) {
		counter++
		printInfo("Computing expensive value (call #" + fmt.Sprint(counter) + ")")
		time.Sleep(100 * time.Millisecond)
		return []byte("computed-value"), nil
	}, time.Minute)
	if err != nil {
		printError("GetOrSet failed: " + err.Error())
	} else {
		printKeyValue("GetOrSet result", string(val))
		printKeyValue("Function called", counter)
	}

	// Batch operations
	printSubsection("Batch Operations")
	items := map[string][]byte{
		"batch:1": []byte("value1"),
		"batch:2": []byte("value2"),
		"batch:3": []byte("value3"),
	}
	err = mem.SetMulti(ctx, items, time.Minute)
	if err != nil {
		printError("SetMulti failed: " + err.Error())
	} else {
		printSuccess("Set 3 items")
	}

	multiVal, _ := mem.GetMulti(ctx, "batch:1", "batch:2", "batch:3")
	printKeyValue("GetMulti count", len(multiVal))

	err = mem.DeleteMulti(ctx, "batch:1", "batch:2", "batch:3")
	if err != nil {
		printError("DeleteMulti failed: " + err.Error())
	} else {
		printSuccess("Deleted 3 items")
	}

	// Atomic operations
	printSubsection("Atomic Operations")
	_, _ = mem.SetNX(ctx, "counter", []byte("10"), 0)
	inc, _ := mem.Increment(ctx, "counter", 5)
	printKeyValue("Increment result", inc)
	dec, _ := mem.Decrement(ctx, "counter", 3)
	printKeyValue("Decrement result", dec)

	// CAS
	swapped, _ := mem.CompareAndSwap(ctx, "counter", []byte("12"), []byte("20"), 0)
	printKeyValue("CAS swapped", swapped)

	// GetSet
	old, _ := mem.GetSet(ctx, "counter", []byte("100"), 0)
	printKeyValue("GetSet old value", string(old))

	// Keys and Clear
	keys, _ := mem.Keys(ctx, "*")
	printKeyValue("Keys count", len(keys))
	_ = mem.Clear(ctx)
	printSuccess("Cache cleared")
}

// -----------------------------------------------------------------------------
// 2. Redis Cache Demonstration
// -----------------------------------------------------------------------------

func demoRedisCache(ctx context.Context) {
	printSection("REDIS CACHE")

	// Create Redis cache (requires Redis running)
	rc, err := redis.New(
		redis.WithAddress("localhost:6379"),
		redis.WithDB(0),
		redis.WithKeyPrefix("demo:"),
		redis.WithTTL(time.Hour),
		redis.WithPoolSize(10),
	)
	if err != nil {
		printWarning("Redis not available: " + err.Error())
		printInfo("Skipping Redis demonstration")
		return
	}
	defer rc.Close(ctx)

	// Ping test
	err = rc.Ping(ctx)
	if err != nil {
		printWarning("Redis connection failed: " + err.Error())
		return
	}
	printSuccess("Redis connected")

	// Basic KV operations
	printSubsection("Basic KV Operations")
	_ = rc.Set(ctx, "key1", []byte("value1"), 30*time.Second)
	val, _ := rc.Get(ctx, "key1")
	printKeyValue("Get key1", string(val))

	// Hash operations
	printSubsection("Hash Operations")
	_ = rc.HSet(ctx, "user:100", "name", "Bob")
	_ = rc.HSet(ctx, "user:100", "email", "bob@example.com")
	name, _ := rc.HGet(ctx, "user:100", "name")
	printKeyValue("Hash field name", string(name))
	all, _ := rc.HGetAll(ctx, "user:100")
	printKeyValue("All hash fields", len(all))

	// List operations
	printSubsection("List Operations")
	_ = rc.LPush(ctx, "queue", "task1", "task2", "task3")
	_ = rc.RPush(ctx, "queue", "task4")
	first, _ := rc.LPop(ctx, "queue")
	printKeyValue("LPopped", string(first))
	rangeVals, _ := rc.LRange(ctx, "queue", 0, -1)
	printKeyValue("Queue length", len(rangeVals))

	// Set operations
	printSubsection("Set Operations")
	_ = rc.SAdd(ctx, "tags", "go", "cache", "redis")
	members, _ := rc.SMembers(ctx, "tags")
	printKeyValue("Set members", strings.Join(members, ", "))

	// Sorted set operations
	printSubsection("Sorted Set Operations")
	_ = rc.ZAdd(ctx, "leaderboard", 100.0, "player1")
	_ = rc.ZAdd(ctx, "leaderboard", 95.0, "player2")
	_ = rc.ZAdd(ctx, "leaderboard", 110.0, "player3")
	top, _ := rc.ZRange(ctx, "leaderboard", 0, -1)
	printKeyValue("Leaderboard", strings.Join(top, ", "))

	// Batch operations
	printSubsection("Batch Operations")
	_ = rc.SetMulti(ctx, map[string][]byte{
		"multi:1": []byte("first"),
		"multi:2": []byte("second"),
	}, time.Minute)
	multi, _ := rc.GetMulti(ctx, "multi:1", "multi:2")
	printKeyValue("GetMulti results", len(multi))

	// Expire and Persist
	printSubsection("TTL Management")
	_ = rc.Expire(ctx, "key1", 10*time.Second)
	ttl, _ := rc.TTL(ctx, "key1")
	printKeyValue("TTL after Expire", ttl)
	_ = rc.Persist(ctx, "key1")
	ttl, _ = rc.TTL(ctx, "key1")
	printKeyValue("TTL after Persist", ttl)

	// Cleanup
	_ = rc.Clear(ctx)
	printSuccess("Redis cache cleared")
}

// -----------------------------------------------------------------------------
// 3. Layered Cache Demonstration
// -----------------------------------------------------------------------------

func demoLayeredCache(ctx context.Context) {
	printSection("LAYERED CACHE (L1 Memory + L2 Redis)")

	// Create layered cache
	lc, err := layered.New(
		layered.WithL1MaxEntries(1000),
		layered.WithL1TTL(2*time.Minute),
		layered.WithL1LRU(),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2TTL(time.Hour),
		layered.WithL2KeyPrefix("layered:"),
		layered.WithPromoteOnHit(true),
		layered.WithNegativeTTL(30*time.Second),
	)
	if err != nil {
		printWarning("Layered cache not available: " + err.Error())
		return
	}
	defer lc.Close(ctx)

	// Set operation (writes to both layers)
	printSubsection("Set Operation")
	err = lc.Set(ctx, "shared:key", []byte("shared-value"), time.Minute)
	if err != nil {
		printError("Set failed: " + err.Error())
	} else {
		printSuccess("Set shared:key")
	}

	// First Get (L1 miss, L2 hit, promotes to L1)
	printSubsection("Get - First Call (L2 Hit, Promote to L1)")
	val, err := lc.Get(ctx, "shared:key")
	if err != nil {
		printError("Get failed: " + err.Error())
	} else {
		printKeyValue("Retrieved", string(val))
	}

	// Second Get (L1 hit)
	printSubsection("Get - Second Call (L1 Hit)")
	val, err = lc.Get(ctx, "shared:key")
	if err != nil {
		printError("Get failed: " + err.Error())
	} else {
		printKeyValue("Retrieved from L1", string(val))
	}

	// GetOrSet with stampede protection
	printSubsection("GetOrSet (Singleflight)")
	var computationCount int
	result, err := lc.GetOrSet(ctx, "computed:key", func() ([]byte, error) {
		computationCount++
		printInfo("Computing value (call #" + fmt.Sprint(computationCount) + ")")
		return []byte("expensive-result"), nil
	}, time.Minute)
	if err != nil {
		printError("GetOrSet failed: " + err.Error())
	} else {
		printKeyValue("GetOrSet result", string(result))
		printKeyValue("Computation count", computationCount)
	}

	// Write-back mode demonstration
	printSubsection("Write-Back Mode")
	wbCache, err := layered.New(
		layered.WithWriteBack(true),
		layered.WithWriteBackConfig(512, 2),
		layered.WithL1MaxEntries(500),
		layered.WithL2Address("localhost:6379"),
	)
	if err != nil {
		printWarning("Write-back cache creation failed: " + err.Error())
	} else {
		err = wbCache.Set(ctx, "async:key", []byte("async-value"), time.Minute)
		if err != nil {
			printError("Async Set failed: " + err.Error())
		} else {
			printSuccess("Async write queued (immediate L1, background L2)")
		}
		wbCache.Close(ctx)
	}

	// Negative caching
	printSubsection("Negative Caching")
	_, err = lc.Get(ctx, "non-existent-key")
	if _errors.IsNotFound(err) {
		printInfo("First miss: key not found")
	}
	_, err = lc.Get(ctx, "non-existent-key")
	if _errors.IsNotFound(err) {
		printInfo("Second miss: served from negative cache")
	}

	// L1 invalidation
	printSubsection("L1 Invalidation")
	_ = lc.InvalidateL1(ctx, "shared:key")
	printSuccess("L1 entry invalidated (L2 still has data)")
	val, _ = lc.Get(ctx, "shared:key")
	printKeyValue("After L1 invalidation (L2 hit)", string(val))
}

// -----------------------------------------------------------------------------
// 4. Builder Pattern Demonstration
// -----------------------------------------------------------------------------

func demoBuilderPattern(ctx context.Context) {
	printSection("BUILDER PATTERN")

	// Builder root
	b := builder.New(ctx)

	// Memory cache via builder
	printSubsection("Memory Cache Builder")
	memCache, err := b.Memory().
		MaxEntries(100).
		MaxMemoryMB(10).
		TTL(5 * time.Minute).
		LRU().
		ShardCount(32).
		Build()
	if err != nil {
		printError("Memory builder failed: " + err.Error())
	} else {
		_ = memCache.Set(ctx, "builder:key", []byte("builder-value"), 0)
		val, _ := memCache.Get(ctx, "builder:key")
		printKeyValue("Builder-created cache", string(val))
		memCache.Close(ctx)
	}

	// Redis cache via builder
	printSubsection("Redis Cache Builder")
	redisCache, err := b.Redis().
		Addr("localhost:6379").
		DB(0).
		TTL(time.Hour).
		KeyPrefix("builder:").
		EnablePipeline(true).
		Build()
	if err != nil {
		printWarning("Redis builder failed: " + err.Error())
	} else {
		_ = redisCache.Set(ctx, "test", []byte("ok"), 0)
		val, _ := redisCache.Get(ctx, "test")
		printKeyValue("Builder-created Redis", string(val))
		redisCache.Close(ctx)
	}

	// Layered cache via builder
	printSubsection("Layered Cache Builder")
	layeredCache, err := b.Layered().
		L1MaxEntries(1000).
		L1TTL(2 * time.Minute).
		L1LRU().
		L2Addr("localhost:6379").
		PromoteOnHit(true).
		NegativeTTL(30 * time.Second).
		Build()
	if err != nil {
		printWarning("Layered builder failed: " + err.Error())
	} else {
		_ = layeredCache.Set(ctx, "builder:layered", []byte("value"), 0)
		val, _ := layeredCache.Get(ctx, "builder:layered")
		printKeyValue("Builder-created layered", string(val))
		layeredCache.Close(ctx)
	}

	// Convenience functions
	printSubsection("Convenience Functions")
	defaultCache, _ := builder.DefaultMemory(ctx)
	_ = defaultCache.Set(ctx, "default", []byte("value"), 0)
	val, _ := defaultCache.Get(ctx, "default")
	printKeyValue("DefaultMemory", string(val))
	defaultCache.Close(ctx)
}

// -----------------------------------------------------------------------------
// 5. Typed Caches Demonstration
// -----------------------------------------------------------------------------

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func demoTypedCaches(ctx context.Context) {
	printSection("TYPED CACHES")

	// Create base memory cache
	base, _ := memory.New()

	// JSON-typed cache for User structs
	printSubsection("JSON-Typed Cache")
	userCache := cache.NewJSONTypedCache[User](base)

	user := User{ID: 1, Name: "Charlie", Email: "charlie@example.com"}
	err := userCache.Set(ctx, "user:1", user, 5*time.Minute)
	if err != nil {
		printError("Set user failed: " + err.Error())
	} else {
		printSuccess("User stored")
	}

	retrieved, err := userCache.Get(ctx, "user:1")
	if err != nil {
		printError("Get user failed: " + err.Error())
	} else {
		printKeyValue("Retrieved user", fmt.Sprintf("%+v", retrieved))
	}

	// String-typed cache
	printSubsection("String-Typed Cache")
	strCache := cache.NewTypedStringCache(base)
	_ = strCache.Set(ctx, "greeting", "Hello, World!", 0)
	greeting, _ := strCache.Get(ctx, "greeting")
	printKeyValue("String cache", greeting)

	// Int64-typed cache with atomic operations
	printSubsection("Int64-Typed Cache (with Counters)")
	intCache := cache.NewTypedInt64Cache(base)
	_, _ = intCache.Increment(ctx, "visits", 1)
	_, _ = intCache.Increment(ctx, "visits", 1)
	visits, _ := intCache.Get(ctx, "visits")
	printKeyValue("Visit count", visits)

	// GetOrSet with typed cache
	printSubsection("Typed GetOrSet")
	var computed bool
	val, err := userCache.GetOrSet(ctx, "computed:user", func() (User, error) {
		computed = true
		printInfo("Computing user value")
		return User{ID: 99, Name: "Computed", Email: "computed@example.com"}, nil
	}, time.Minute)
	if err != nil {
		printError("GetOrSet failed: " + err.Error())
	} else {
		printKeyValue("GetOrSet result", fmt.Sprintf("%+v", val))
		printKeyValue("Computed", computed)
	}

	// Batch operations with typed cache
	printSubsection("Batch Operations")
	users := map[string]User{
		"user:2": {ID: 2, Name: "Dave", Email: "dave@example.com"},
		"user:3": {ID: 3, Name: "Eve", Email: "eve@example.com"},
	}
	_ = userCache.SetMulti(ctx, users, time.Minute)
	multi, _ := userCache.GetMulti(ctx, "user:2", "user:3")
	printKeyValue("Batch retrieved count", len(multi))

	base.Close(ctx)
	printSuccess("Typed caches demonstrated")
}

// -----------------------------------------------------------------------------
// 6. Eviction Policies Demonstration
// -----------------------------------------------------------------------------

func demoEvictionPolicies(ctx context.Context) {
	printSection("EVICTION POLICIES")

	// LRU Example
	printSubsection("LRU (Least Recently Used)")
	lru, _ := memory.New(memory.WithMaxEntries(3), memory.WithLRU())
	_ = lru.Set(ctx, "A", []byte("1"), 0)
	_ = lru.Set(ctx, "B", []byte("2"), 0)
	_ = lru.Set(ctx, "C", []byte("3"), 0)
	_, _ = lru.Get(ctx, "A")              // Make A most recent
	_ = lru.Set(ctx, "D", []byte("4"), 0) // Evicts B
	_, err := lru.Get(ctx, "B")
	printKeyValue("B evicted", err != nil)
	lru.Close(ctx)

	// LFU Example
	printSubsection("LFU (Least Frequently Used)")
	lfu, _ := memory.New(memory.WithMaxEntries(3), memory.WithLFU())
	_ = lfu.Set(ctx, "X", []byte("1"), 0)
	_ = lfu.Set(ctx, "Y", []byte("2"), 0)
	_ = lfu.Set(ctx, "Z", []byte("3"), 0)
	for i := 0; i < 5; i++ {
		_, _ = lfu.Get(ctx, "X")
		_, _ = lfu.Get(ctx, "Y")
	}
	_ = lfu.Set(ctx, "W", []byte("4"), 0) // Evicts Z (least frequent)
	_, err = lfu.Get(ctx, "Z")
	printKeyValue("Z evicted", err != nil)
	lfu.Close(ctx)

	// FIFO Example
	printSubsection("FIFO (First In First Out)")
	fifo, _ := memory.New(memory.WithMaxEntries(3), memory.WithFIFO())
	_ = fifo.Set(ctx, "1st", []byte("first"), 0)
	_ = fifo.Set(ctx, "2nd", []byte("second"), 0)
	_ = fifo.Set(ctx, "3rd", []byte("third"), 0)
	_ = fifo.Set(ctx, "4th", []byte("fourth"), 0) // Evicts 1st
	_, err = fifo.Get(ctx, "1st")
	printKeyValue("First evicted", err != nil)
	fifo.Close(ctx)

	// TinyLFU Example
	printSubsection("TinyLFU (W-TinyLFU)")
	tinylfu, _ := memory.New(memory.WithMaxEntries(10), memory.WithTinyLFU())
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key:%d", i)
		_ = tinylfu.Set(ctx, key, []byte("value"), 0)
	}
	size, _ := tinylfu.Size(ctx)
	printKeyValue("Entries after overfill (max 10)", size)
	tinylfu.Close(ctx)

	printSuccess("All eviction policies demonstrated successfully")
}

// -----------------------------------------------------------------------------
// 7. Resilience Wrappers Demonstration
// -----------------------------------------------------------------------------

func demoResilienceWrappers(ctx context.Context) {
	printSection("RESILIENCE WRAPPERS")

	// Create a backend
	backend, err := memory.New(memory.WithMaxEntries(100))
	if err != nil {
		printError("Backend creation failed: " + err.Error())
		return
	}
	defer backend.Close(ctx)

	// Circuit breaker
	printSubsection("Circuit Breaker")
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	// Simulate failures
	for i := 0; i < 5; i++ {
		if cb.Allow() {
			if i < 2 {
				cb.Failure()
				printInfo("Request failed")
			} else {
				cb.Success()
				printInfo("Request succeeded")
			}
		} else {
			printWarning("Circuit breaker OPEN - request rejected")
		}
	}
	printKeyValue("Circuit breaker state", cb.State().String())
	cb.Reset()
	printKeyValue("After reset", cb.State().String())

	// Rate limiter
	printSubsection("Rate Limiter")
	limiter := resilience.NewLimiterWithConfig(resilience.LimiterConfig{
		ReadRPS:   5,
		ReadBurst: 3,
	})
	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.AllowRead(ctx) {
			allowed++
		}
	}
	printKeyValue("Allowed requests (RPS 5, burst 3)", allowed)

	// Full resilience wrapper
	printSubsection("Full Resilience Wrapper")
	resilientCache := resilience.NewCache(backend, resilience.Options{
		CircuitBreaker: cb,
		Limiter:        limiter,
		Hooks: &resilience.Hooks{
			OnGet: func(ctx context.Context, key string, hit bool, errKind string, d time.Duration) {
				printMetric("Get latency", d)
			},
		},
	})

	_ = resilientCache.Set(ctx, "resilient:key", []byte("value"), 0)
	val, err := resilientCache.Get(ctx, "resilient:key")
	if err != nil {
		printError("Get failed: " + err.Error())
	} else {
		printKeyValue("Resilient cache get", string(val))
	}

	printSuccess("Resilience wrappers demonstrated")
}

// -----------------------------------------------------------------------------
// 8. Key Builder Demonstration
// -----------------------------------------------------------------------------

func demoKeyBuilder(ctx context.Context) {
	printSection("KEY BUILDER")

	// Basic key building
	printSubsection("Basic Key Building")
	kb, err := builder.NewKey("api")
	if err != nil {
		printError("KeyBuilder creation failed: " + err.Error())
	} else {
		key := kb.MustAdd("v2").MustAdd("users").MustAdd("123").Build()
		printKeyValue("Built key", key)
		printKeyValue("Prefix", kb.Prefix())
		printKeyValue("Depth", kb.Depth())
	}

	// Hierarchical keys
	printSubsection("Hierarchical Keys")
	cacheKey, _ := builder.NewKey("cache")
	cacheKey, _ = cacheKey.Add("users")
	cacheKey, _ = cacheKey.Add("profile")
	cacheKey, _ = cacheKey.Add("123")
	printKeyValue("Hierarchical key", cacheKey.Build())

	// Safe building with error handling
	printSubsection("Error Handling")
	kb2, _ := builder.NewKey("app")
	kb2, err = kb2.Add("segment1")
	if err != nil {
		printError("Add failed: " + err.Error())
	}
	// Attempt to add invalid segment (would fail validation)
	_, err = kb2.Add("")
	if err != nil {
		printInfo("Empty segment rejected: " + err.Error())
	}

	printSuccess("Key builder demonstrated")
}

// -----------------------------------------------------------------------------
// 9. Error Handling Demonstration
// -----------------------------------------------------------------------------

func demoErrorHandling(ctx context.Context) {
	printSection("ERROR HANDLING")

	mem, _ := memory.New()
	defer mem.Close(ctx)

	// NotFound error
	printSubsection("NotFound Error")
	_, err := mem.Get(ctx, "non-existent")
	if _errors.IsNotFound(err) {
		printInfo("NotFound correctly detected")
	}
	printKeyValue("Error code", _errors.CodeOf(err))

	// Empty key error
	printSubsection("Empty Key Validation")
	_, err = mem.Get(ctx, "")
	if _errors.Is(err, _errors.CodeEmptyKey) {
		printInfo("EmptyKey correctly detected")
	}
	printKeyValue("Error type", fmt.Sprintf("%T", err))

	// Invalid key (with validation)
	printSubsection("Invalid Key")
	kb, _ := builder.NewKey("prefix")
	_, err = kb.Add("")
	if err != nil {
		printInfo("Invalid segment rejected")
	}

	// Retryable errors
	printSubsection("Retryable Detection")
	retryableErr := _errors.TimeoutError("test")
	printKeyValue("IsTimeout", _errors.IsTimeout(retryableErr))
	printKeyValue("Retryable", _errors.Retryable(retryableErr))

	// Error wrapping
	printSubsection("Error Wrapping")
	wrappedErr := _errors.WrapKey("operation", "key", err)
	if ce, ok := _errors.AsType(wrappedErr); ok {
		printKeyValue("Operation", ce.Operation)
		printKeyValue("Key", ce.Key)
		printKeyValue("Code", ce.Code.String())
	}

	printSuccess("Error handling demonstrated")
}

// -----------------------------------------------------------------------------
// 10. Statistics Demonstration
// -----------------------------------------------------------------------------

func demoStats(ctx context.Context) {
	printSection("STATISTICS")

	mem, _ := memory.New(
		memory.WithMaxEntries(100),
		memory.WithEnableMetrics(true),
	)
	defer mem.Close(ctx)

	// Generate some operations
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("stats:%d", i)
		_ = mem.Set(ctx, key, []byte("value"), 0)
		_, _ = mem.Get(ctx, key)
	}
	// Generate some misses
	for i := 0; i < 5; i++ {
		_, _ = mem.Get(ctx, fmt.Sprintf("miss:%d", i))
	}

	stats := mem.Stats()
	printSubsection("Cache Statistics")
	printMetric("Hits", stats.Hits)
	printMetric("Misses", stats.Misses)
	printMetric("Hit Rate", fmt.Sprintf("%.2f%%", stats.HitRate))
	printMetric("Gets", stats.Gets)
	printMetric("Sets", stats.Sets)
	printMetric("Deletes", stats.Deletes)
	printMetric("Evictions", stats.Evictions)
	printMetric("Errors", stats.Errors)
	printMetric("Items", stats.Items)
	printMetric("Memory (bytes)", stats.Memory)
	printMetric("Ops Per Second", fmt.Sprintf("%.2f", stats.OpsPerSecond))
	printMetric("Uptime", stats.Uptime)

	// Clear and verify stats reset
	_ = mem.Clear(ctx)
	newStats := mem.Stats()
	printKeyValue("After Clear - Items", newStats.Items)

	printSuccess("Statistics demonstrated")
}

// -----------------------------------------------------------------------------
// Footer
// -----------------------------------------------------------------------------

func printFooter() {
	fmt.Printf("\n%s\n", strings.Repeat("━", 80))
	fmt.Println("✅ All demonstrations completed successfully!")
	fmt.Printf("%s\n\n", strings.Repeat("━", 80))
}
