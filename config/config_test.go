package config

import (
	"strings"
	"testing"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// ----------------------------------------------------------------------------
// SetDefault helper tests
// ----------------------------------------------------------------------------

func TestSetDefaultInt(t *testing.T) {
	tests := []struct {
		name         string
		initial      int
		defaultValue int
		expected     int
	}{
		{"zero value", 0, 100, 100},
		{"non-zero value", 50, 100, 50},
		{"negative value", -100, 100, -100}, // SetDefaultInt only sets when == 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultInt(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultInt() = %d, want %d", val, tt.expected)
			}
		})
	}
}

func TestSetDefaultInt64(t *testing.T) {
	tests := []struct {
		name         string
		initial      int64
		defaultValue int64
		expected     int64
	}{
		{"zero value", 0, 100, 100},
		{"non-zero value", 50, 100, 50},
		{"negative value", -100, 100, -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultInt64(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultInt64() = %d, want %d", val, tt.expected)
			}
		})
	}
}

func TestSetDefaultDuration(t *testing.T) {
	tests := []struct {
		name         string
		initial      time.Duration
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"zero value", 0, 100 * time.Millisecond, 100 * time.Millisecond},
		{"non-zero value", 50 * time.Millisecond, 100 * time.Millisecond, 50 * time.Millisecond},
		{"negative value", -50 * time.Millisecond, 100 * time.Millisecond, -50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultDuration(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultDuration() = %v, want %v", val, tt.expected)
			}
		})
	}
}

func TestSetDefaultString(t *testing.T) {
	tests := []struct {
		name         string
		initial      string
		defaultValue string
		expected     string
	}{
		{"empty string", "", "default", "default"},
		{"non-empty string", "custom", "default", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultString(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultString() = %s, want %s", val, tt.expected)
			}
		})
	}
}

func TestSetDefaultBool(t *testing.T) {
	tests := []struct {
		name         string
		initial      bool
		defaultValue bool
		expected     bool
	}{
		{"false with default true", false, true, true},
		{"false with default false", false, false, false},
		{"true with default false", true, false, true},
		{"true with default true", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultBool(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultBool() = %v, want %v", val, tt.expected)
			}
		})
	}
}

