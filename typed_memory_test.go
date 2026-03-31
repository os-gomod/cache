package cache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/os-gomod/cache/memory"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// ----------------------------------------------------------------------------
// Codec Tests
// ----------------------------------------------------------------------------

func TestJSONCodec(t *testing.T) {
	codec := JSONCodec[User]{}

	user := User{ID: 1, Name: "Alice", Age: 30}
	data, err := codec.Encode(user)
	require.NoError(t, err)

	var decoded User
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, user, decoded)
}

func TestJSONCodec_Decode(t *testing.T) {
	codec := JSONCodec[User]{}

	data := []byte(`{"id":1,"name":"Alice","age":30}`)
	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, 1, decoded.ID)
	assert.Equal(t, "Alice", decoded.Name)
	assert.Equal(t, 30, decoded.Age)
}

func TestJSONCodec_InvalidData(t *testing.T) {
	codec := JSONCodec[User]{}
	_, err := codec.Decode([]byte("invalid json"))
	assert.Error(t, err)
}

func TestStringCodec(t *testing.T) {
	codec := StringCodec{}

	data, err := codec.Encode("hello")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)

	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded)
}

func TestStringCodec_Empty(t *testing.T) {
	codec := StringCodec{}

	data, err := codec.Encode("")
	require.NoError(t, err)
	assert.Equal(t, []byte{}, data)

	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, "", decoded)
}

func TestBytesCodec(t *testing.T) {
	codec := BytesCodec{}

	original := []byte{0x01, 0x02, 0x03}
	data, err := codec.Encode(original)
	require.NoError(t, err)
	assert.Equal(t, original, data)

	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestBytesCodec_Nil(t *testing.T) {
	codec := BytesCodec{}

	data, err := codec.Encode(nil)
	require.NoError(t, err)
	assert.Nil(t, data)

	decoded, err := codec.Decode(nil)
	require.NoError(t, err)
	assert.Nil(t, decoded)
}

func TestInt64Codec(t *testing.T) {
	codec := Int64Codec{}

	tests := []int64{0, 1, -1, 12345, -12345, 9223372036854775807}
	for _, val := range tests {
		data, err := codec.Encode(val)
		require.NoError(t, err)

		decoded, err := codec.Decode(data)
		require.NoError(t, err)
		assert.Equal(t, val, decoded)
	}
}

func TestInt64Codec_InvalidData(t *testing.T) {
	codec := Int64Codec{}
	_, err := codec.Decode([]byte("not a number"))
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// TypedCache Constructor Tests
// ----------------------------------------------------------------------------

func TestNewTypedCache(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	codec := JSONCodec[User]{}
	tc := NewTypedCache(memCache, codec)

	assert.NotNil(t, tc)
	assert.Equal(t, memCache, tc.cache)
	assert.Equal(t, CacheMemory, tc.cacheType)
	assert.NotEmpty(t, tc.name)
}

func TestNewJSONTypedCache(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)

	assert.NotNil(t, tc)
	assert.IsType(t, JSONCodec[User]{}, tc.codec)
}

func TestNewMemoryTypedCache(t *testing.T) {
	tc, err := NewMemoryTypedCache[User](nil)
	require.NoError(t, err)
	assert.NotNil(t, tc)
	defer tc.Close(context.Background())

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, user, got)
}

func TestNewLayeredTypedCache(t *testing.T) {
	tc, err := NewLayeredTypedCache[User](nil)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	defer tc.Close(context.Background())
	assert.NotNil(t, tc)
}

func TestWithOnSetError(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	called := false
	fn := func(_ string, _ error) {
		called = true
	}

	tc := NewJSONTypedCache(memCache, WithOnSetError[User](fn))
	assert.NotNil(t, tc.onSetError)
	assert.False(t, called)
}

func TestDetectCacheType(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	assert.Equal(t, CacheMemory, detectCacheType(memCache))
}

// ----------------------------------------------------------------------------
// TypedCache Basic Operations Tests
// ----------------------------------------------------------------------------

func TestTypedCache_Get_Set(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, user, got)
}

func TestTypedCache_Get_NotFound(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	_, err = tc.Get(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestTypedCache_Delete(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	err = tc.Delete(ctx, "user:1")
	require.NoError(t, err)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
}

func TestTypedCache_Exists(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	exists, err := tc.Exists(ctx, "user:1")
	require.NoError(t, err)
	assert.False(t, exists)

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, 0)
	require.NoError(t, err)

	exists, err = tc.Exists(ctx, "user:1")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTypedCache_TTL(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "user:1", user, time.Minute)
	require.NoError(t, err)

	ttl, err := tc.TTL(ctx, "user:1")
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, time.Minute)
}

// ----------------------------------------------------------------------------
// TypedCache Multi Operations Tests
// ----------------------------------------------------------------------------

