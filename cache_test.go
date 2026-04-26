package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/memory"
)

// ---------------------------------------------------------------------------
// TypedCache tests
// ---------------------------------------------------------------------------

func TestNewTyped_StringCodec(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	if tc == nil {
		t.Fatal("NewTyped returned nil")
	}
	if tc.Name() == "" {
		t.Error("Name() returned empty string")
	}
}

// StringCodec is a minimal codec for testing purposes.
type StringCodec struct{}

func (s *StringCodec) Encode(val string, buf []byte) ([]byte, error) {
	return []byte(val), nil
}

func (s *StringCodec) Decode(data []byte) (string, error) {
	return string(data), nil
}

func (s *StringCodec) Name() string {
	return "string"
}

func TestTypedCache_SetGet(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})

	ctx := context.Background()
	err = tc.Set(ctx, "greeting", "hello, world", 5*time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := tc.Get(ctx, "greeting")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "hello, world" {
		t.Errorf("Get returned %q, want %q", val, "hello, world")
	}
}

func TestTypedCache_Delete(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	tc.Set(ctx, "key", "value", time.Minute)
	err = tc.Delete(ctx, "key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = tc.Get(ctx, "key")
	if err == nil {
		t.Error("Get after Delete should return error")
	}
}

func TestTypedCache_Exists(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	exists, err := tc.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists should return false for missing key")
	}

	tc.Set(ctx, "key", "value", time.Minute)
	exists, err = tc.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists should return true for existing key")
	}
}

func TestTypedCache_GetOrSet(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	// First call: key doesn't exist, fn is called.
	callCount := 0
	val, err := tc.GetOrSet(ctx, "key", func() (string, error) {
		callCount++
		return "loaded", nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if val != "loaded" {
		t.Errorf("GetOrSet returned %q, want %q", val, "loaded")
	}
	if callCount != 1 {
		t.Errorf("fn was called %d times, want 1", callCount)
	}

	// Second call: key exists, fn should not be called.
	val, err = tc.GetOrSet(ctx, "key", func() (string, error) {
		callCount++
		return "should-not-be-called", nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if val != "loaded" {
		t.Errorf("GetOrSet returned %q, want %q", val, "loaded")
	}
	if callCount != 1 {
		t.Errorf("fn was called %d times, want 1", callCount)
	}
}

func TestTypedCache_SetNX(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	ok, err := tc.SetNX(ctx, "key", "first", time.Minute)
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if !ok {
		t.Error("SetNX should return true for new key")
	}

	ok, err = tc.SetNX(ctx, "key", "second", time.Minute)
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if ok {
		t.Error("SetNX should return false for existing key")
	}

	val, _ := tc.Get(ctx, "key")
	if val != "first" {
		t.Errorf("value should be %q, got %q", "first", val)
	}
}

func TestTypedCache_GetMulti(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	tc.Set(ctx, "k1", "v1", time.Minute)
	tc.Set(ctx, "k2", "v2", time.Minute)
	tc.Set(ctx, "k3", "v3", time.Minute)

	result, err := tc.GetMulti(ctx, "k1", "k2", "k4")
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("GetMulti returned %d entries, want 2", len(result))
	}
	if result["k1"] != "v1" {
		t.Errorf("k1 = %q, want %q", result["k1"], "v1")
	}
	if result["k2"] != "v2" {
		t.Errorf("k2 = %q, want %q", result["k2"], "v2")
	}
}

func TestTypedCache_SetMulti(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	items := map[string]string{
		"k1": "v1",
		"k2": "v2",
		"k3": "v3",
	}
	err = tc.SetMulti(ctx, items, time.Minute)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	val, _ := tc.Get(ctx, "k1")
	if val != "v1" {
		t.Errorf("k1 = %q, want %q", val, "v1")
	}
}

func TestTypedCache_DeleteMulti(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	ctx := context.Background()

	tc.Set(ctx, "k1", "v1", time.Minute)
	tc.Set(ctx, "k2", "v2", time.Minute)
	tc.Set(ctx, "k3", "v3", time.Minute)

	err = tc.DeleteMulti(ctx, "k1", "k3")
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	exists, _ := tc.Exists(ctx, "k1")
	if exists {
		t.Error("k1 should be deleted")
	}
	exists, _ = tc.Exists(ctx, "k2")
	if !exists {
		t.Error("k2 should still exist")
	}
	exists, _ = tc.Exists(ctx, "k3")
	if exists {
		t.Error("k3 should be deleted")
	}
}

func TestTypedCache_Close(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}

	tc := NewTyped[string](backend, &StringCodec{})

	if tc.Closed() {
		t.Error("should not be closed initially")
	}

	err = tc.Close(context.Background())
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !tc.Closed() {
		t.Error("should be closed after Close()")
	}
}

func TestTypedCache_Backend(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	tc := NewTyped[string](backend, &StringCodec{})
	returned := tc.Backend()
	if returned != backend {
		t.Error("Backend() should return the same instance")
	}
}

// ---------------------------------------------------------------------------
// Namespace tests
// ---------------------------------------------------------------------------

func TestNamespace_Basic(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	ns, err := NewNamespace("myapp", backend)
	if err != nil {
		t.Fatalf("NewNamespace failed: %v", err)
	}

	ctx := context.Background()

	// Set via namespace.
	err = ns.Set(ctx, "key", []byte("value"), time.Minute)
	if err != nil {
		t.Fatalf("Namespace.Set failed: %v", err)
	}

	// Get via namespace.
	val, err := ns.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Namespace.Get failed: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("Namespace.Get returned %q, want %q", string(val), "value")
	}

	// Direct backend access should see the prefixed key.
	rawVal, err := backend.Get(ctx, "myapp:key")
	if err != nil {
		t.Fatalf("direct Get failed: %v", err)
	}
	if string(rawVal) != "value" {
		t.Errorf("direct Get returned %q, want %q", string(rawVal), "value")
	}
}

func TestNamespace_EmptyPrefix(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	_, err = NewNamespace("", backend)
	if err == nil {
		t.Error("NewNamespace with empty prefix should return error")
	}
}

func TestNamespace_Keys(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	// Put a key outside the namespace.
	backend.Set(context.Background(), "other:key", []byte("other"), time.Minute)

	ns, _ := NewNamespace("myapp", backend)
	ctx := context.Background()

	ns.Set(ctx, "key1", []byte("v1"), time.Minute)
	ns.Set(ctx, "key2", []byte("v2"), time.Minute)

	keys, err := ns.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Keys returned %d keys, want 2", len(keys))
	}
}

