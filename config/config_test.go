package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ─── defaults.go ───

func TestDefaultMemory(t *testing.T) {
	c := DefaultMemory()
	if c == nil {
		t.Fatal("DefaultMemory returned nil")
	}
	if c.MaxEntries != 10000 {
		t.Errorf("MaxEntries = %d, want 10000", c.MaxEntries)
	}
	if c.DefaultTTL != 30*time.Minute {
		t.Errorf("DefaultTTL = %v, want 30m", c.DefaultTTL)
	}
}

func TestMemory_SetDefaults(t *testing.T) {
	var c Memory
	c.SetDefaults()
	if c.MaxEntries != 10000 {
		t.Errorf("MaxEntries = %d, want 10000", c.MaxEntries)
	}
	if c.DefaultTTL == 0 {
		t.Error("DefaultTTL should be set after SetDefaults")
	}
	if c.CleanupInterval == 0 {
		t.Error("CleanupInterval should be set")
	}
	if c.ShardCount <= 0 {
		t.Error("ShardCount should be positive")
	}
}

func TestMemory_SetDefaults_MaxMemoryBytes(t *testing.T) {
	tests := []struct {
		name      string
		maxMB     int
		maxBytes  int64
		wantMB    int
		wantBytes int64
	}{
		{"both zero", 0, 0, 100, 100 * 1024 * 1024},
		{"mb set bytes zero", 50, 0, 50, 50 * 1024 * 1024},
		{"mb zero bytes set", 0, 64 * 1024 * 1024, 64, 64 * 1024 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Memory
			c.MaxMemoryMB = tt.maxMB
			c.MaxMemoryBytes = tt.maxBytes
			c.SetDefaults()
			if c.MaxMemoryMB != tt.wantMB {
				t.Errorf("MaxMemoryMB = %d, want %d", c.MaxMemoryMB, tt.wantMB)
			}
			if c.MaxMemoryBytes != tt.wantBytes {
				t.Errorf("MaxMemoryBytes = %d, want %d", c.MaxMemoryBytes, tt.wantBytes)
			}
		})
	}
}

func TestMemory_SetDefaults_NegativeValues(t *testing.T) {
	var c Memory
	c.MaxMemoryMB = -10
	c.MaxMemoryBytes = -5
	c.DefaultTTL = -time.Hour
	c.CleanupInterval = -time.Minute
	c.ShardCount = -3
	c.SetDefaults()
	if c.MaxMemoryMB < 0 {
		t.Errorf("MaxMemoryMB should not be negative, got %d", c.MaxMemoryMB)
	}
	if c.MaxMemoryBytes < 0 {
		t.Errorf("MaxMemoryBytes should not be negative, got %d", c.MaxMemoryBytes)
	}
	if c.DefaultTTL < 0 {
		t.Errorf("DefaultTTL should not be negative, got %v", c.DefaultTTL)
	}
}

func TestMemory_SetDefaults_NonPowerOfTwoShardCount(t *testing.T) {
	var c Memory
	c.ShardCount = 7 // not power of 2
	c.SetDefaults()
	// Should be rounded up to next power of 2
	if c.ShardCount != 8 {
		t.Errorf("ShardCount = %d, want 8 (next power of 2)", c.ShardCount)
	}
}

func TestMemory_SetDefaults_InvalidEvictionPolicy(t *testing.T) {
	var c Memory
	c.EvictionPolicy = EvictionPolicy(99)
	c.SetDefaults()
	if c.EvictionPolicy != EvictLRU {
		t.Errorf("EvictionPolicy = %d, want EvictLRU", c.EvictionPolicy)
	}
}

func TestMemory_Clone(t *testing.T) {
	original := DefaultMemory()
	clone := original.Clone()
	if clone == nil {
		t.Fatal("Clone returned nil")
	}
	if clone.MaxEntries != original.MaxEntries {
		t.Error("Clone should copy MaxEntries")
	}
	clone.MaxEntries = 999
	if original.MaxEntries == 999 {
		t.Error("Clone should be independent")
	}
}