func TestTypedCache_GetMulti(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
	}

	for k, v := range users {
		err = tc.Set(ctx, k, v, 0)
		require.NoError(t, err)
	}

	results, err := tc.GetMulti(ctx, "user:1", "user:2", "user:3")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, users["user:1"], results["user:1"])
	assert.Equal(t, users["user:2"], results["user:2"])
}

func TestTypedCache_SetMulti(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
	}

	err = tc.SetMulti(ctx, users, 0)
	require.NoError(t, err)

	for k, v := range users {
		got, getErr := tc.Get(ctx, k)
		require.NoError(t, getErr)
		assert.Equal(t, v, got)
	}
}

func TestTypedCache_DeleteMulti(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice", Age: 30},
		"user:2": {ID: 2, Name: "Bob", Age: 25},
		"user:3": {ID: 3, Name: "Charlie", Age: 35},
	}

	err = tc.SetMulti(ctx, users, 0)
	require.NoError(t, err)

	err = tc.DeleteMulti(ctx, "user:1", "user:2")
	require.NoError(t, err)

	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
	_, err = tc.Get(ctx, "user:2")
	assert.Error(t, err)

	_, err = tc.Get(ctx, "user:3")
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// TypedCache Advanced Operations Tests
// ----------------------------------------------------------------------------

func TestTypedCache_GetOrSet(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	called := 0
	fn := func() (User, error) {
		called++
		return User{ID: 1, Name: "Alice", Age: 30}, nil
	}

	// First call - should compute
	user, err := tc.GetOrSet(ctx, "user:1", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "Alice", user.Name)

	// Second call - should use cache
	user, err = tc.GetOrSet(ctx, "user:1", fn, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "Alice", user.Name)
}

func TestTypedCache_GetOrSet_WithError(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	called := 0
	fn := func() (User, error) {
		called++
		return User{}, assert.AnError
	}

	_, err = tc.GetOrSet(ctx, "user:1", fn, 0)
	assert.Error(t, err)
	assert.Equal(t, 1, called)

	// Should not be cached
	_, err = tc.Get(ctx, "user:1")
	assert.Error(t, err)
}

func TestTypedCache_CompareAndSwap(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}

	err = tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	success, err := tc.CompareAndSwap(ctx, "user:1", oldUser, newUser, 0)
	require.NoError(t, err)
	assert.True(t, success)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, newUser, got)
}

func TestTypedCache_CompareAndSwap_Failure(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}
	wrongUser := User{ID: 1, Name: "Wrong", Age: 99}

	err = tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	success, err := tc.CompareAndSwap(ctx, "user:1", wrongUser, newUser, 0)
	require.NoError(t, err)
	assert.False(t, success)

	got, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, oldUser, got)
}

func TestTypedCache_SetNX(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}

	// First set - should succeed
	success, err := tc.SetNX(ctx, "user:1", user, 0)
	require.NoError(t, err)
	assert.True(t, success)

	// Second set - should fail
	success, err = tc.SetNX(ctx, "user:1", user, 0)
	require.NoError(t, err)
	assert.False(t, success)
}

func TestTypedCache_GetSet(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	oldUser := User{ID: 1, Name: "Alice", Age: 30}
	newUser := User{ID: 1, Name: "Alice Updated", Age: 31}

	err = tc.Set(ctx, "user:1", oldUser, 0)
	require.NoError(t, err)

	got, err := tc.GetSet(ctx, "user:1", newUser, 0)
	require.NoError(t, err)
	assert.Equal(t, oldUser, got)

	current, err := tc.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, newUser, current)
}

// ----------------------------------------------------------------------------
// TypedInt64Cache Tests
// ----------------------------------------------------------------------------

func TestNewTypedInt64Cache(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	assert.NotNil(t, tc)
}

func TestNewMemoryInt64Cache(t *testing.T) {
	tc, err := NewMemoryInt64Cache()
	require.NoError(t, err)
	assert.NotNil(t, tc)
	defer tc.Close(context.Background())

	ctx := context.Background()
	val, err := tc.Increment(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)
}

func TestNewRedisInt64Cache(t *testing.T) {
	tc, err := NewRedisInt64Cache()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	defer tc.Close(context.Background())
	assert.NotNil(t, tc)
}

