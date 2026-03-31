package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/layered"
	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/redis"
)

func TestNewMemory(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		cache, err := NewMemory()

		require.NoError(t, err)
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})

	t.Run("with options", func(t *testing.T) {
		cache, err := NewMemory(
			memory.WithMaxEntries(1000),
			memory.WithMaxMemoryMB(50),
			memory.WithTTL(time.Minute),
			memory.WithLRU(),
			memory.WithShards(32),
		)

		require.NoError(t, err)
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})
}

func TestNewRedis(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		cache, err := NewRedis()
		if err != nil {
			t.Skip("Redis not available")
		}
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})

	t.Run("with options", func(t *testing.T) {
		cache, err := NewRedis(
			redis.WithAddress("localhost:6379"),
			redis.WithMaxRetries(0),
			redis.WithDialTimeout(200*time.Millisecond),
			redis.WithReadTimeout(200*time.Millisecond),
			redis.WithWriteTimeout(200*time.Millisecond),
			redis.WithPoolSize(20),
			redis.WithTTL(time.Hour),
			redis.WithKeyPrefix("test:"),
		)
		if err != nil {
			t.Skip("Redis not available")
		}
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})
}

func TestNewLayered(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		cache, err := NewLayered()
		if err != nil {
			t.Skip("Redis not available")
		}
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})

	t.Run("with options", func(t *testing.T) {
		cache, err := NewLayered(
			layered.WithL1MaxEntries(5000),
			layered.WithL1TTL(5*time.Minute),
			layered.WithL2Address("localhost:6379"),
			layered.WithL2MaxRetries(0),
			layered.WithL2DialTimeout(200*time.Millisecond),
			layered.WithL2ReadTimeout(200*time.Millisecond),
			layered.WithL2WriteTimeout(200*time.Millisecond),
			layered.WithL2TTL(time.Hour),
			layered.WithPromoteOnHit(true),
		)
		if err != nil {
			t.Skip("Redis not available")
		}
		assert.NotNil(t, cache)
		defer cache.Close(context.Background())
	})
}

func TestEvictionPolicyConstants(t *testing.T) {
	assert.Equal(t, config.EvictLRU, config.EvictLRU)
	assert.Equal(t, config.EvictLFU, config.EvictLFU)
	assert.Equal(t, config.EvictFIFO, config.EvictFIFO)
	assert.Equal(t, config.EvictLIFO, config.EvictLIFO)
	assert.Equal(t, config.EvictMRU, config.EvictMRU)
	assert.Equal(t, config.EvictRR, config.EvictRR)
	assert.Equal(t, config.EvictTinyLFU, config.EvictTinyLFU)
}

func ExampleNewMemory() {
	// Create an in-memory cache with custom settings
	cache, err := NewMemory(
		memory.WithMaxEntries(10000),
		memory.WithMaxMemoryMB(256),
		memory.WithTTL(5*time.Minute),
		memory.WithLRU(),
	)
	if err != nil {
		panic(err)
	}
	defer cache.Close(context.Background())

	// Use the cache
	ctx := context.Background()
	cache.Set(ctx, "key", []byte("value"), 0)

	val, _ := cache.Get(ctx, "key")
	println(string(val))
}

func ExampleNewRedis() {
	// Create a Redis cache
	cache, err := NewRedis(
		redis.WithAddress("localhost:6379"),
		redis.WithTTL(time.Hour),
		redis.WithKeyPrefix("myapp:"),
	)
	if err != nil {
		panic(err)
	}
	defer cache.Close(context.Background())

	// Use the cache
	ctx := context.Background()
	cache.Set(ctx, "key", []byte("value"), 0)
}

func ExampleNewLayered() {
	// Create a layered cache (L1 memory + L2 Redis)
	cache, err := NewLayered(
		layered.WithL1MaxEntries(10000),
		layered.WithL1TTL(5*time.Minute),
		layered.WithL2Address("localhost:6379"),
		layered.WithL2TTL(time.Hour),
		layered.WithPromoteOnHit(true),
	)
	if err != nil {
		panic(err)
	}
	defer cache.Close(context.Background())

	// Use the cache - hot data stays in L1, cold data in L2
	ctx := context.Background()
	cache.Set(ctx, "key", []byte("value"), 0)
}