func TestMemory_Clone_Nil(t *testing.T) {
	var c *Memory
	if got := c.Clone(); got != nil {
		t.Error("Clone(nil) should return nil")
	}
}

func TestMemory_Validate(t *testing.T) {
	c := DefaultMemory()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestMemory_ValidateWithContext(t *testing.T) {
	c := DefaultMemory()
	if err := c.ValidateWithContext("test:memory"); err != nil {
		t.Fatalf("ValidateWithContext() error: %v", err)
	}
}

func TestEvictionPolicy_String(t *testing.T) {
	tests := []struct {
		p    EvictionPolicy
		want string
	}{
		{EvictLRU, "lru"},
		{EvictLFU, "lfu"},
		{EvictFIFO, "fifo"},
		{EvictLIFO, "lifo"},
		{EvictMRU, "mru"},
		{EvictRR, "random"},
		{EvictTinyLFU, "tinylfu"},
		{EvictionPolicy(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("EvictionPolicy(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestEvictionPolicy_IsValid(t *testing.T) {
	valid := []EvictionPolicy{EvictLRU, EvictLFU, EvictFIFO, EvictLIFO, EvictMRU, EvictRR, EvictTinyLFU}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("EvictionPolicy(%d) should be valid", p)
		}
	}
	if EvictionPolicy(99).IsValid() {
		t.Error("unknown policy should not be valid")
	}
}

// ─── defaults.go SetDefault* functions ───

func TestSetDefaultInt(t *testing.T) {
	var v int
	SetDefaultInt(&v, 42)
	if v != 42 {
		t.Errorf("got %d, want 42", v)
	}
	v = 10
	SetDefaultInt(&v, 42)
	if v != 10 {
		t.Errorf("got %d, want 10 (already set)", v)
	}
}

func TestSetDefaultInt64(t *testing.T) {
	var v int64
	SetDefaultInt64(&v, 100)
	if v != 100 {
		t.Errorf("got %d, want 100", v)
	}
	v = 50
	SetDefaultInt64(&v, 100)
	if v != 50 {
		t.Errorf("got %d, want 50", v)
	}
}

func TestSetDefaultDuration(t *testing.T) {
	var v time.Duration
	SetDefaultDuration(&v, time.Hour)
	if v != time.Hour {
		t.Errorf("got %v, want 1h", v)
	}
	v = time.Minute
	SetDefaultDuration(&v, time.Hour)
	if v != time.Minute {
		t.Errorf("got %v, want 1m", v)
	}
}

func TestSetDefaultString(t *testing.T) {
	var v string
	SetDefaultString(&v, "default")
	if v != "default" {
		t.Errorf("got %q, want %q", v, "default")
	}
	v = "custom"
	SetDefaultString(&v, "default")
	if v != "custom" {
		t.Errorf("got %q, want %q", v, "custom")
	}
}

func TestSetDefaultBool(t *testing.T) {
	var v bool
	SetDefaultBool(&v, true)
	if !v {
		t.Error("got false, want true")
	}
	v = true
	SetDefaultBool(&v, false)
	// SetDefaultBool only sets if !*field, so true should stay true
	if !v {
		t.Error("existing true should stay true")
	}
}

func TestSetDefaultFloat64(t *testing.T) {
	var v float64
	SetDefaultFloat64(&v, 3.14)
	if v != 3.14 {
		t.Errorf("got %f, want 3.14", v)
	}
	v = 1.0
	SetDefaultFloat64(&v, 3.14)
	if v != 1.0 {
		t.Errorf("got %f, want 1.0", v)
	}
}

// ─── redis.go ───

func TestDefaultRedis(t *testing.T) {
	c := DefaultRedis()
	if c == nil {
		t.Fatal("DefaultRedis returned nil")
	}
	if c.Addr == "" {
		t.Error("Addr should have a default")
	}
	if c.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", c.PoolSize)
	}
	if c.MinIdleConns != 2 {
		t.Errorf("MinIdleConns = %d, want 2", c.MinIdleConns)
	}
	if c.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", c.MaxRetries)
	}
	if c.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", c.DefaultTTL)
	}
	if !c.EnablePipeline {
		t.Error("EnablePipeline should be true")
	}
	if c.TLS == nil {
		t.Error("TLS should not be nil after SetDefaults")
	}
}