// ---------------------------------------------------------------------------
// Manager tests
// ---------------------------------------------------------------------------

func TestManager_NewWithNoBackends(t *testing.T) {
	_, err := NewManager()
	if err == nil {
		t.Error("NewManager with no backends should return error")
	}
}

func TestManager_DefaultBackend(t *testing.T) {
	b, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer b.Close(context.Background())

	mgr, err := NewManager(WithDefaultBackend(b))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close(context.Background())

	ctx := context.Background()
	err = mgr.Set(ctx, "key", []byte("value"), time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := mgr.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("Get returned %q, want %q", string(val), "value")
	}
}

func TestManager_NamedBackends(t *testing.T) {
	b1, _ := NewMemory(memory.WithMaxEntries(100))
	defer b1.Close(context.Background())
	b2, _ := NewMemory(memory.WithMaxEntries(100))
	defer b2.Close(context.Background())

	mgr, _ := NewManager(
		WithNamedBackend("local", b1),
		WithNamedBackend("remote", b2),
		WithDefaultBackend(b1),
	)
	defer mgr.Close(context.Background())

	// Get named backend.
	backend, err := mgr.Backend("remote")
	if err != nil {
		t.Fatalf("Backend('remote') failed: %v", err)
	}
	if backend != b2 {
		t.Error("Backend('remote') should return b2")
	}

	// Get default backend.
	def, err := mgr.Default()
	if err != nil {
		t.Fatalf("Default() failed: %v", err)
	}
	if def != b1 {
		t.Error("Default() should return b1")
	}
}

func TestManager_BackendNotFound(t *testing.T) {
	b, _ := NewMemory(memory.WithMaxEntries(100))
	defer b.Close(context.Background())

	mgr, _ := NewManager(WithNamedBackend("local", b))
	defer mgr.Close(context.Background())

	_, err := mgr.Backend("nonexistent")
	if err == nil {
		t.Error("Backend('nonexistent') should return error")
	}
}

func TestManager_HealthCheck(t *testing.T) {
	b, _ := NewMemory(memory.WithMaxEntries(100))
	defer b.Close(context.Background())

	mgr, _ := NewManager(
		WithNamedBackend("local", b),
	)
	defer mgr.Close(context.Background())

	health := mgr.HealthCheck(context.Background())
	if len(health) != 1 {
		t.Errorf("HealthCheck returned %d entries, want 1", len(health))
	}
	if health["local"] != nil {
		t.Errorf("local backend should be healthy, got error: %v", health["local"])
	}
}

func TestManager_Stats(t *testing.T) {
	b, _ := NewMemory(memory.WithMaxEntries(100))
	defer b.Close(context.Background())

	mgr, _ := NewManager(
		WithNamedBackend("local", b),
	)
	defer mgr.Close(context.Background())

	stats := mgr.Stats()
	if len(stats) != 1 {
		t.Errorf("Stats returned %d entries, want 1", len(stats))
	}
}