func TestSetDefaultFloat64(t *testing.T) {
	tests := []struct {
		name         string
		initial      float64
		defaultValue float64
		expected     float64
	}{
		{"zero value", 0.0, 100.5, 100.5},
		{"non-zero value", 50.5, 100.5, 50.5},
		{"negative value", -50.5, 100.5, -50.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.initial
			SetDefaultFloat64(&val, tt.defaultValue)
			if val != tt.expected {
				t.Errorf("SetDefaultFloat64() = %f, want %f", val, tt.expected)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// EvictionPolicy tests
// ----------------------------------------------------------------------------

func TestEvictionPolicyString(t *testing.T) {
	tests := []struct {
		policy   EvictionPolicy
		expected string
	}{
		{EvictLRU, "lru"},
		{EvictLFU, "lfu"},
		{EvictFIFO, "fifo"},
		{EvictLIFO, "lifo"},
		{EvictMRU, "mru"},
		{EvictRR, "random"},
		{EvictTinyLFU, "tinylfu"},
		{EvictionPolicy(100), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.policy.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEvictionPolicyIsValid(t *testing.T) {
	tests := []struct {
		policy   EvictionPolicy
		expected bool
	}{
		{EvictLRU, true},
		{EvictLFU, true},
		{EvictFIFO, true},
		{EvictLIFO, true},
		{EvictMRU, true},
		{EvictRR, true},
		{EvictTinyLFU, true},
		{EvictionPolicy(-1), false},
		{EvictionPolicy(100), false},
	}

	for _, tt := range tests {
		t.Run(tt.policy.String(), func(t *testing.T) {
			if got := tt.policy.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Memory config tests
// ----------------------------------------------------------------------------

func TestMemorySetDefaults(t *testing.T) {
	c := &Memory{}
	c.SetDefaults()

	if c.MaxEntries != 10000 {
		t.Errorf("MaxEntries = %d, want 10000", c.MaxEntries)
	}
	if c.MaxMemoryMB != 100 {
		t.Errorf("MaxMemoryMB = %d, want 100", c.MaxMemoryMB)
	}
	if c.MaxMemoryBytes != 100*1024*1024 {
		t.Errorf("MaxMemoryBytes = %d, want %d", c.MaxMemoryBytes, 100*1024*1024)
	}
	if c.DefaultTTL != 30*time.Minute {
		t.Errorf("DefaultTTL = %v, want %v", c.DefaultTTL, 30*time.Minute)
	}
	if c.CleanupInterval != 5*time.Minute {
		t.Errorf("CleanupInterval = %v, want %v", c.CleanupInterval, 5*time.Minute)
	}
	if c.ShardCount != 32 {
		t.Errorf("ShardCount = %d, want 32", c.ShardCount)
	}
	if c.EvictionPolicy != EvictLRU {
		t.Errorf("EvictionPolicy = %v, want EvictLRU", c.EvictionPolicy)
	}
}

func TestMemorySetDefaultsWithNegativeValues(t *testing.T) {
	c := &Memory{
		MaxEntries:      -100,
		MaxMemoryMB:     -50,
		DefaultTTL:      -1 * time.Hour,
		CleanupInterval: -1 * time.Minute,
		ShardCount:      -5,
	}
	c.SetDefaults()

	// MaxEntries should remain negative because SetDefaultInt only sets when == 0
	if c.MaxEntries != -100 {
		t.Errorf("MaxEntries = %d, want -100", c.MaxEntries)
	}
	// MaxMemoryMB gets set to 0 because negative values are clamped
	if c.MaxMemoryMB != 0 {
		t.Errorf("MaxMemoryMB = %d, want 0", c.MaxMemoryMB)
	}
	if c.MaxMemoryBytes != 0 {
		t.Errorf("MaxMemoryBytes = %d, want 0", c.MaxMemoryBytes)
	}
	// Negative TTL gets set to 0
	if c.DefaultTTL != 0 {
		t.Errorf("DefaultTTL = %v, want 0", c.DefaultTTL)
	}
	// Negative cleanup interval gets set to 0
	if c.CleanupInterval != 0 {
		t.Errorf("CleanupInterval = %v, want 0", c.CleanupInterval)
	}
	// ShardCount should become 32 (positive default)
	if c.ShardCount != 32 {
		t.Errorf("ShardCount = %d, want 32", c.ShardCount)
	}
}

func TestMemorySetDefaultsWithBytesOnly(t *testing.T) {
	c := &Memory{
		MaxMemoryBytes: 200 * 1024 * 1024,
	}
	c.SetDefaults()

	if c.MaxMemoryMB != 200 {
		t.Errorf("MaxMemoryMB = %d, want 200", c.MaxMemoryMB)
	}
	if c.MaxMemoryBytes != 200*1024*1024 {
		t.Errorf("MaxMemoryBytes = %d, want %d", c.MaxMemoryBytes, 200*1024*1024)
	}
}

func TestMemorySetDefaultsWithNonPowerOfTwoShardCount(t *testing.T) {
	c := &Memory{
		ShardCount: 10,
	}
	c.SetDefaults()

	// Should round up to next power of two (16)
	if c.ShardCount != 16 {
		t.Errorf("ShardCount = %d, want 16", c.ShardCount)
	}
}

func TestMemoryValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Memory
		wantError bool
	}{
		{
			name:      "valid default config",
			config:    DefaultMemory(),
			wantError: false,
		},
		{
			name: "valid with custom values",
			config: &Memory{
				MaxEntries:       1000,
				MaxMemoryMB:      50,
				DefaultTTL:       10 * time.Minute,
				CleanupInterval:  1 * time.Minute,
				ShardCount:       16,
				EvictionPolicy:   EvictLFU,
				EnableMetrics:    true,
				OnEvictionPolicy: func(key, reason string) { t.Logf("Evicted key: %s, reason: %s\n", key, reason) },
			},
			wantError: false,
		},
		{
			name: "invalid negative max entries",
			config: &Memory{
				MaxEntries: -1,
				// Need at least one capacity limit
				MaxMemoryMB: 100,
			},
			wantError: true,
		},
		{
			name: "invalid negative max memory MB",
			config: &Memory{
				MaxMemoryMB: -1,
				MaxEntries:  100,
			},
			wantError: true,
		},
		{
			name: "invalid negative max memory bytes",
			config: &Memory{
				MaxMemoryBytes: -1,
				MaxEntries:     100,
			},
			wantError: true,
		},
		{
			name: "no capacity limits",
			config: &Memory{
				MaxEntries:      0,
				MaxMemoryMB:     0,
				MaxMemoryBytes:  0,
				DefaultTTL:      10 * time.Minute,
				CleanupInterval: 1 * time.Minute,
				ShardCount:      32,
				EvictionPolicy:  EvictLRU,
			},
			wantError: true,
		},
		{
			name: "invalid shard count not power of two",
			config: &Memory{
				MaxEntries:      1000,
				ShardCount:      10,
				DefaultTTL:      10 * time.Minute,
				CleanupInterval: 1 * time.Minute,
				EvictionPolicy:  EvictLRU,
			},
			wantError: true,
		},
		{
			name: "invalid eviction policy",
			config: &Memory{
				MaxEntries:      1000,
				DefaultTTL:      10 * time.Minute,
				CleanupInterval: 1 * time.Minute,
				ShardCount:      16,
				EvictionPolicy:  EvictionPolicy(100),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't call SetDefaults for invalid configs - we want to test raw validation
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestMemoryValidateWithContext(t *testing.T) {
	c := &Memory{
		MaxEntries:  -1,
		MaxMemoryMB: 100,
	}
	err := c.ValidateWithContext("test")
	if err == nil {
		t.Error("ValidateWithContext() expected error, got nil")
	}
}

func TestMemoryClone(t *testing.T) {
	original := &Memory{
		MaxEntries:       1000,
		MaxMemoryMB:      50,
		DefaultTTL:       10 * time.Minute,
		CleanupInterval:  1 * time.Minute,
		ShardCount:       32,
		EvictionPolicy:   EvictLFU,
		EnableMetrics:    true,
		OnEvictionPolicy: func(key, reason string) { t.Logf("Evicted key: %s, reason: %s\n", key, reason) },
	}

	clone := original.Clone()

	if clone == original {
		t.Error("Clone() returned same pointer")
	}
	if clone.MaxEntries != original.MaxEntries {
		t.Errorf("MaxEntries = %d, want %d", clone.MaxEntries, original.MaxEntries)
	}
	if clone.MaxMemoryMB != original.MaxMemoryMB {
		t.Errorf("MaxMemoryMB = %d, want %d", clone.MaxMemoryMB, original.MaxMemoryMB)
	}
	if clone.DefaultTTL != original.DefaultTTL {
		t.Errorf("DefaultTTL = %v, want %v", clone.DefaultTTL, original.DefaultTTL)
	}
}

func TestMemoryCloneNil(t *testing.T) {
	var c *Memory
	clone := c.Clone()
	if clone != nil {
		t.Error("Clone() of nil should return nil")
	}
}

func TestDefaultMemory(t *testing.T) {
	c := DefaultMemory()
	if c == nil {
		t.Fatal("DefaultMemory() returned nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("DefaultMemory() validation failed: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Redis config tests
// ----------------------------------------------------------------------------

func TestRedisSetDefaults(t *testing.T) {
	c := &Redis{}
	c.SetDefaults()

	if c.Addr != "localhost:6379" {
		t.Errorf("Addr = %s, want localhost:6379", c.Addr)
	}
	if c.DB != 0 {
		t.Errorf("DB = %d, want 0", c.DB)
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
	if c.RetryBackoff != 100*time.Millisecond {
		t.Errorf("RetryBackoff = %v, want 100ms", c.RetryBackoff)
	}
	if c.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want 5s", c.DialTimeout)
	}
	if c.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want 3s", c.ReadTimeout)
	}
	if c.WriteTimeout != 3*time.Second {
		t.Errorf("WriteTimeout = %v, want 3s", c.WriteTimeout)
	}
	if c.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", c.DefaultTTL)
	}
	if !c.EnablePipeline {
		t.Error("EnablePipeline should be true by default")
	}
	if c.StampedeLockTTL != 5*time.Second {
		t.Errorf("StampedeLockTTL = %v, want 5s", c.StampedeLockTTL)
	}
	if c.StampedeWaitTimeout != 2*time.Second {
		t.Errorf("StampedeWaitTimeout = %v, want 2s", c.StampedeWaitTimeout)
	}
	if c.StampedeRetryInterval != 25*time.Millisecond {
		t.Errorf("StampedeRetryInterval = %v, want 25ms", c.StampedeRetryInterval)
	}
}

func TestRedisSetDefaultsWithNegativeValues(t *testing.T) {
	c := &Redis{
		DB:         -1,
		PoolSize:   -5,
		MaxRetries: -3,
		DefaultTTL: -1 * time.Hour,
	}
	c.SetDefaults()

	if c.DB != 0 {
		t.Errorf("DB = %d, want 0", c.DB)
	}
	// PoolSize remains negative because SetDefaultInt only sets when == 0
	if c.PoolSize != -5 {
		t.Errorf("PoolSize = %d, want -5", c.PoolSize)
	}
	// MaxRetries gets clamped to 0
	if c.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0", c.MaxRetries)
	}
	if c.DefaultTTL != 0 {
		t.Errorf("DefaultTTL = %v, want 0", c.DefaultTTL)
	}
}

func TestRedisSetDefaultsWithMinIdleConnsExceedsPoolSize(t *testing.T) {
	c := &Redis{
		PoolSize:     5,
		MinIdleConns: 10,
	}
	c.SetDefaults()

	if c.MinIdleConns != 5 {
		t.Errorf("MinIdleConns = %d, want 5", c.MinIdleConns)
	}
}

func TestRedisValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Redis
		wantError bool
	}{
		{
			name:      "valid default config",
			config:    DefaultRedis(),
			wantError: false,
		},
		{
			name: "valid with custom values",
			config: &Redis{
				Addr:                                "localhost:6379",
				DB:                                  0,
				PoolSize:                            10,
				MinIdleConns:                        2,
				MaxRetries:                          3,
				RetryBackoff:                        100 * time.Millisecond,
				DialTimeout:                         5 * time.Second,
				ReadTimeout:                         3 * time.Second,
				WriteTimeout:                        3 * time.Second,
				DefaultTTL:                          time.Hour,
				EnablePipeline:                      true,
				EnableDistributedStampedeProtection: true,
				StampedeLockTTL:                     5 * time.Second,
				StampedeWaitTimeout:                 2 * time.Second,
				StampedeRetryInterval:               25 * time.Millisecond,
			},
			wantError: false,
		},
		{
			name: "invalid empty address",
			config: &Redis{
				Addr: "",
			},
			wantError: true,
		},
		{
			name: "invalid negative DB",
			config: &Redis{
				Addr: "localhost:6379",
				DB:   -1,
			},
			wantError: true,
		},
		{
			name: "invalid DB > 15",
			config: &Redis{
				Addr: "localhost:6379",
				DB:   16,
			},
			wantError: true,
		},
		{
			name: "invalid pool size zero",
			config: &Redis{
				Addr:     "localhost:6379",
				PoolSize: 0,
			},
			wantError: true,
		},
		{
			name: "invalid min idle connections > pool size",
			config: &Redis{
				Addr:         "localhost:6379",
				PoolSize:     5,
				MinIdleConns: 10,
			},
			wantError: true,
		},
		{
			name: "invalid dial timeout zero",
			config: &Redis{
				Addr:        "localhost:6379",
				DialTimeout: 0,
			},
			wantError: true,
		},
		{
			name: "invalid stampede lock TTL zero",
			config: &Redis{
				Addr:                                "localhost:6379",
				EnableDistributedStampedeProtection: true,
				StampedeLockTTL:                     0,
				StampedeRetryInterval:               25 * time.Millisecond,
			},
			wantError: true,
		},
		{
			name: "invalid stampede retry interval zero",
			config: &Redis{
				Addr:                                "localhost:6379",
				EnableDistributedStampedeProtection: true,
				StampedeLockTTL:                     5 * time.Second,
				StampedeRetryInterval:               0,
			},
			wantError: true,
		},
		{
			name: "key prefix too long",
			config: &Redis{
				Addr:      "localhost:6379",
				KeyPrefix: string(make([]byte, 300)),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't call SetDefaults for invalid configs - we want to test raw validation
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRedisValidateWithContext(t *testing.T) {
	c := &Redis{
		Addr: "",
	}
	err := c.ValidateWithContext("test")
	if err == nil {
		t.Error("ValidateWithContext() expected error, got nil")
	}
}

func TestRedisClone(t *testing.T) {
	original := &Redis{
		Addr:           "localhost:6379",
		DB:             1,
		PoolSize:       20,
		MinIdleConns:   5,
		MaxRetries:     5,
		DefaultTTL:     2 * time.Hour,
		EnablePipeline: false,
	}

	clone := original.Clone()

	if clone == original {
		t.Error("Clone() returned same pointer")
	}
	if clone.Addr != original.Addr {
		t.Errorf("Addr = %s, want %s", clone.Addr, original.Addr)
	}
	if clone.DB != original.DB {
		t.Errorf("DB = %d, want %d", clone.DB, original.DB)
	}
	if clone.PoolSize != original.PoolSize {
		t.Errorf("PoolSize = %d, want %d", clone.PoolSize, original.PoolSize)
	}
}

func TestRedisCloneNil(t *testing.T) {
	var c *Redis
	clone := c.Clone()
	if clone != nil {
		t.Error("Clone() of nil should return nil")
	}
}

func TestRedisIsClusterMode(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"single address", "localhost:6379", false},
		{"comma-separated", "node1:6379,node2:6379", true},
		{"bracket quoted", "[::1]:6379", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Redis{Addr: tt.addr}
			if got := c.IsClusterMode(); got != tt.expected {
				t.Errorf("IsClusterMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultRedis(t *testing.T) {
	c := DefaultRedis()
	if c == nil {
		t.Fatal("DefaultRedis() returned nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("DefaultRedis() validation failed: %v", err)
	}
}

func TestDefaultRedisWithAddress(t *testing.T) {
	addr := "custom:6379"
	c := DefaultRedisWithAddress(addr)
	if c.Addr != addr {
		t.Errorf("Addr = %s, want %s", c.Addr, addr)
	}
}

func TestDefaultRedisCluster(t *testing.T) {
	addrs := []string{"node1:6379", "node2:6379", "node3:6379"}
	c := DefaultRedisCluster(addrs...)

	if c.Addr != "node1:6379,node2:6379,node3:6379" {
		t.Errorf("Addr = %s, want node1:6379,node2:6379,node3:6379", c.Addr)
	}
	if c.PoolSize != 20 {
		t.Errorf("PoolSize = %d, want 20", c.PoolSize)
	}
	if c.MinIdleConns != 5 {
		t.Errorf("MinIdleConns = %d, want 5", c.MinIdleConns)
	}
}

func TestDefaultRedisClusterEmpty(t *testing.T) {
	c := DefaultRedisCluster()
	if c.Addr != "localhost:6379" {
		t.Errorf("Addr = %s, want localhost:6379", c.Addr)
	}
}

// ----------------------------------------------------------------------------
// Layered config tests
// ----------------------------------------------------------------------------

func TestLayeredSetDefaults(t *testing.T) {
	c := &Layered{}
	c.SetDefaults()

	if c.L1Config == nil {
		t.Error("L1Config should not be nil")
	}
	if c.L2Config == nil {
		t.Error("L2Config should not be nil")
	}
	if !c.PromoteOnHit {
		t.Error("PromoteOnHit should be true by default")
	}
	if c.NegativeTTL != 30*time.Second {
		t.Errorf("NegativeTTL = %v, want 30s", c.NegativeTTL)
	}
	if c.SyncChannel != "cache:invalidate" {
		t.Errorf("SyncChannel = %s, want cache:invalidate", c.SyncChannel)
	}
	if c.SyncBufferSize != 1000 {
		t.Errorf("SyncBufferSize = %d, want 1000", c.SyncBufferSize)
	}
	if c.WriteBackQueueSize != 512 {
		t.Errorf("WriteBackQueueSize = %d, want 512", c.WriteBackQueueSize)
	}
	if c.WriteBackWorkers != 4 {
		t.Errorf("WriteBackWorkers = %d, want 4", c.WriteBackWorkers)
	}
	if !c.EnableStats {
		t.Error("EnableStats should be true by default")
	}
}

func TestLayeredSetDefaultsWithNegativeValues(t *testing.T) {
	c := &Layered{
		NegativeTTL:        -1 * time.Second,
		SyncBufferSize:     -100,
		WriteBackQueueSize: -10,
		WriteBackWorkers:   -5,
		L1TTLOverride:      -1 * time.Minute,
	}
	c.SetDefaults()

	if c.NegativeTTL != 0 {
		t.Errorf("NegativeTTL = %v, want 0", c.NegativeTTL)
	}
	if c.SyncBufferSize != 0 {
		t.Errorf("SyncBufferSize = %d, want 0", c.SyncBufferSize)
	}
	if c.WriteBackQueueSize != 512 {
		t.Errorf("WriteBackQueueSize = %d, want 512", c.WriteBackQueueSize)
	}
	if c.WriteBackWorkers != 4 {
		t.Errorf("WriteBackWorkers = %d, want 4", c.WriteBackWorkers)
	}
	if c.L1TTLOverride != 0 {
		t.Errorf("L1TTLOverride = %v, want 0", c.L1TTLOverride)
	}
}

func TestLayeredSetDefaultsPreservePromoteOnHitFalse(t *testing.T) {
	c := &Layered{
		PromoteOnHit:    false,
		PromoteOnHitSet: true,
	}
	c.SetDefaults()

	if c.PromoteOnHit {
		t.Error("PromoteOnHit should remain false when explicitly set")
	}
}

func TestLayeredValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Layered
		wantError bool
	}{
		{
			name:      "valid default config",
			config:    DefaultLayered(),
			wantError: false,
		},
		{
			name: "valid with sync enabled",
			config: &Layered{
				SyncEnabled:    true,
				SyncChannel:    "custom:channel",
				SyncBufferSize: 500,
			},
			wantError: false,
		},
		{
			name: "valid with write-back enabled",
			config: &Layered{
				WriteBack:          true,
				WriteBackQueueSize: 100,
				WriteBackWorkers:   2,
			},
			wantError: false,
		},
		{
			name: "sync enabled missing channel",
			config: &Layered{
				SyncEnabled:    true,
				SyncChannel:    "",
				SyncBufferSize: 1000,
			},
			wantError: true,
		},
		{
			name: "sync enabled invalid buffer size",
			config: &Layered{
				SyncEnabled:    true,
				SyncChannel:    "channel",
				SyncBufferSize: 0,
			},
			wantError: true,
		},
		{
			name: "write-back invalid queue size",
			config: &Layered{
				WriteBack:          true,
				WriteBackQueueSize: 0,
				WriteBackWorkers:   4,
			},
			wantError: true,
		},
		{
			name: "write-back invalid workers",
			config: &Layered{
				WriteBack:          true,
				WriteBackQueueSize: 512,
				WriteBackWorkers:   0,
			},
			wantError: true,
		},
		{
			name: "invalid negative TTL",
			config: &Layered{
				NegativeTTL: -1 * time.Second,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't call SetDefaults for invalid configs - we want to test raw validation
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestLayeredValidateWithContext(t *testing.T) {
	c := &Layered{
		NegativeTTL: -1 * time.Second,
	}
	err := c.ValidateWithContext("test")
	if err == nil {
		t.Error("ValidateWithContext() expected error, got nil")
	}
}

func TestLayeredClone(t *testing.T) {
	original := &Layered{
		PromoteOnHit:       true,
		WriteBack:          true,
		WriteBackQueueSize: 100,
		WriteBackWorkers:   2,
		NegativeTTL:        30 * time.Second,
		SyncEnabled:        true,
		SyncChannel:        "test",
		SyncBufferSize:     500,
		L1TTLOverride:      10 * time.Minute,
		EnableStats:        true,
		L1Config: &Memory{
			MaxEntries: 5000,
		},
		L2Config: &Redis{
			Addr: "localhost:6379",
		},
	}

	clone := original.Clone()

	if clone == original {
		t.Error("Clone() returned same pointer")
	}
	if clone.L1Config == original.L1Config {
		t.Error("L1Config should be deep copied")
	}
	if clone.L2Config == original.L2Config {
		t.Error("L2Config should be deep copied")
	}
	if clone.PromoteOnHit != original.PromoteOnHit {
		t.Errorf("PromoteOnHit = %v, want %v", clone.PromoteOnHit, original.PromoteOnHit)
	}
}

func TestLayeredCloneNil(t *testing.T) {
	var c *Layered
	clone := c.Clone()
	if clone != nil {
		t.Error("Clone() of nil should return nil")
	}
}

func TestDefaultLayered(t *testing.T) {
	c := DefaultLayered()
	if c == nil {
		t.Fatal("DefaultLayered() returned nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("DefaultLayered() validation failed: %v", err)
	}
}

func TestDefaultLayeredWithRedis(t *testing.T) {
	addr := "custom:6379"
	c := DefaultLayeredWithRedis(addr)
	if c.L2Config.Addr != addr {
		t.Errorf("L2Config.Addr = %s, want %s", c.L2Config.Addr, addr)
	}
}

func TestDefaultLayeredWithWriteBack(t *testing.T) {
	addr := "custom:6379"
	c := DefaultLayeredWithWriteBack(addr)
	if !c.WriteBack {
		t.Error("WriteBack should be true")
	}
	if c.L2Config.Addr != addr {
		t.Errorf("L2Config.Addr = %s, want %s", c.L2Config.Addr, addr)
	}
}

func TestDefaultLayeredWithSync(t *testing.T) {
	addr := "custom:6379"
	channel := "custom:sync"
	c := DefaultLayeredWithSync(addr, channel)

	if !c.SyncEnabled {
		t.Error("SyncEnabled should be true")
	}
	if c.SyncChannel != channel {
		t.Errorf("SyncChannel = %s, want %s", c.SyncChannel, channel)
	}
	if c.L2Config.Addr != addr {
		t.Errorf("L2Config.Addr = %s, want %s", c.L2Config.Addr, addr)
	}
}

func TestDefaultLayeredWithTTL(t *testing.T) {
	l1TTL := 5 * time.Minute
	l2TTL := 30 * time.Minute
	negativeTTL := 10 * time.Second

	c := DefaultLayeredWithTTL(l1TTL, l2TTL, negativeTTL)

	if c.L1Config.DefaultTTL != l1TTL {
		t.Errorf("L1Config.DefaultTTL = %v, want %v", c.L1Config.DefaultTTL, l1TTL)
	}
	if c.L2Config.DefaultTTL != l2TTL {
		t.Errorf("L2Config.DefaultTTL = %v, want %v", c.L2Config.DefaultTTL, l2TTL)
	}
	if c.NegativeTTL != negativeTTL {
		t.Errorf("NegativeTTL = %v, want %v", c.NegativeTTL, negativeTTL)
	}
}

// ----------------------------------------------------------------------------
// Base validation helper tests
// ----------------------------------------------------------------------------

func TestBaseValidateNonNegative(t *testing.T) {
	b := &Base{}
	err := b.validateNonNegative("test", 5, "op")
	if err != nil {
		t.Errorf("validateNonNegative() should not error for positive value: %v", err)
	}

	err = b.validateNonNegative("test", 0, "op")
	if err != nil {
		t.Errorf("validateNonNegative() should not error for zero: %v", err)
	}

	err = b.validateNonNegative("test", -1, "op")
	if err == nil {
		t.Error("validateNonNegative() should error for negative value")
	}
}

func TestBaseValidatePositive(t *testing.T) {
	b := &Base{}
	err := b.validatePositive("test", 5, "op")
	if err != nil {
		t.Errorf("validatePositive() should not error for positive value: %v", err)
	}

	err = b.validatePositive("test", 0, "op")
	if err == nil {
		t.Error("validatePositive() should error for zero")
	}

	err = b.validatePositive("test", -1, "op")
	if err == nil {
		t.Error("validatePositive() should error for negative value")
	}
}

func TestBaseValidatePositiveInt(t *testing.T) {
	b := &Base{}
	err := b.validatePositiveInt("test", 5, "op")
	if err != nil {
		t.Errorf("validatePositiveInt() should not error for positive value: %v", err)
	}

	err = b.validatePositiveInt("test", 0, "op")
	if err == nil {
		t.Error("validatePositiveInt() should error for zero")
	}
}

func TestBaseValidateDuration(t *testing.T) {
	b := &Base{}
	err := b.validateDuration("test", 5*time.Second, "op")
	if err != nil {
		t.Errorf("validateDuration() should not error for positive duration: %v", err)
	}

	err = b.validateDuration("test", 0, "op")
	if err != nil {
		t.Errorf("validateDuration() should not error for zero: %v", err)
	}

	err = b.validateDuration("test", -1*time.Second, "op")
	if err == nil {
		t.Error("validateDuration() should error for negative duration")
	}
}

func TestBaseValidateDurationPositive(t *testing.T) {
	b := &Base{}
	err := b.validateDurationPositive("test", 5*time.Second, "op")
	if err != nil {
		t.Errorf("validateDurationPositive() should not error for positive duration: %v", err)
	}

	err = b.validateDurationPositive("test", 0, "op")
	if err == nil {
		t.Error("validateDurationPositive() should error for zero")
	}

	err = b.validateDurationPositive("test", -1*time.Second, "op")
	if err == nil {
		t.Error("validateDurationPositive() should error for negative duration")
	}
}

func TestBaseValidateRequired(t *testing.T) {
	b := &Base{}
	err := b.validateRequired("test", "value", "op")
	if err != nil {
		t.Errorf("validateRequired() should not error for non-empty string: %v", err)
	}

	err = b.validateRequired("test", "", "op")
	if err == nil {
		t.Error("validateRequired() should error for empty string")
	}
}

func TestBaseValidateMin(t *testing.T) {
	b := &Base{}
	err := b.validateMin("test", 10, 5, "op")
	if err != nil {
		t.Errorf("validateMin() should not error when value >= min: %v", err)
	}

	err = b.validateMin("test", 5, 5, "op")
	if err != nil {
		t.Errorf("validateMin() should not error when value == min: %v", err)
	}

	err = b.validateMin("test", 3, 5, "op")
	if err == nil {
		t.Error("validateMin() should error when value < min")
	}
}

func TestBaseValidateMax(t *testing.T) {
	b := &Base{}
	err := b.validateMax("test", 5, 10, "op")
	if err != nil {
		t.Errorf("validateMax() should not error when value <= max: %v", err)
	}

	err = b.validateMax("test", 10, 10, "op")
	if err != nil {
		t.Errorf("validateMax() should not error when value == max: %v", err)
	}

	err = b.validateMax("test", 15, 10, "op")
	if err == nil {
		t.Error("validateMax() should error when value > max")
	}
}

func TestBaseValidateOneOf(t *testing.T) {
	b := &Base{}
	allowed := []any{"a", "b", "c"}

	err := b.validateOneOf("test", "b", allowed, "op")
	if err != nil {
		t.Errorf("validateOneOf() should not error for allowed value: %v", err)
	}

	err = b.validateOneOf("test", "d", allowed, "op")
	if err == nil {
		t.Error("validateOneOf() should error for disallowed value")
	}
}

func TestBaseOp(t *testing.T) {
	tests := []struct {
		name      string
		base      *Base
		operation string
		expected  string
	}{
		{"with prefix", &Base{opPrefix: "test"}, "validate", "test.validate"},
		{"without prefix", &Base{}, "validate", "validate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.base.op(tt.operation); got != tt.expected {
				t.Errorf("op() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestNewBase(t *testing.T) {
	b := NewBase()
	if b.opPrefix != "" {
		t.Errorf("opPrefix = %s, want empty", b.opPrefix)
	}

	b = NewBase("test")
	if b.opPrefix != "test" {
		t.Errorf("opPrefix = %s, want test", b.opPrefix)
	}

	b = NewBase("")
	if b.opPrefix != "" {
		t.Errorf("opPrefix = %s, want empty", b.opPrefix)
	}
}

// ----------------------------------------------------------------------------
// ValidationErrors tests
// ----------------------------------------------------------------------------

func TestValidationErrors(t *testing.T) {
	ve := &ValidationErrors{}

	if ve.HasErrors() {
		t.Error("HasErrors() should be false for empty ValidationErrors")
	}

	err1 := _errors.New("", "error 1", _errors.ErrNilValue)
	err2 := _errors.New("", "error 2", _errors.ErrNotSupported)

	ve.Add(err1)
	ve.Add(err2)

	if !ve.HasErrors() {
		t.Error("HasErrors() should be true after adding errors")
	}

	if len(ve.Errors()) != 2 {
		t.Errorf("Errors() length = %d, want 2", len(ve.Errors()))
	}

	errStr := ve.Error()
	if !strings.Contains(errStr, "error 1") || !strings.Contains(errStr, "error 2") {
		t.Errorf("Error() = %s, should contain both error messages", errStr)
	}

	unwrapped := ve.Unwrap()
	if len(unwrapped) != 2 {
		t.Errorf("Unwrap() length = %d, want 2", len(unwrapped))
	}
}

func TestValidationErrorsToError(t *testing.T) {
	ve := &ValidationErrors{}
	if ve.ToError() != nil {
		t.Error("ToError() should return nil for empty ValidationErrors")
	}

	ve.Add(_errors.ErrNotSupported)
	if ve.ToError() == nil {
		t.Error("ToError() should return ValidationErrors for non-empty")
	}
}

// ----------------------------------------------------------------------------
// Helper function tests
// ----------------------------------------------------------------------------

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n        int
		expected bool
	}{
		{1, true},
		{2, true},
		{4, true},
		{8, true},
		{16, true},
		{32, true},
		{0, false},
		{3, false},
		{5, false},
		{6, false},
		{10, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			if got := isPowerOfTwo(tt.n); got != tt.expected {
				t.Errorf("isPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.expected)
			}
		})
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		n        int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{6, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{17, 32},
		{31, 32},
		{32, 32},
		{33, 64},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			if got := nextPowerOfTwo(tt.n); got != tt.expected {
				t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.n, got, tt.expected)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Benchmark tests
// ----------------------------------------------------------------------------

func BenchmarkMemorySetDefaults(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := &Memory{}
		c.SetDefaults()
	}
}

func BenchmarkRedisSetDefaults(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := &Redis{}
		c.SetDefaults()
	}
}

func BenchmarkLayeredSetDefaults(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := &Layered{}
		c.SetDefaults()
	}
}

func BenchmarkMemoryValidate(b *testing.B) {
	c := DefaultMemory()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Validate()
	}
}

func BenchmarkRedisValidate(b *testing.B) {
	c := DefaultRedis()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Validate()
	}
}

func BenchmarkLayeredValidate(b *testing.B) {
	c := DefaultLayered()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Validate()
	}
}