func TestTypedInt64Cache_Increment(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	ctx := context.Background()

	val, err := tc.Increment(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	val, err = tc.Increment(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestTypedInt64Cache_Decrement(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	ctx := context.Background()

	val, err := tc.Decrement(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(-5), val)

	val, err = tc.Decrement(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(-8), val)
}

// ----------------------------------------------------------------------------
// TypedStringCache Tests
// ----------------------------------------------------------------------------

func TestNewTypedStringCache(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedStringCache(memCache)
	assert.NotNil(t, tc)
}

func TestNewMemoryStringCache(t *testing.T) {
	tc, err := NewMemoryStringCache()
	require.NoError(t, err)
	assert.NotNil(t, tc)
	defer tc.Close(context.Background())

	ctx := context.Background()
	err = tc.Set(ctx, "greeting", "Hello, World!", 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "greeting")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", val)
}

func TestNewRedisStringCache(t *testing.T) {
	tc, err := NewRedisStringCache()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	defer tc.Close(context.Background())
	assert.NotNil(t, tc)
}

// ----------------------------------------------------------------------------
// TypedBytesCache Tests
// ----------------------------------------------------------------------------

func TestNewTypedBytesCache(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedBytesCache(memCache)
	assert.NotNil(t, tc)
}

func TestTypedBytesCache_Operations(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedBytesCache(memCache)
	ctx := context.Background()

	data := []byte{0x01, 0x02, 0x03, 0x04}
	err = tc.Set(ctx, "data", data, 0)
	require.NoError(t, err)

	val, err := tc.Get(ctx, "data")
	require.NoError(t, err)
	assert.Equal(t, data, val)
}

// ----------------------------------------------------------------------------
// Cache Type and Name Tests
// ----------------------------------------------------------------------------

// func TestTypedCache_CacheType(t *testing.T) {
// 	memCache, err := memory.New(memory.WithMaxEntries(100))
// 	require.NoError(t, err)
// 	defer memCache.Close(context.Background())

// 	tc := NewJSONTypedCache[User](memCache)
// 	assert.Equal(t, CacheMemory, tc.CacheType())
// }

func TestTypedCache_Name(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	assert.NotEmpty(t, tc.Name())
}

// ----------------------------------------------------------------------------
// Stats Tests
// ----------------------------------------------------------------------------

func TestTypedCache_Stats(t *testing.T) {
	memCache, err := memory.New(
		memory.WithMaxEntries(100),
		memory.WithEnableMetrics(true),
	)
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}
	_ = tc.Set(ctx, "user:1", user, 0)
	_, _ = tc.Get(ctx, "user:1")

	stats := tc.Stats()
	assert.NotNil(t, stats)
}

// ----------------------------------------------------------------------------
// Lifecycle Tests
// ----------------------------------------------------------------------------

func TestTypedCache_Closed(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)

	tc := NewJSONTypedCache[User](memCache)

	assert.False(t, tc.Closed())

	err = tc.Close(context.Background())
	require.NoError(t, err)
	assert.True(t, tc.Closed())
}

func TestTypedCache_Close(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)

	tc := NewJSONTypedCache[User](memCache)

	err = tc.Close(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	_, err = tc.Get(ctx, "key")
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// Concurrency Tests
// ----------------------------------------------------------------------------

func TestTypedCache_ConcurrentAccess(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(1000))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	ctx := context.Background()

	var wg sync.WaitGroup
	concurrency := 50
	iterations := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = tc.Increment(ctx, "counter", 1)
			}
		}()
	}

	wg.Wait()

	val, err := tc.Get(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(concurrency*iterations), val)
}

// ----------------------------------------------------------------------------
// Edge Cases Tests
// ----------------------------------------------------------------------------

func TestTypedCache_EmptyKey(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	user := User{ID: 1, Name: "Alice", Age: 30}
	err = tc.Set(ctx, "", user, 0)
	assert.Error(t, err)

	_, err = tc.Get(ctx, "")
	assert.Error(t, err)
}

func TestTypedCache_LargeValue(t *testing.T) {
	memCache, err := memory.New(memory.WithMaxEntries(100))
	require.NoError(t, err)
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()

	largeName := make([]byte, 1024*1024)
	for i := range largeName {
		largeName[i] = 'a'
	}
	user := User{ID: 1, Name: string(largeName), Age: 30}

	err = tc.Set(ctx, "large", user, 0)
	require.NoError(t, err)

	got, err := tc.Get(ctx, "large")
	require.NoError(t, err)
	assert.Equal(t, user, got)
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkTypedCache_Set(b *testing.B) {
	memCache, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		b.Fatal(err)
	}
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tc.Set(ctx, "user:1", user, 0)
	}
}

func BenchmarkTypedCache_Get(b *testing.B) {
	memCache, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		b.Fatal(err)
	}
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()
	user := User{ID: 1, Name: "Alice", Age: 30}
	_ = tc.Set(ctx, "user:1", user, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Get(ctx, "user:1")
	}
}

func BenchmarkTypedCache_GetOrSet(b *testing.B) {
	memCache, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		b.Fatal(err)
	}
	defer memCache.Close(context.Background())

	tc := NewJSONTypedCache[User](memCache)
	ctx := context.Background()
	fn := func() (User, error) {
		return User{ID: 1, Name: "Alice", Age: 30}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.GetOrSet(ctx, "user:1", fn, 0)
	}
}

func BenchmarkTypedInt64Cache_Increment(b *testing.B) {
	memCache, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		b.Fatal(err)
	}
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Increment(ctx, "counter", 1)
	}
}

func BenchmarkTypedCache_Parallel(b *testing.B) {
	memCache, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		b.Fatal(err)
	}
	defer memCache.Close(context.Background())

	tc := NewTypedInt64Cache(memCache)
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = tc.Increment(ctx, "counter", 1)
		}
	})
}