func TestRedis_SetDefaults_EdgeCases(t *testing.T) {
	var c Redis
	c.MinIdleConns = 20
	c.PoolSize = 5 // MinIdleConns > PoolSize
	c.MaxRetries = -2
	c.DialTimeout = -1
	c.ReadTimeout = -1
	c.WriteTimeout = -1
	c.DefaultTTL = -1
	c.DB = -5
	c.SetDefaults()
	if c.MinIdleConns > c.PoolSize {
		t.Errorf("MinIdleConns(%d) should not exceed PoolSize(%d)", c.MinIdleConns, c.PoolSize)
	}
	if c.MaxRetries < 0 {
		t.Errorf("MaxRetries = %d, want >= 0", c.MaxRetries)
	}
	if c.DialTimeout <= 0 {
		t.Errorf("DialTimeout = %v, want > 0", c.DialTimeout)
	}
}

func TestRedis_Validate(t *testing.T) {
	c := DefaultRedis()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestRedis_Validate_InvalidAddr(t *testing.T) {
	var c Redis
	c.SetDefaults()
	c.Addr = ""
	if err := c.Validate(); err == nil {
		t.Error("expected error for empty addr")
	}
}

func TestRedis_ValidateWithContext(t *testing.T) {
	c := DefaultRedis()
	if err := c.ValidateWithContext("redis:test"); err != nil {
		t.Fatalf("ValidateWithContext() error: %v", err)
	}
}

func TestRedis_Clone(t *testing.T) {
	c := DefaultRedis()
	clone := c.Clone()
	if clone == nil {
		t.Fatal("Clone returned nil")
	}
	if clone.Addr != c.Addr {
		t.Error("Clone should copy Addr")
	}
	clone.Addr = "different"
	if c.Addr == "different" {
		t.Error("Clone should be independent")
	}
}

func TestRedis_Clone_Nil(t *testing.T) {
	var c *Redis
	if got := c.Clone(); got != nil {
		t.Error("Clone(nil) should return nil")
	}
}

func TestRedisTLS_BuildTLSConfig_Disabled(t *testing.T) {
	tls := &RedisTLS{Enabled: false}
	cfg, err := tls.BuildTLSConfig()
	if err != nil {
		t.Fatalf("BuildTLSConfig() error: %v", err)
	}
	if cfg != nil {
		t.Error("BuildTLSConfig() should return nil when disabled")
	}
}

func TestRedisTLS_BuildTLSConfig_InvalidCertFile(t *testing.T) {
	tls := &RedisTLS{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}
	_, err := tls.BuildTLSConfig()
	if err == nil {
		t.Error("expected error for invalid cert files")
	}
}

func TestRedisTLS_BuildTLSConfig_InvalidCAFile(t *testing.T) {
	tls := &RedisTLS{
		Enabled: true,
		CAFile:  "/nonexistent/ca.pem",
	}
	_, err := tls.BuildTLSConfig()
	if err == nil {
		t.Error("expected error for invalid CA file")
	}
}

func TestRedis_IsClusterMode(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"localhost:6379", false},
		{"[::1]:6379", true},            // IPv6
		{"node1:6379,node2:6379", true}, // comma-separated
		{"", false},
	}
	for _, tt := range tests {
		c := &Redis{Addr: tt.addr}
		if got := c.IsClusterMode(); got != tt.want {
			t.Errorf("IsClusterMode(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestDefaultRedisWithAddress(t *testing.T) {
	c := DefaultRedisWithAddress("redis.example.com:6380")
	if c.Addr != "redis.example.com:6380" {
		t.Errorf("Addr = %q, want %q", c.Addr, "redis.example.com:6380")
	}
}

func TestDefaultRedisCluster(t *testing.T) {
	c := DefaultRedisCluster("node1:6379", "node2:6379")
	if !strings.Contains(c.Addr, ",") {
		t.Errorf("Addr = %q, expected comma-separated addresses", c.Addr)
	}
	if c.PoolSize != 20 {
		t.Errorf("PoolSize = %d, want 20", c.PoolSize)
	}
	if c.MinIdleConns != 5 {
		t.Errorf("MinIdleConns = %d, want 5", c.MinIdleConns)
	}
}

func TestDefaultRedisCluster_NoAddresses(t *testing.T) {
	c := DefaultRedisCluster()
	if c == nil {
		t.Fatal("DefaultRedisCluster returned nil")
	}
	// Should use default addr
	if c.Addr == "" {
		t.Error("Addr should not be empty")
	}
}

// ─── layered.go ───

func TestDefaultLayered(t *testing.T) {
	c := DefaultLayered()
	if c == nil {
		t.Fatal("DefaultLayered returned nil")
	}
	if c.L1Config == nil {
		t.Error("L1Config should not be nil")
	}
	if c.L2Config == nil {
		t.Error("L2Config should not be nil")
	}
	if !c.EnableStats {
		t.Error("EnableStats should be true")
	}
}

func TestLayered_SetDefaults_EdgeCases(t *testing.T) {
	var c Layered
	c.NegativeTTL = -1
	c.SyncBufferSize = -5
	c.WriteBackQueueSize = 0
	c.WriteBackWorkers = -2
	c.L1TTLOverride = -1
	c.SetDefaults()
	if c.NegativeTTL < 0 {
		t.Errorf("NegativeTTL = %v, want >= 0", c.NegativeTTL)
	}
	if c.SyncBufferSize < 0 {
		t.Errorf("SyncBufferSize = %d, want >= 0", c.SyncBufferSize)
	}
	if c.WriteBackQueueSize <= 0 {
		t.Errorf("WriteBackQueueSize = %d, want > 0", c.WriteBackQueueSize)
	}
	if c.WriteBackWorkers <= 0 {
		t.Errorf("WriteBackWorkers = %d, want > 0", c.WriteBackWorkers)
	}
}

func TestLayered_Validate(t *testing.T) {
	c := DefaultLayered()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestLayered_ValidateWithContext(t *testing.T) {
	c := DefaultLayered()
	if err := c.ValidateWithContext("layered:test"); err != nil {
		t.Fatalf("ValidateWithContext() error: %v", err)
	}
}

func TestLayered_Clone(t *testing.T) {
	c := DefaultLayered()
	clone := c.Clone()
	if clone == nil {
		t.Fatal("Clone returned nil")
	}
	if clone.L1Config == nil || clone.L2Config == nil {
		t.Error("Cloned L1Config and L2Config should not be nil")
	}
	clone.L1Config.MaxEntries = 999
	if c.L1Config.MaxEntries == 999 {
		t.Error("Clone should be independent")
	}
}

func TestLayered_Clone_Nil(t *testing.T) {
	var c *Layered
	if got := c.Clone(); got != nil {
		t.Error("Clone(nil) should return nil")
	}
}

func TestDefaultLayeredWithRedis(t *testing.T) {
	c := DefaultLayeredWithRedis("redis:6379")
	if c.L2Config.Addr != "redis:6379" {
		t.Errorf("L2Config.Addr = %q, want %q", c.L2Config.Addr, "redis:6379")
	}
}

func TestDefaultLayeredWithWriteBack(t *testing.T) {
	c := DefaultLayeredWithWriteBack("redis:6379")
	if !c.WriteBack {
		t.Error("WriteBack should be true")
	}
}

func TestDefaultLayeredWithSync(t *testing.T) {
	c := DefaultLayeredWithSync("redis:6379", "my:channel")
	if !c.SyncEnabled {
		t.Error("SyncEnabled should be true")
	}
	if c.SyncChannel != "my:channel" {
		t.Errorf("SyncChannel = %q, want %q", c.SyncChannel, "my:channel")
	}
}

func TestDefaultLayeredWithTTL(t *testing.T) {
	c := DefaultLayeredWithTTL(5*time.Minute, time.Hour, 30*time.Second)
	if c.L1Config.DefaultTTL != 5*time.Minute {
		t.Errorf("L1Config.DefaultTTL = %v, want 5m", c.L1Config.DefaultTTL)
	}
	if c.L2Config.DefaultTTL != time.Hour {
		t.Errorf("L2Config.DefaultTTL = %v, want 1h", c.L2Config.DefaultTTL)
	}
	if c.NegativeTTL != 30*time.Second {
		t.Errorf("NegativeTTL = %v, want 30s", c.NegativeTTL)
	}
}

// ─── root.go ───

func TestConfig_SetDefaults(t *testing.T) {
	var c Config
	c.SetDefaults()
	if c.Memory == nil {
		t.Error("Memory should be set")
	}
	if c.Redis == nil {
		t.Error("Redis should be set")
	}
	if c.Layered == nil {
		t.Error("Layered should be set")
	}
	if c.Resilience.CircuitBreakerThreshold != 5 {
		t.Errorf("CircuitBreakerThreshold = %d, want 5", c.Resilience.CircuitBreakerThreshold)
	}
	if c.Resilience.CircuitBreakerTimeout != 30*time.Second {
		t.Errorf("CircuitBreakerTimeout = %v, want 30s", c.Resilience.CircuitBreakerTimeout)
	}
	if c.Resilience.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", c.Resilience.MaxRetries)
	}
	if c.Observability.MetricsEnabled {
		t.Error("MetricsEnabled should be false by default")
	}
	if c.Observability.TracingEnabled {
		t.Error("TracingEnabled should be false by default")
	}
	if c.Observability.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", c.Observability.LogLevel, "info")
	}
}

func TestConfig_Validate(t *testing.T) {
	var c Config
	c.SetDefaults()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestConfig_Validate_InvalidResilience(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"negative cb threshold", Config{Resilience: ResilienceConfig{CircuitBreakerThreshold: -1}}},
		{"negative cb timeout", Config{Resilience: ResilienceConfig{CircuitBreakerTimeout: -1}}},
		{"negative max retries", Config{Resilience: ResilienceConfig{MaxRetries: -1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.SetDefaults()
			// Override defaults
			if tt.name == "negative cb threshold" {
				tt.cfg.Resilience.CircuitBreakerThreshold = -1
			}
			if tt.name == "negative cb timeout" {
				tt.cfg.Resilience.CircuitBreakerTimeout = -1
			}
			if tt.name == "negative max retries" {
				tt.cfg.Resilience.MaxRetries = -1
			}
			if err := tt.cfg.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	var c Config
	c.SetDefaults()
	c.Observability.LogLevel = "invalid"
	if err := c.Validate(); err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestConfig_Validate_ValidLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", ""}
	for _, level := range levels {
		var c Config
		c.SetDefaults()
		c.Observability.LogLevel = level
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() with LogLevel=%q: %v", level, err)
		}
	}
}

func TestConfig_Validate_NegativeSlowThreshold(t *testing.T) {
	var c Config
	c.SetDefaults()
	c.Observability.SlowThreshold = -1
	if err := c.Validate(); err == nil {
		t.Error("expected error for negative slow threshold")
	}
}

func TestConfigOptions(t *testing.T) {
	t.Run("FromEnv", func(t *testing.T) {
		opt := FromEnv()
		var lo loadOpts
		opt(&lo)
		if !lo.fromEnv {
			t.Error("FromEnv should set fromEnv")
		}
	})
	t.Run("FromMap", func(t *testing.T) {
		m := map[string]string{"test": "value"}
		opt := FromMap(m)
		var lo loadOpts
		opt(&lo)
		if lo.fromMap == nil {
			t.Error("FromMap should set fromMap")
		}
	})
	t.Run("WithDefaults", func(t *testing.T) {
		opt := WithDefaults()
		var lo loadOpts
		opt(&lo)
		if !lo.defaults {
			t.Error("WithDefaults should set defaults")
		}
	})
}

func TestLoadConfig_WithDefaults(t *testing.T) {
	cfg, err := LoadConfig(WithDefaults())
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig() returned nil")
	}
	if cfg.Memory == nil {
		t.Error("Memory config should not be nil")
	}
}

func TestLoadConfig_FromMap(t *testing.T) {
	cfg, err := LoadConfig(WithDefaults(), FromMap(map[string]string{
		"max_entries": "5000",
	}))
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Memory == nil {
		t.Fatal("Memory config should not be nil")
	}
	if cfg.Memory.MaxEntries != 5000 {
		t.Errorf("Memory.MaxEntries = %d, want 5000", cfg.Memory.MaxEntries)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	os.Setenv("CACHE_MAX_ENTRIES", "2000")
	defer os.Unsetenv("CACHE_MAX_ENTRIES")

	cfg, err := LoadConfig(WithDefaults(), FromEnv())
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Memory == nil {
		t.Fatal("Memory config should not be nil")
	}
	if cfg.Memory.MaxEntries != 2000 {
		t.Errorf("Memory.MaxEntries = %d, want 2000", cfg.Memory.MaxEntries)
	}
}

func TestLoadConfig_NoDefaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig() returned nil")
	}
}

// ─── env.go ───

func TestApplyEnv_NilPointer(t *testing.T) {
	err := ApplyEnv(nil)
	if err == nil {
		t.Error("expected error for nil pointer")
	}
}

func TestApplyEnv_NonPointer(t *testing.T) {
	err := ApplyEnv(Config{})
	if err == nil {
		t.Error("expected error for non-pointer")
	}
}

func TestApplyEnv_WithStandaloneStruct(t *testing.T) {
	// ApplyEnv with a standalone Memory struct uses flat CACHE_ prefix
	os.Setenv("CACHE_MAX_ENTRIES", "5000")
	os.Setenv("CACHE_DEFAULT_TTL", "10m")
	os.Setenv("CACHE_MAX_MEMORY_MB", "200")
	os.Setenv("CACHE_ENABLE_METRICS", "true")
	defer func() {
		os.Unsetenv("CACHE_MAX_ENTRIES")
		os.Unsetenv("CACHE_DEFAULT_TTL")
		os.Unsetenv("CACHE_MAX_MEMORY_MB")
		os.Unsetenv("CACHE_ENABLE_METRICS")
	}()

	var mem Memory
	mem.base = *NewBase("memory")
	err := ApplyEnv(&mem)
	if err != nil {
		t.Fatalf("ApplyEnv() error: %v", err)
	}
	if mem.MaxEntries != 5000 {
		t.Errorf("MaxEntries = %d, want 5000", mem.MaxEntries)
	}
	if mem.DefaultTTL != 10*time.Minute {
		t.Errorf("DefaultTTL = %v, want 10m", mem.DefaultTTL)
	}
	if mem.MaxMemoryMB != 200 {
		t.Errorf("MaxMemoryMB = %d, want 200", mem.MaxMemoryMB)
	}
	if !mem.EnableMetrics {
		t.Error("EnableMetrics should be true")
	}
}

func TestApplyEnv_InvalidValues(t *testing.T) {
	os.Setenv("CACHE_MAX_ENTRIES", "not_a_number")
	defer os.Unsetenv("CACHE_MAX_ENTRIES")

	var mem Memory
	mem.base = *NewBase("memory")
	err := ApplyEnv(&mem)
	if err == nil {
		t.Error("expected error for invalid integer")
	}
}

func TestApplyEnv_InvalidDuration(t *testing.T) {
	os.Setenv("CACHE_DEFAULT_TTL", "not-a-duration")
	defer os.Unsetenv("CACHE_DEFAULT_TTL")

	var mem Memory
	mem.base = *NewBase("memory")
	err := ApplyEnv(&mem)
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestApplyEnv_InvalidBool(t *testing.T) {
	os.Setenv("CACHE_ENABLE_METRICS", "maybe")
	defer os.Unsetenv("CACHE_ENABLE_METRICS")

	var mem Memory
	mem.base = *NewBase("memory")
	err := ApplyEnv(&mem)
	if err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestApplyEnv_ObservabilityFields(t *testing.T) {
	// Config is root config, so nested struct prefixes are "" → flat CACHE_ keys
	os.Setenv("CACHE_METRICS_ENABLED", "true")
	os.Setenv("CACHE_LOG_LEVEL", "debug")
	os.Setenv("CACHE_SLOW_THRESHOLD", "50ms")
	defer func() {
		os.Unsetenv("CACHE_METRICS_ENABLED")
		os.Unsetenv("CACHE_LOG_LEVEL")
		os.Unsetenv("CACHE_SLOW_THRESHOLD")
	}()

	var cfg Config
	err := ApplyEnv(&cfg)
	if err != nil {
		t.Fatalf("ApplyEnv() error: %v", err)
	}
	if !cfg.Observability.MetricsEnabled {
		t.Error("MetricsEnabled should be true")
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}
	if cfg.Observability.SlowThreshold != 50*time.Millisecond {
		t.Errorf("SlowThreshold = %v, want 50ms", cfg.Observability.SlowThreshold)
	}
}

func TestApplyEnv_UnsetFieldsSkipped(t *testing.T) {
	// Ensure no env vars for this test
	os.Unsetenv("CACHE_METRICS_ENABLED")
	os.Unsetenv("CACHE_LOG_LEVEL")

	var cfg Config
	cfg.Observability.MetricsEnabled = false
	cfg.Observability.LogLevel = "info"
	err := ApplyEnv(&cfg)
	if err != nil {
		t.Fatalf("ApplyEnv() error: %v", err)
	}
	if cfg.Observability.MetricsEnabled != false {
		t.Error("MetricsEnabled should remain false")
	}
}

// ─── validation.go ───

func TestApply(t *testing.T) {
	c := DefaultMemory()
	if err := Apply(c); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
}

func TestApplyWithEnv_NonValidator(t *testing.T) {
	type Simple struct {
		Name string `env:"CACHE_SIMPLE_NAME"`
	}
	s := &Simple{}
	os.Setenv("CACHE_SIMPLE_NAME", "test")
	defer os.Unsetenv("CACHE_SIMPLE_NAME")

	err := ApplyWithEnv(s)
	if err != nil {
		t.Fatalf("ApplyWithEnv() error: %v", err)
	}
	if s.Name != "test" {
		t.Errorf("Name = %q, want %q", s.Name, "test")
	}
}

func TestApplyWithEnv_WithValidator(t *testing.T) {
	c := DefaultMemory()
	os.Setenv("CACHE_MEMORY_DEFAULT_TTL", "10m")
	defer os.Unsetenv("CACHE_MEMORY_DEFAULT_TTL")

	err := ApplyWithEnv(c)
	if err != nil {
		t.Fatalf("ApplyWithEnv() error: %v", err)
	}
}

func TestApplyWithEnv_InvalidEnv(t *testing.T) {
	c := DefaultMemory()
	os.Setenv("CACHE_MAX_ENTRIES", "invalid")
	defer os.Unsetenv("CACHE_MAX_ENTRIES")

	err := ApplyWithEnv(c)
	if err == nil {
		t.Error("expected error for invalid env value")
	}
}

func TestNewBase(t *testing.T) {
	b := NewBase("test")
	if b == nil {
		t.Fatal("NewBase returned nil")
	}
	if b.opPrefix != "test" {
		t.Errorf("opPrefix = %q, want %q", b.opPrefix, "test")
	}
}

func TestNewBase_EmptyPrefix(t *testing.T) {
	b := NewBase()
	if b.opPrefix != "" {
		t.Errorf("opPrefix = %q, want empty", b.opPrefix)
	}
}

func TestTranslateValidationErrors_Nil(t *testing.T) {
	if translateValidationErrors(nil) != nil {
		t.Error("expected nil for nil error")
	}
}

// ─── tagName helper ───

func TestTagName(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"memory", "memory"},
		{"memory,opt", "memory"},
		{"memory,required", "memory"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := tagName(tt.tag); got != tt.want {
			t.Errorf("tagName(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}