// ---------------------------------------------------------------------------
// HotKeyDetector tests
// ---------------------------------------------------------------------------

func TestHotKeyDetector_Basic(t *testing.T) {
	detector := NewHotKeyDetector(
		WithHotKeyThreshold(5),
		WithHotKeyCallback(func(key string, count int64) {
			t.Logf("hot key: %s (count: %d)", key, count)
		}),
	)

	for i := 0; i < 10; i++ {
		detector.Record("popular-key")
	}

	if !detector.IsHot("popular-key") {
		t.Error("popular-key should be hot after 10 accesses")
	}

	if detector.IsHot("unknown-key") {
		t.Error("unknown-key should not be hot")
	}

	count := detector.Count("popular-key")
	if count < 10 {
		t.Errorf("count = %d, want >= 10", count)
	}
}

func TestHotKeyDetector_TopKeys(t *testing.T) {
	detector := NewHotKeyDetector(WithHotKeyThreshold(100))

	// Create some hot keys.
	for i := 0; i < 50; i++ {
		detector.Record("key-a")
	}
	for i := 0; i < 30; i++ {
		detector.Record("key-b")
	}
	for i := 0; i < 10; i++ {
		detector.Record("key-c")
	}

	top := detector.TopKeys(3)
	if len(top) != 3 {
		t.Errorf("TopKeys returned %d entries, want 3", len(top))
	}
	if top[0].Key != "key-a" {
		t.Errorf("top key should be key-a, got %s", top[0].Key)
	}
	if top[0].Count < top[1].Count {
		t.Error("keys should be sorted by count descending")
	}
}

func TestHotKeyDetector_Reset(t *testing.T) {
	detector := NewHotKeyDetector(WithHotKeyThreshold(5))

	detector.Record("key")
	if detector.Size() != 1 {
		t.Errorf("Size() = %d, want 1", detector.Size())
	}

	detector.Reset()
	if detector.Size() != 0 {
		t.Errorf("Size() after Reset() = %d, want 0", detector.Size())
	}
}

// ---------------------------------------------------------------------------
// AdaptiveTTL tests
// ---------------------------------------------------------------------------

func TestAdaptiveTTL_Basic(t *testing.T) {
	adaptive := NewAdaptiveTTL(30*time.Second, 10*time.Minute)

	// Unknown key: should return baseTTL clamped.
	ttl := adaptive.TTL("unknown", 5*time.Minute)
	if ttl < 30*time.Second || ttl > 10*time.Minute {
		t.Errorf("TTL = %v, want within [30s, 10m]", ttl)
	}

	// Record some accesses.
	for i := 0; i < 100; i++ {
		adaptive.RecordAccess("hot-key")
	}

	hotTTL := adaptive.TTL("hot-key", 5*time.Minute)
	// Hot key should get TTL closer to maxTTL.
	if hotTTL <= 5*time.Minute {
		t.Errorf("hot key TTL = %v, should be > base TTL", hotTTL)
	}
}

func TestAdaptiveTTL_Clamp(t *testing.T) {
	adaptive := NewAdaptiveTTL(1*time.Minute, 5*time.Minute)

	// Min clamp.
	ttl := adaptive.TTL("key", 10*time.Second)
	if ttl != 1*time.Minute {
		t.Errorf("TTL below min should be clamped: got %v, want %v", ttl, 1*time.Minute)
	}

	// Max clamp.
	ttl = adaptive.TTL("key", 30*time.Minute)
	if ttl != 5*time.Minute {
		t.Errorf("TTL above max should be clamped: got %v, want %v", ttl, 5*time.Minute)
	}
}

func TestAdaptiveTTL_PanicOnInvalidRange(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAdaptiveTTL with min > max should panic")
		}
	}()
	NewAdaptiveTTL(10*time.Minute, 1*time.Minute)
}

func TestAdaptiveTTL_Reset(t *testing.T) {
	adaptive := NewAdaptiveTTL(30*time.Second, 10*time.Minute)

	adaptive.RecordAccess("key")
	if adaptive.Size() != 1 {
		t.Errorf("Size() = %d, want 1", adaptive.Size())
	}

	adaptive.Reset()
	if adaptive.Size() != 0 {
		t.Errorf("Size() after Reset() = %d, want 0", adaptive.Size())
	}
}

// ---------------------------------------------------------------------------
// Compression tests
// ---------------------------------------------------------------------------

func TestGzipCompressor(t *testing.T) {
	c := NewGzipCompressor(6)
	if c.Name() != "gzip" {
		t.Errorf("Name() = %q, want %q", c.Name(), "gzip")
	}

	data := []byte("hello, world! this is some data that should compress reasonably well. " +
		"repeated patterns help: abcdefghijklmnopqrstuvwxyz abcdefghijklmnopqrstuvwxyz")

	compressed, err := c.Compress(data)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(compressed) >= len(data) {
		t.Errorf("compressed size %d >= original size %d", len(compressed), len(data))
	}

	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if string(decompressed) != string(data) {
		t.Error("decompressed data does not match original")
	}
}

