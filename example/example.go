//go:build ignore

// Package example demonstrates usage patterns for the cache library.
// Run standalone: go run ./example/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"

	"github.com/os-gomod/cache"
	"github.com/os-gomod/cache/codec"
	cacheerrors "github.com/os-gomod/cache/errors"
	"github.com/os-gomod/cache/invalidation"
	"github.com/os-gomod/cache/layer"
	"github.com/os-gomod/cache/manager"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/redis"
	"github.com/os-gomod/cache/resilience"
	"github.com/os-gomod/cache/stampede"
)

type Product struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Price     float64           `json:"price"`
	Category  string            `json:"category"`
	Stock     int               `json:"stock"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Version   int64             `json:"version"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
}
type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     float64     `json:"total"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}
type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

func runMemoryCacheDemo(ctx context.Context, chain *observability.Chain) (*memory.Cache, error) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("2. Memory Cache - Multi-Shard with TinyLFU Eviction")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	memCache, err := cache.MemoryWithContext(
		ctx,
		memory.WithMaxEntries(50000),
		memory.WithMaxMemoryMB(512),
		memory.WithTTL(30*time.Minute),
		memory.WithCleanupInterval(5*time.Minute),
		memory.WithShards(64),
		memory.WithTinyLFU(),
		memory.WithOnEvictionPolicy(func(key, reason string) {
			fmt.Printf("  [eviction] key=%s reason=%s\n", key, reason)
		}),
		memory.WithEnableMetrics(true),
		memory.WithInterceptors(chain),
	)
	if err != nil {
		return nil, err
	}
	fmt.Println("  ✅ Memory cache created: 64 shards, TinyLFU eviction, 512MB limit")
	product := Product{
		ID:        "prod-001",
		Name:      "Gaming Laptop",
		Price:     1299.99,
		Category:  "Electronics",
		Stock:     50,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
		Tags:      []string{"gaming", "laptop", "high-performance"},
		Metadata:  map[string]string{"brand": "TechBrand", "warranty": "2 years"},
	}
	productJSON, _ := json.Marshal(product)
	if errSet := memCache.Set(ctx, "product:prod-001", productJSON, 1*time.Hour); errSet != nil {
		return nil, errSet
	}
	fmt.Println("  ✅ Set product:prod-001")
	if val, errGet := memCache.Get(ctx, "product:prod-001"); errGet == nil {
		var retrieved Product
		if errJSON := json.Unmarshal(val, &retrieved); errJSON == nil {
			fmt.Printf("  ✅ Retrieved product: %s - $%.2f\n", retrieved.Name, retrieved.Price)
		}
	}
	return memCache, nil
}

func runTypedCacheDemo(ctx context.Context, memCache *memory.Cache) error {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("3. Typed Cache - Type-Safe Operations")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	productCache := cache.NewJSONTypedCache[Product](memCache,
		cache.WithOnSetError[Product](func(key string, err error) {
			fmt.Printf("  ⚠️ Set error for key %s: %v\n", key, err)
		}),
	)
	newProduct := Product{
		ID:        "prod-002",
		Name:      "Wireless Mouse",
		Price:     49.99,
		Category:  "Electronics",
		Stock:     200,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
		Tags:      []string{"wireless", "mouse", "accessory"},
	}
	if err := productCache.Set(ctx, "product:prod-002", newProduct, 1*time.Hour); err != nil {
		return err
	}
	retrievedProduct, errGet := productCache.Get(ctx, "product:prod-002")
	if errGet != nil {
		return errGet
	}
	fmt.Printf("  ✅ Typed get: %s (Price: $%.2f)\n", retrievedProduct.Name, retrievedProduct.Price)
	return nil
}

func runAtomicOpsDemo(ctx context.Context, memCache *memory.Cache, product Product) error {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("4. Atomic Operations - CAS, SetNX, Increment/Decrement")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	oldProduct := product
	newProductVersion := product
	newProductVersion.Version = 2
	newProductVersion.Stock = 45
	oldJSON, _ := json.Marshal(oldProduct)
	newJSON, _ := json.Marshal(newProductVersion)
	swapped, err := memCache.CompareAndSwap(ctx, "product:prod-001", oldJSON, newJSON, 1*time.Hour)
	if err != nil {
		return err
	}
	fmt.Printf("  ✅ CAS operation: swapped=%v (version updated to 2)\n", swapped)
	set, errSetNX := memCache.SetNX(ctx, "lock:order:123", []byte("processing"), 30*time.Second)
	if errSetNX != nil {
		return errSetNX
	}
	fmt.Printf("  ✅ SetNX (distributed lock): acquired=%v\n", set)
	set, errSetNX = memCache.SetNX(ctx, "lock:order:123", []byte("processing"), 30*time.Second)
	if errSetNX != nil {
		return errSetNX
	}
	fmt.Printf("  ✅ SetNX second attempt: acquired=%v (already locked)\n", set)
	if errSet := memCache.Set(ctx, "counter:page_views", []byte("0"), 0); errSet != nil {
		return errSet
	}
	views, errIncrement := memCache.Increment(ctx, "counter:page_views", 1)
	if errIncrement != nil {
		return errIncrement
	}
	fmt.Printf("  ✅ Increment: page_views = %d\n", views)
	views, errDecrement := memCache.Decrement(ctx, "counter:page_views", 1)
	if errDecrement != nil {
		return errDecrement
	}
	fmt.Printf("  ✅ Decrement: page_views = %d\n", views)
	oldVal, errGetSet := memCache.GetSet(ctx, "counter:page_views", []byte("100"), 0)
	if errGetSet != nil {
		return errGetSet
	}
	fmt.Printf("  ✅ GetSet: old=%s, new=100\n", string(oldVal))
	return nil
}

func runBatchOpsDemo(ctx context.Context, memCache *memory.Cache) error {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("5. Batch Operations - Multi Get/Set/Delete")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	products := map[string]Product{
		"product:003": {ID: "003", Name: "Keyboard", Price: 89.99, Category: "Electronics"},
		"product:004": {ID: "004", Name: "Monitor", Price: 299.99, Category: "Electronics"},
		"product:005": {ID: "005", Name: "Desk", Price: 199.99, Category: "Furniture"},
	}
	items := make(map[string][]byte)
	for k, v := range products {
		data, _ := json.Marshal(v)
		items[k] = data
	}
	if err := memCache.SetMulti(ctx, items, 30*time.Minute); err != nil {
		return err
	}
	fmt.Printf("  ✅ SetMulti: %d products stored\n", len(items))
	results, errGetMulti := memCache.GetMulti(
		ctx,
		"product:003",
		"product:004",
		"product:005",
		"nonexistent",
	)
	if errGetMulti != nil {
		return errGetMulti
	}
	fmt.Printf("  ✅ GetMulti: retrieved %d items (nonexistent key omitted)\n", len(results))
	if errDeleteMulti := memCache.DeleteMulti(ctx, "product:004", "product:005"); errDeleteMulti != nil {
		return errDeleteMulti
	}
	fmt.Println("  ✅ DeleteMulti: removed product:004 and product:005")
	return nil
}

func runRedisCacheDemo(ctx context.Context, chain *observability.Chain) (*redis.Cache, error) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("6. Redis Cache - Distributed with Stampede Protection")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	redisCache, err := cache.RedisWithContext(
		ctx,
		redis.WithAddress("localhost:6379"),
		redis.WithPassword(""),
		redis.WithDB(0),
		redis.WithPoolSize(50),
		redis.WithMinIdleConns(10),
		redis.WithTTL(2*time.Hour),
		redis.WithKeyPrefix("ecomm:"),
		redis.WithMaxRetries(3),
		redis.WithRetryBackoff(100*time.Millisecond),
		redis.WithTimeouts(3*time.Second, 2*time.Second, 2*time.Second),
		redis.WithEnablePipeline(true),
		redis.WithDistributedStampedeProtection(true),
		redis.WithStampedeLockTTL(5*time.Second),
		redis.WithStampedeWaitTimeout(3*time.Second),
		redis.WithStampedeRetryInterval(100*time.Millisecond),
		redis.WithEnableMetrics(true),
		redis.WithInterceptors(chain),
	)
	if err != nil {
		fmt.Println("  ⚠️ Redis not available - skipping distributed features")
		return nil, err
	}
	var loaderCalls int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			val, errGetOrSet := redisCache.GetOrSet(ctx, "global:config", func() ([]byte, error) {
				mu.Lock()
				loaderCalls++
				mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				return json.Marshal(map[string]any{
					"site_name":   "Ecommerce Platform",
					"maintenance": false,
					"version":     "2.0.0",
					"loaded_by":   fmt.Sprintf("goroutine-%d", id),
				})
			}, 5*time.Minute)
			if errGetOrSet == nil {
				var config map[string]any
				if errJSON := json.Unmarshal(val, &config); errJSON == nil && id%20 == 0 {
					fmt.Printf(
						"  ✅ Goroutine %d loaded config (total loader calls: %d)\n",
						id,
						loaderCalls,
					)
				}
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf(
		"  ✅ Distributed stampede protection: 5 concurrent calls → only %d loader executions\n",
		loaderCalls,
	)
	if errHSet := redisCache.HSet(ctx, "user:123", "name", "John Doe"); errHSet != nil {
		return redisCache, nil
	}
	if errHSet1 := redisCache.HSet(ctx, "user:123", "email", "john@example.com"); errHSet1 != nil {
		return redisCache, nil
	}
	name, errHGet := redisCache.HGet(ctx, "user:123", "name")
	if errHGet == nil {
		fmt.Printf("  ✅ Redis Hash: user:123 name = %s\n", string(name))
	}
	if errLPush := redisCache.LPush(ctx, "recent:views", "prod-001", "prod-002", "prod-003"); errLPush == nil {
		recent, _ := redisCache.LRange(ctx, "recent:views", 0, 2)
		fmt.Printf("  ✅ Redis List: recent views = %v\n", recent)
	}
	if errSAdd := redisCache.SAdd(ctx, "category:electronics", "prod-001", "prod-002", "prod-003"); errSAdd == nil {
		members, _ := redisCache.SMembers(ctx, "category:electronics")
		fmt.Printf("  ✅ Redis Set: category:electronics members = %v\n", members)
	}
	if errZAdd := redisCache.ZAdd(ctx, "leaderboard:sales", 1500.0, "product-001"); errZAdd == nil {
		leaderboard, _ := redisCache.ZRange(ctx, "leaderboard:sales", 0, 10)
		fmt.Printf("  ✅ Redis Sorted Set: top products = %v\n", leaderboard)
	}
	return redisCache, nil
}

func runLayeredCacheDemo(ctx context.Context, chain *observability.Chain) error {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("7. Layered Cache - Multi-Tier with Write-Back")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	layeredCache, err := cache.LayeredWithContext(
		ctx,
		layer.WithL1MaxEntries(10000),
		layer.WithL1TTL(5*time.Minute),
		layer.WithL1CleanupInterval(1*time.Minute),
		layer.WithL1Shards(32),
		layer.WithL1LRU(),
		layer.WithL2Address("localhost:6379"),
		layer.WithL2Password(""),
		layer.WithL2DB(0),
		layer.WithL2PoolSize(20),
		layer.WithL2TTL(1*time.Hour),
		layer.WithL2KeyPrefix("layered:"),
		layer.WithPromoteOnHit(true),
		layer.WithWriteBack(true),
		layer.WithWriteBackConfig(1024, 8),
		layer.WithNegativeTTL(10*time.Second),
		layer.WithSyncEnabled(true),
		layer.WithSyncChannel("cache:invalidate"),
		layer.WithSyncBufferSize(500),
		layer.WithInterceptors(chain),
	)
	if err != nil {
		fmt.Println("  ⚠️ Layered cache requires Redis - skipping")
		return err
	}
	defer layeredCache.Close(ctx)
	start := time.Now()
	if errSet := layeredCache.Set(
		ctx,
		"popular:product",
		[]byte(`{"id":"popular","views":1000}`),
		10*time.Minute,
	); errSet != nil {
		return errSet
	}
	fmt.Printf("  ✅ Set to L1 + L2 (write-back: async L2)\n")
	if val, errGet := layeredCache.Get(ctx, "popular:product"); errGet == nil {
		fmt.Printf("  ✅ Get (L1 hit): %s (latency: %v)\n", string(val), time.Since(start))
	}
	if errInvalidate := layeredCache.InvalidateL1(ctx, "popular:product"); errInvalidate != nil {
		return errInvalidate
	}
	fmt.Println("  ✅ L1 invalidated - next get will hit L2")
	if errRefresh := layeredCache.Refresh(ctx, "popular:product"); errRefresh != nil {
		return errRefresh
	}
	fmt.Println("  ✅ Refreshed L1 from L2")
	snap := layeredCache.Stats()
	fmt.Printf("  📊 Layered stats: L1 Hits=%d, L1 Misses=%d, L2 Hits=%d, Promotions=%d\n",
		snap.L1Hits, snap.L1Misses, snap.L2Hits, snap.L2Promotions)
	return nil
}

func runResilienceDemo(
	ctx context.Context,
	memCache *memory.Cache,
	chain *observability.Chain,
) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("8. Resilience Patterns - Circuit Breaker, Rate Limiter, Retry")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	cb := resilience.NewCircuitBreaker(3, 10*time.Second)
	fmt.Printf("  🔌 Circuit Breaker initial state: %s\n", cb.State())
	for i := 0; i < 5; i++ {
		cb.Failure()
	}
	fmt.Printf("  🔌 After 5 failures: state=%s, Allow()=%v\n", cb.State(), cb.Allow())
	cb.Reset()
	fmt.Printf("  🔌 After Reset: state=%s, Allow()=%v\n", cb.State(), cb.Allow())
	limiter := resilience.NewLimiterWithConfig(resilience.LimiterConfig{
		ReadRPS:    1000,
		ReadBurst:  100,
		WriteRPS:   500,
		WriteBurst: 50,
	})
	for i := 0; i < 5; i++ {
		if limiter.AllowRead(ctx) {
			fmt.Printf("  🚦 Read request %d: allowed\n", i+1)
		} else {
			fmt.Printf("  🚦 Read request %d: rate limited\n", i+1)
		}
	}
	retryConfig := resilience.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableErr: func(err error) bool {
			return cacheerrors.IsTimeout(err) || cacheerrors.IsConnectionError(err)
		},
	}
	fmt.Printf("  🔄 Retry config: attempts=%d, initial=%v, max=%v\n",
		retryConfig.MaxAttempts, retryConfig.InitialDelay, retryConfig.MaxDelay)
	policy := resilience.Policy{
		CircuitBreaker: resilience.NewCircuitBreaker(5, 30*time.Second),
		Limiter:        resilience.NewLimiter(500, 50),
		Retry:          retryConfig,
		Timeout:        3 * time.Second,
	}
	resilientCache := resilience.NewCacheWithPolicy(memCache, policy,
		resilience.WithInterceptors(chain),
	)
	fmt.Println("  ✅ Cache wrapped with full resilience policy (CB + RateLimit + Retry + Timeout)")
	if err := resilientCache.Ping(ctx); err == nil {
		fmt.Println("  ✅ Resilient cache ping successful")
	}
}

func runManagerDemo(ctx context.Context, chain *observability.Chain) error {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("9. Cache Manager - Multi-Backend Management")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	hotCache, _ := cache.MemoryWithContext(
		ctx,
		memory.WithMaxEntries(10000),
		memory.WithTTL(5*time.Minute),
	)
	warmCache, _ := cache.MemoryWithContext(
		ctx,
		memory.WithMaxEntries(50000),
		memory.WithTTL(30*time.Minute),
	)
	coldCache, _ := cache.RedisWithContext(
		ctx,
		redis.WithAddress("localhost:6379"),
		redis.WithKeyPrefix("cold:"),
	)
	managerCache, err := cache.New(
		manager.WithDefaultBackend(hotCache),
		manager.WithBackend("warm", warmCache),
		manager.WithBackend("cold", coldCache),
		manager.WithPolicy(resilience.DefaultPolicy()),
		manager.WithInterceptors(chain),
	)
	if err != nil {
		fmt.Println("  ⚠️ Manager requires Redis - skipping cold tier")
		managerCache, _ = cache.New(
			manager.WithDefaultBackend(hotCache),
			manager.WithBackend("warm", warmCache),
			manager.WithPolicy(resilience.DefaultPolicy()),
		)
	}
	defer managerCache.Close(ctx)
	if errSet := managerCache.Set(
		ctx,
		"user:session:123",
		[]byte(`{"user_id":"123","active":true}`),
		15*time.Minute,
	); errSet != nil {
		return errSet
	}
	fmt.Println("  ✅ Set in default (hot) tier")
	warm, _ := managerCache.Backend("warm")
	if errSet1 := warm.Set(
		ctx,
		"analytics:daily",
		[]byte(`{"views":15000,"sales":342}`),
		24*time.Hour,
	); errSet1 != nil {
		return errSet1
	}
	fmt.Println("  ✅ Set in warm tier (longer TTL)")
	health := managerCache.HealthCheck(ctx)
	for name, err := range health {
		if err != nil {
			fmt.Printf("  ⚠️ Backend %s: unhealthy - %v\n", name, err)
		} else {
			fmt.Printf("  ✅ Backend %s: healthy\n", name)
		}
	}
	tenantNS := managerCache.Namespace("tenant:acme:")
	if errSet2 := tenantNS.Set(ctx, "config", []byte(`{"theme":"dark","language":"en"}`), 0); errSet2 != nil {
		return errSet2
	}
	anotherTenant := managerCache.Namespace("tenant:beta:")
	if errSet3 := anotherTenant.Set(ctx, "config", []byte(`{"theme":"light","language":"es"}`), 0); errSet3 != nil {
		return errSet3
	}
	acmeConfig, _ := tenantNS.Get(ctx, "config")
	betaConfig, _ := anotherTenant.Get(ctx, "config")
	fmt.Printf(
		"  ✅ Namespace isolation: ACME=%s, BETA=%s\n",
		string(acmeConfig),
		string(betaConfig),
	)
	return nil
}

func runInvalidationDemo(_ context.Context) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("10. Cache Invalidation - Event Bus")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	bus := invalidation.NewLocalBus()
	var eventsReceived []invalidation.Event
	unsubscribe := bus.Subscribe(invalidation.HandlerFunc(func(evt invalidation.Event) {
		eventsReceived = append(eventsReceived, evt)
		fmt.Printf("  📢 Event received: kind=%s key=%q pattern=%q\n",
			evt.Kind, evt.Key, evt.Pattern)
	}))
	defer unsubscribe()
	bus.Publish(invalidation.Event{
		Kind:      invalidation.KindDelete,
		Key:       "user:123",
		Backend:   "cache",
		Timestamp: time.Now(),
	})
	bus.Publish(invalidation.Event{
		Kind:      invalidation.KindInvalidate,
		Pattern:   "product:*",
		Backend:   "layered",
		Timestamp: time.Now(),
	})
	bus.Publish(invalidation.Event{
		Kind:      invalidation.KindEvict,
		Key:       "session:expired",
		Backend:   "memory",
		Timestamp: time.Now(),
	})
	bus.Publish(invalidation.Event{
		Kind:      invalidation.KindExpire,
		Key:       "temp:key",
		Backend:   "redis",
		Timestamp: time.Now(),
	})
	bus.Publish(invalidation.Event{
		Kind:      invalidation.KindClear,
		Backend:   "all",
		Timestamp: time.Now(),
	})
	fmt.Printf("  ✅ Published %d invalidation events\n", 5)
	time.Sleep(10 * time.Millisecond)
	fmt.Printf("  ✅ Received %d events\n", len(eventsReceived))
}

func runStampedeProtectionDemo(
	ctx context.Context,
	memCache *memory.Cache,
	chain *observability.Chain,
) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("11. Stampede Protection - Local & Distributed")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	var computeCount int
	var computeMu sync.Mutex
	var wgStampede sync.WaitGroup
	for i := 0; i < 5; i++ {
		wgStampede.Add(1)
		go func(id int) {
			defer wgStampede.Done()
			val, err := memCache.GetOrSet(ctx, "stampede:test", func() ([]byte, error) {
				computeMu.Lock()
				computeCount++
				computeMu.Unlock()
				time.Sleep(50 * time.Millisecond)
				return []byte(fmt.Sprintf("computed-value-by-%d", id)), nil
			}, 1*time.Minute)
			if err == nil && id%100 == 0 {
				fmt.Printf("  🎯 Request %d got: %s\n", id, string(val))
			}
		}(i)
	}
	wgStampede.Wait()
	fmt.Printf(
		"  ✅ Local stampede protection: 5 concurrent → only %d computations\n",
		computeCount,
	)
	detector := stampede.NewDetector(1.0, chain)
	defer detector.Close()
	if err := memCache.Set(ctx, "early:refresh", []byte("stale-value"), 5*time.Second); err != nil {
		fmt.Printf("  ⚠️ Failed to set early refresh key: %v\n", err)
	}
	fmt.Println("  ✅ Stampede detector: configured with beta=1.0 (XFetch algorithm)")
}

func runCodecDemo() {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("12. Codec System - Multiple Serialization Formats")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	jsonCodec := codec.NewJSONCodec[Order]()
	order := Order{
		ID:     "ord-001",
		UserID: "user-123",
		Items: []OrderItem{
			{ProductID: "prod-001", Quantity: 2, Price: 1299.99},
			{ProductID: "prod-002", Quantity: 1, Price: 49.99},
		},
		Total:     2649.97,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	encoded, _ := jsonCodec.Encode(order, nil)
	decoded, _ := jsonCodec.Decode(encoded)
	fmt.Printf("  ✅ JSON codec: order encoded to %d bytes, decoded total=%.2f\n",
		len(encoded), decoded.Total)
	stringCodec := codec.StringCodec{}
	strData, _ := stringCodec.Encode("Hello, Cache!", nil)
	strBack, _ := stringCodec.Decode(strData)
	fmt.Printf("  ✅ String codec (zero-alloc): %s\n", strBack)
	int64Codec := codec.Int64Codec{}
	intData, _ := int64Codec.Encode(1234567890, nil)
	intBack, _ := int64Codec.Decode(intData)
	fmt.Printf("  ✅ Int64 codec (zero-alloc): %d\n", intBack)
	floatCodec := codec.Float64Codec{}
	floatData, _ := floatCodec.Encode(3.14159265359, nil)
	floatBack, _ := floatCodec.Decode(floatData)
	fmt.Printf("  ✅ Float64 codec (zero-alloc): %g\n", floatBack)
	rawCodec := codec.RawCodec{}
	rawData := []byte("binary\x00data\x01\x02\x03")
	rawBack, _ := rawCodec.Encode(rawData, nil)
	fmt.Printf("  ✅ Raw codec (passthrough): %d bytes\n", len(rawBack))
}

func runMonitoringDemo(ctx context.Context, memCache *memory.Cache) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("13. Monitoring - Comprehensive Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("stats:key:%d", i%20)
		if i%3 == 0 {
			_, _ = memCache.Get(ctx, key)
		} else {
			_ = memCache.Set(ctx, key, []byte(fmt.Sprintf("value-%d", i)), 1*time.Minute)
		}
	}
	snap := memCache.Stats()
	fmt.Printf("  📊 Overall Statistics:\n")
	fmt.Printf("     Hits: %d | Misses: %d | Hit Rate: %.2f%%\n",
		snap.Hits, snap.Misses, snap.HitRate)
	fmt.Printf("     Gets: %d | Sets: %d | Deletes: %d\n",
		snap.Gets, snap.Sets, snap.Deletes)
	fmt.Printf("     Evictions: %d | Errors: %d\n",
		snap.Evictions, snap.Errors)
	fmt.Printf("     Items: %d | Memory: %d bytes (%.2f MB)\n",
		snap.Items, snap.Memory, float64(snap.Memory)/(1024*1024))
	fmt.Printf("     Ops/Sec: %.2f | Uptime: %v\n",
		snap.OpsPerSecond, snap.Uptime.Round(time.Second))
	analytics := observability.NewHitRateWindow(10*time.Second, 6)
	for i := 0; i < 5; i++ {
		hit := rand.Float64() > 0.3
		analytics.Record(hit)
		analytics.RecordLatency(time.Duration(rand.Int63n(100)) * time.Millisecond)
		if i%100 == 0 {
			analytics.Advance()
		}
	}
	fmt.Printf("\n  📊 Hit Rate Analytics (sliding window):\n")
	fmt.Printf("     Current Hit Rate: %.2f%%\n", analytics.HitRate()*100)
	fmt.Printf("     P50 Latency: %v\n", analytics.P50Latency())
	fmt.Printf("     P99 Latency: %v\n", analytics.P99Latency())
}

func runErrorHandlingDemo(ctx context.Context, memCache *memory.Cache) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("14. Error Handling - Comprehensive Error Types")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	_, err := memCache.Get(ctx, "nonexistent:key:xyz")
	if cacheerrors.IsNotFound(err) {
		fmt.Println("  ✅ NotFound error correctly identified")
	}
	closedCache, _ := cache.MemoryWithContext(ctx)
	_ = closedCache.Close(ctx)
	err = closedCache.Set(ctx, "key", []byte("value"), 0)
	if cacheerrors.IsCacheClosed(err) {
		fmt.Println("  ✅ CacheClosed error correctly identified")
	}
	cbError := cacheerrors.CircuitOpenError("api.call")
	if cacheerrors.IsCircuitOpen(cbError) {
		fmt.Println("  ✅ CircuitOpen error correctly identified")
	}
	rlError := cacheerrors.RateLimitedError("api.call")
	if cacheerrors.IsRateLimited(rlError) {
		fmt.Println("  ✅ RateLimited error correctly identified")
	}
	customErr := cacheerrors.New("payment.process", "order-123", fmt.Errorf("insufficient funds")).
		WithMetadata("attempt", 3).
		WithMetadata("payment_method", "credit_card")
	if code := cacheerrors.CodeOf(customErr); code == cacheerrors.CodeInvalid {
		fmt.Printf("  ✅ Error with metadata: %s (code=%v)\n", customErr.Error(), code)
	}
	if metadata, ok := cacheerrors.GetMetadata(customErr, "attempt"); ok {
		fmt.Printf("  ✅ Error metadata retrieved: attempt=%v\n", metadata)
	}
}

func runDistributedLockDemo(ctx context.Context, redisCache *redis.Cache) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("15. Distributed Lock - Redis-Based Coordination")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	token := stampede.GenerateToken()
	fmt.Printf("  🔐 Generated lock token: %s (len=%d)\n", token[:16]+"...", len(token))
	if redisCache != nil {
		lockKey := "distributed:lock:critical-section"
		var lockWg sync.WaitGroup
		acquiredCount := 0
		var acquireMu sync.Mutex
		for i := 0; i < 5; i++ {
			lockWg.Add(1)
			go func(id int) {
				defer lockWg.Done()
				lockToken := stampede.GenerateToken()
				lock, acquired, lockErr := stampede.AcquireLock(ctx,
					redisCache.Client(),
					lockKey, lockToken, 5*time.Second)
				if lockErr == nil && acquired {
					acquireMu.Lock()
					acquiredCount++
					acquireMu.Unlock()
					time.Sleep(100 * time.Millisecond)
					_ = lock.Release(ctx)
					if id%2 == 0 {
						fmt.Printf("  🔐 Instance %d acquired and released lock\n", id)
					}
				}
			}(i)
		}
		lockWg.Wait()
		fmt.Printf(
			"  ✅ Distributed lock: %d of 5 instances acquired (only 1 at a time)\n",
			acquiredCount,
		)
	} else {
		fmt.Println("  ⚠️ Distributed lock requires Redis - skipping")
	}
}

func runCluster(ctx context.Context) {
	fmt.Println(
		"\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	)
	fmt.Println("16. Redis Cluster Operations")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	// Method 1: Using cache library with cluster addresses
	clusterCache, errSet := redis.New(
		redis.WithAddress("localhost:6379,localhost:6380,localhost:6381"),
		redis.WithPoolSize(50),
		redis.WithMinIdleConns(10),
		redis.WithMaxRetries(3),
		redis.WithTTL(1*time.Hour),
		redis.WithKeyPrefix("cluster:"),
		redis.WithEnablePipeline(true),
	)
	if errSet != nil {
		log.Fatalf("Failed to create cluster cache: %v", errSet)
	}
	defer clusterCache.Close(ctx)

	// Set and Get
	errSet = clusterCache.Set(ctx, "user:123", []byte(`{"name":"Alice","city":"New York"}`), 10*time.Minute)
	if errSet != nil {
		log.Printf("Set error: %v", errSet)
	} else {
		fmt.Println("✅ Set user:123")
	}

	val, errGet := clusterCache.Get(ctx, "user:123")
	if errGet == nil {
		fmt.Printf("✅ Get user:123 = %s\n", string(val))
	}

	// Distributed counter (auto-increment across cluster)
	views, _ := clusterCache.Increment(ctx, "counter:page_views", 1)
	fmt.Printf("✅ Page views: %d\n", views)

	// SetNX for distributed locks
	acquired, _ := clusterCache.SetNX(ctx, "lock:order:456", []byte("processing"), 30*time.Second)
	fmt.Printf("✅ Distributed lock acquired: %v\n\n", acquired)

	if acquired {
		defer clusterCache.Delete(ctx, "lock:order:456")
		// Process order...
	}
}

func printSummary() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           Demo Complete - All Features                       ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Features Demonstrated:                                                      ║")
	fmt.Println("║  ✅ Memory Cache (TinyLFU, sharded)                                          ║")
	fmt.Println("║  ✅ Typed Cache with Generics                                                ║")
	fmt.Println("║  ✅ Atomic Operations (CAS, SetNX, Increment/Decrement)                      ║")
	fmt.Println("║  ✅ Batch Operations (Multi Get/Set/Delete)                                  ║")
	fmt.Println("║  ✅ Redis Cache (Pipeline, Data Structures)                                  ║")
	fmt.Println("║  ✅ Distributed Stampede Protection                                          ║")
	fmt.Println("║  ✅ Layered Cache (L1+L2 with Write-Back)                                    ║")
	fmt.Println("║  ✅ Circuit Breaker & Rate Limiter                                           ║")
	fmt.Println("║  ✅ Retry with Exponential Backoff                                           ║")
	fmt.Println("║  ✅ Cache Manager with Multiple Backends                                     ║")
	fmt.Println("║  ✅ Namespace Isolation                                                      ║")
	fmt.Println("║  ✅ Invalidation Event Bus                                                   ║")
	fmt.Println("║  ✅ Observability (Logging, Metrics, Tracing)                                ║")
	fmt.Println("║  ✅ Comprehensive Error Types                                                ║")
	fmt.Println("║  ✅ Multiple Codecs (JSON, String, Int64, Float64, Raw)                      ║")
	fmt.Println("║  ✅ Distributed Lock                                                         ║")
	fmt.Println("║  ✅ Hit Rate Analytics                                                       ║")
	fmt.Println()
}

func main() {
	ctx := context.Background()
	fmt.Println(
		"╔════════════════════════════════════════════════════════════════════════════════╗",
	)
	fmt.Println(
		"║                    Complete Cache Feature Demo - E-commerce Platform           ║",
	)
	fmt.Println(
		"╚════════════════════════════════════════════════════════════════════════════════╝",
	)
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. Observability Stack Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	loggingInterceptor := observability.NewLoggingInterceptor(
		logger,
		observability.WithSlowThreshold(50*time.Millisecond),
		observability.WithLoggingLevel(slog.LevelDebug),
	)
	promReg := prometheus.NewRegistry()
	promInterceptor, _ := observability.NewPrometheusInterceptor(promReg)
	tracer := otel.Tracer("ecommerce-cache")
	otelInterceptor := observability.NewOTelInterceptor(tracer)
	observabilityChain := observability.NewChain(
		loggingInterceptor,
		promInterceptor,
		otelInterceptor,
	)
	fmt.Println("  ✅ Logging interceptor configured (slow threshold: 50ms)")
	fmt.Println("  ✅ Prometheus metrics interceptor configured")
	fmt.Println("  ✅ OpenTelemetry tracing interceptor configured")
	fmt.Println()
	memCache, err := runMemoryCacheDemo(ctx, observabilityChain)
	if err != nil {
		fmt.Printf("  ❌ Memory cache demo failed: %v\n", err)
		return
	}
	defer memCache.Close(ctx)
	if errCacheDemo := runTypedCacheDemo(ctx, memCache); errCacheDemo != nil {
		fmt.Printf("  ❌ Typed cache demo failed: %v\n", errCacheDemo)
	}
	val, _ := memCache.Get(ctx, "product:prod-001")
	var originalProduct Product
	_ = json.Unmarshal(val, &originalProduct)
	if errOpsDemo := runAtomicOpsDemo(ctx, memCache, originalProduct); errOpsDemo != nil {
		fmt.Printf("  ❌ Atomic ops demo failed: %v\n", errOpsDemo)
	}
	if errBatchDemo := runBatchOpsDemo(ctx, memCache); errBatchDemo != nil {
		fmt.Printf("  ❌ Batch ops demo failed: %v\n", errBatchDemo)
	}
	redisCache, _ := runRedisCacheDemo(ctx, observabilityChain)
	if redisCache != nil {
		defer redisCache.Close(ctx)
	}
	if errCacheDemo := runLayeredCacheDemo(ctx, observabilityChain); errCacheDemo != nil {
		fmt.Printf("  ❌ Layered cache demo failed: %v\n", errCacheDemo)
	}
	runResilienceDemo(ctx, memCache, observabilityChain)
	if errManagerDemo := runManagerDemo(ctx, observabilityChain); errManagerDemo != nil {
		fmt.Printf("  ❌ Manager demo failed: %v\n", errManagerDemo)
	}
	runInvalidationDemo(ctx)
	runStampedeProtectionDemo(ctx, memCache, observabilityChain)
	runCodecDemo()
	runMonitoringDemo(ctx, memCache)
	runErrorHandlingDemo(ctx, memCache)
	runDistributedLockDemo(ctx, redisCache)
	runCluster(ctx)
	printSummary()
}