func TestSnappyCompressor(t *testing.T) {
	c := NewSnappyCompressor()
	if c.Name() != "snappy" {
		t.Errorf("Name() = %q, want %q", c.Name(), "snappy")
	}

	data := []byte("hello, world! snappy compression test data here")

	compressed, err := c.Compress(data)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if string(decompressed) != string(data) {
		t.Error("decompressed data does not match original")
	}
}

func TestCompressionMiddleware_SmallValue(t *testing.T) {
	cm := NewCompressionMiddleware(NewGzipCompressor(6), 1024)

	// Small value should not be compressed.
	small := []byte("hi")
	result, err := cm.Compress(small)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if string(result) != string(small) {
		t.Error("small values should not be compressed")
	}
}

func TestCompressionMiddleware_LargeValue(t *testing.T) {
	cm := NewCompressionMiddleware(NewGzipCompressor(6), 64)

	// Large value should be compressed.
	large := make([]byte, 1024)
	for i := range large {
		large[i] = byte(i % 256)
	}

	compressed, err := cm.Compress(large)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(compressed) >= len(large) {
		t.Error("large value should be compressed to smaller size")
	}

	decompressed, err := cm.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if string(decompressed) != string(large) {
		t.Error("decompressed data does not match original")
	}
}

// ---------------------------------------------------------------------------
// Resilience tests
// ---------------------------------------------------------------------------

func TestResilienceConfig(t *testing.T) {
	cfg := &resilienceConfig{}
	WithRetry(5, 200*time.Millisecond)(cfg)
	if cfg.maxAttempts != 5 {
		t.Errorf("maxAttempts = %d, want 5", cfg.maxAttempts)
	}
	if cfg.initialDelay != 200*time.Millisecond {
		t.Errorf("initialDelay = %v, want %v", cfg.initialDelay, 200*time.Millisecond)
	}

	WithCircuitBreaker(10, 60*time.Second)(cfg)
	if cfg.cbThreshold != 10 {
		t.Errorf("cbThreshold = %d, want 10", cfg.cbThreshold)
	}

	WithRateLimit(5000, 500)(cfg)
	if cfg.rateLimitRate != 5000 {
		t.Errorf("rateLimitRate = %f, want 5000", cfg.rateLimitRate)
	}
}

// ---------------------------------------------------------------------------
// Warmer tests
// ---------------------------------------------------------------------------

func TestWarmer_Basic(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	loader := func(keys []string) (map[string][]byte, error) {
		result := make(map[string][]byte, len(keys))
		for _, k := range keys {
			result[k] = []byte("data-for-" + k)
		}
		return result, nil
	}

	warmer := NewWarmer(backend, loader,
		WithWarmerBatchSize(5),
		WithWarmerConcurrency(2),
	)

	err = warmer.Warm(context.Background(), "k1", "k2", "k3")
	if err != nil {
		t.Fatalf("Warm failed: %v", err)
	}

	for _, key := range []string{"k1", "k2", "k3"} {
		val, err := backend.Get(context.Background(), key)
		if err != nil {
			t.Errorf("Get(%q) failed: %v", key, err)
			continue
		}
		expected := "data-for-" + key
		if string(val) != expected {
			t.Errorf("Get(%q) = %q, want %q", key, string(val), expected)
		}
	}
}

func TestWarmer_WarmAll(t *testing.T) {
	backend, err := NewMemory(memory.WithMaxEntries(100))
	if err != nil {
		t.Fatalf("NewMemory failed: %v", err)
	}
	defer backend.Close(context.Background())

	loader := func(keys []string) (map[string][]byte, error) {
		result := make(map[string][]byte, len(keys))
		for _, k := range keys {
			result[k] = []byte("data")
		}
		return result, nil
	}

	warmer := NewWarmer(backend, loader)

	source := func() ([]string, error) {
		return []string{"a", "b", "c", "d", "e"}, nil
	}

	err = warmer.WarmAll(context.Background(), source)
	if err != nil {
		t.Fatalf("WarmAll failed: %v", err)
	}
}

func TestWarmer_EmptyKeys(t *testing.T) {
	backend, _ := NewMemory(memory.WithMaxEntries(100))
	defer backend.Close(context.Background())

	warmer := NewWarmer(backend, func(keys []string) (map[string][]byte, error) {
		return nil, fmt.Errorf("should not be called")
	})

	err := warmer.Warm(context.Background())
	if err != nil {
		t.Errorf("Warm with no keys should not error: %v", err)
	}
}
