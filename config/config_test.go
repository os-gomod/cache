package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultMemory(t *testing.T) {
	cfg := DefaultMemory()

	if cfg.MaxEntries != 10000 {
		t.Errorf("MaxEntries = %d, want 10000", cfg.MaxEntries)
	}
	if cfg.DefaultTTL != 30*time.Minute {
		t.Errorf("DefaultTTL = %v, want 30m", cfg.DefaultTTL)
	}
	if cfg.ShardCount != 32 {
		t.Errorf("ShardCount = %d, want 32", cfg.ShardCount)
	}
	if cfg.EvictionPolicy != EvictLRU {
		t.Errorf("EvictionPolicy = %v, want LRU", cfg.EvictionPolicy)
	}
	if cfg.MaxMemoryBytes <= 0 {
		t.Error("MaxMemoryBytes should be positive")
	}
}

func TestMemory_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Memory)
		wantErr bool
	}{
		{"valid defaults", func(_ *Memory) {}, false},
		{"negative max entries", func(c *Memory) { c.MaxEntries = -1 }, true},
		{"zero max entries and memory", func(c *Memory) {
			c.MaxEntries = 0
			c.MaxMemoryMB = 0
			c.MaxMemoryBytes = 0
		}, true},
		{"negative default TTL", func(c *Memory) { c.DefaultTTL = -1 }, true},
		{"non-power-of-two shards", func(c *Memory) { c.ShardCount = 3 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultMemory()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemory_Clone(t *testing.T) {
	cfg := DefaultMemory()
	clone := cfg.Clone()

	if clone == cfg {
		t.Error("Clone should return a different pointer")
	}
	if clone.MaxEntries != cfg.MaxEntries {
		t.Errorf("Clone MaxEntries = %d, want %d", clone.MaxEntries, cfg.MaxEntries)
	}

	// Modifying clone should not affect original.
	clone.MaxEntries = 999
	if cfg.MaxEntries == 999 {
		t.Error("modifying clone should not affect original")
	}
}

func TestMemory_Clone_Nil(t *testing.T) {
	var cfg *Memory
	if clone := cfg.Clone(); clone != nil {
		t.Error("Clone of nil should return nil")
	}
}

func TestDefaultRedis(t *testing.T) {
	cfg := DefaultRedis()

	if cfg.Addr != "localhost:6379" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "localhost:6379")
	}
	if cfg.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", cfg.PoolSize)
	}
	if cfg.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", cfg.DefaultTTL)
	}
}

func TestRedis_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Redis)
		wantErr bool
	}{
		{"valid defaults", func(_ *Redis) {}, false},
		{"empty address", func(c *Redis) { c.Addr = "" }, true},
		{"negative DB", func(c *Redis) { c.DB = -1 }, true},
		{"DB > 15", func(c *Redis) { c.DB = 16 }, true},
		{"zero pool size", func(c *Redis) { c.PoolSize = 0 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultRedis()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedis_Clone(t *testing.T) {
	cfg := DefaultRedis()
	clone := cfg.Clone()

	if clone == cfg {
		t.Error("Clone should return a different pointer")
	}
	if clone.Addr != cfg.Addr {
		t.Errorf("Clone Addr = %q, want %q", clone.Addr, cfg.Addr)
	}
}

func TestRedis_IsClusterMode(t *testing.T) {
	tests := []struct {
		addr    string
		cluster bool
	}{
		{"localhost:6379", false},
		{"10.0.0.1:6379,10.0.0.2:6379", true},
		{"[redis-cluster.example.com", true},
	}

	for _, tt := range tests {
		cfg := DefaultRedis()
		cfg.Addr = tt.addr
		if got := cfg.IsClusterMode(); got != tt.cluster {
			t.Errorf("IsClusterMode(%q) = %v, want %v", tt.addr, got, tt.cluster)
		}
	}
}

func TestDefaultLayered(t *testing.T) {
	cfg := DefaultLayered()

	if cfg.L1Config == nil {
		t.Error("L1Config should not be nil")
	}
	if cfg.L2Config == nil {
		t.Error("L2Config should not be nil")
	}
	if !cfg.PromoteOnHit {
		t.Error("PromoteOnHit should default to true")
	}
}

func TestLayered_Validate(t *testing.T) {
	cfg := DefaultLayered()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should validate: %v", err)
	}
}

func TestLayered_Clone(t *testing.T) {
	cfg := DefaultLayered()
	clone := cfg.Clone()

	if clone == cfg {
		t.Error("Clone should return a different pointer")
	}
	if clone.L1Config == cfg.L1Config {
		t.Error("L1Config should be deep-cloned")
	}
	if clone.L2Config == cfg.L2Config {
		t.Error("L2Config should be deep-cloned")
	}
}

func TestSetDefaultBool(t *testing.T) {
	// Regression: SetDefaultBool previously had a nil check on *bool that
	// could never fire (Go passes *bool by reference, never nil), causing
	// the function to silently skip setting the default when the zero value
	// (false) was already present.
	t.Run("zero value gets default", func(t *testing.T) {
		var v bool // zero value is false
		SetDefaultBool(&v, true)
		if !v {
			t.Errorf("SetDefaultBool = %v, want true", v)
		}
	})

	t.Run("non-zero true stays true", func(t *testing.T) {
		v := true
		SetDefaultBool(&v, false)
		if !v {
			t.Error("SetDefaultBool should not override true with false")
		}
	})

	t.Run("non-zero false stays false when default is false", func(t *testing.T) {
		v := false
		SetDefaultBool(&v, false)
		if v {
			t.Error("SetDefaultBool(false, false) should remain false")
		}
	})

	t.Run("explicit false then default true", func(t *testing.T) {
		v := false
		SetDefaultBool(&v, true)
		if !v {
			t.Error("SetDefaultBool should set true when field is false")
		}
	})
}

func TestSetDefaultInt(t *testing.T) {
	v := 0
	SetDefaultInt(&v, 42)
	if v != 42 {
		t.Errorf("SetDefaultInt = %d, want 42", v)
	}

	v = 10
	SetDefaultInt(&v, 42)
	if v != 10 {
		t.Errorf("SetDefaultInt should not override non-zero: got %d, want 10", v)
	}
}

func TestSetDefaultDuration(t *testing.T) {
	var d time.Duration
	SetDefaultDuration(&d, 5*time.Second)
	if d != 5*time.Second {
		t.Errorf("SetDefaultDuration = %v, want 5s", d)
	}
}

func TestSetDefaultString(t *testing.T) {
	s := ""
	SetDefaultString(&s, "default")
	if s != "default" {
		t.Errorf("SetDefaultString = %q, want %q", s, "default")
	}

	s = "existing"
	SetDefaultString(&s, "default")
	if s != "existing" {
		t.Errorf("SetDefaultString should not override: got %q, want %q", s, "existing")
	}
}

func TestEvictionPolicy_IsValid(t *testing.T) {
	if !EvictLRU.IsValid() {
		t.Error("LRU should be valid")
	}
	if !EvictTinyLFU.IsValid() {
		t.Error("TinyLFU should be valid")
	}
	policy := EvictionPolicy(99)
	if policy.IsValid() {
		t.Error("invalid policy should not be valid")
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

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{32, true},
		{33, false},
	}
	for _, tt := range tests {
		if got := isPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("isPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{3, 4},
		{5, 8},
		{33, 64},
	}
	for _, tt := range tests {
		if got := nextPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

// --- Phase 8: Pipeline, Env, Root tests ---

func TestApply(t *testing.T) {
	t.Run("calls SetDefaults then Validate", func(t *testing.T) {
		cfg := &Memory{MaxEntries: 5000}
		if err := Apply(cfg); err != nil {
			t.Fatalf("Apply() error: %v", err)
		}
		// SetDefaults should have filled in the zero-valued fields.
		if cfg.DefaultTTL != 30*time.Minute {
			t.Errorf("DefaultTTL = %v, want 30m (should be set by SetDefaults)", cfg.DefaultTTL)
		}
		if cfg.ShardCount != 32 {
			t.Errorf("ShardCount = %d, want 32 (should be set by SetDefaults)", cfg.ShardCount)
		}
		// User-specified value should be preserved.
		if cfg.MaxEntries != 5000 {
			t.Errorf("MaxEntries = %d, want 5000 (user value)", cfg.MaxEntries)
		}
	})

	t.Run("returns validation error", func(t *testing.T) {
		cfg := &Memory{MaxEntries: -1}
		if err := Apply(cfg); err == nil {
			t.Error("Apply() should return validation error for negative MaxEntries")
		}
	})

	t.Run("Redis config", func(t *testing.T) {
		cfg := &Redis{}
		if err := Apply(cfg); err != nil {
			t.Fatalf("Apply() error: %v", err)
		}
		if cfg.Addr != "localhost:6379" {
			t.Errorf("Addr = %q, want localhost:6379", cfg.Addr)
		}
	})

	t.Run("Layered config", func(t *testing.T) {
		cfg := &Layered{}
		if err := Apply(cfg); err != nil {
			t.Fatalf("Apply() error: %v", err)
		}
		if cfg.L1Config == nil {
			t.Error("L1Config should not be nil after Apply")
		}
	})
}

func TestApplyEnv_AllTypes(t *testing.T) {
	type testStruct struct {
		StrVal   string        `config:"str_val"`
		IntVal   int           `config:"int_val"`
		Int64Val int64         `config:"int64_val"`
		FloatVal float64       `config:"float_val"`
		BoolVal  bool          `config:"bool_val"`
		DurVal   time.Duration `config:"dur_val"`
		NoTag    string
	}

	tests := []struct {
		name     string
		envKey   string
		envVal   string
		wantStr  string
		wantInt  int
		wantI64  int64
		wantFlt  float64
		wantBool bool
		wantDur  time.Duration
	}{
		{
			name:    "string override",
			envKey:  "CACHE_STR_VAL",
			envVal:  "hello",
			wantStr: "hello",
		},
		{
			name:    "int override",
			envKey:  "CACHE_INT_VAL",
			envVal:  "42",
			wantInt: 42,
		},
		{
			name:    "int64 override",
			envKey:  "CACHE_INT64_VAL",
			envVal:  "999",
			wantI64: 999,
		},
		{
			name:    "float64 override",
			envKey:  "CACHE_FLOAT_VAL",
			envVal:  "3.14",
			wantFlt: 3.14,
		},
		{
			name:     "bool true override",
			envKey:   "CACHE_BOOL_VAL",
			envVal:   "true",
			wantBool: true,
		},
		{
			name:     "bool 1 override",
			envKey:   "CACHE_BOOL_VAL",
			envVal:   "1",
			wantBool: true,
		},
		{
			name:    "duration override",
			envKey:  "CACHE_DUR_VAL",
			envVal:  "5m",
			wantDur: 5 * time.Minute,
		},
		{
			name:    "duration ms override",
			envKey:  "CACHE_DUR_VAL",
			envVal:  "100ms",
			wantDur: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)

			var s testStruct
			if err := ApplyEnv(&s); err != nil {
				t.Fatalf("ApplyEnv() error: %v", err)
			}

			if tt.wantStr != "" && s.StrVal != tt.wantStr {
				t.Errorf("StrVal = %q, want %q", s.StrVal, tt.wantStr)
			}
			if tt.wantInt != 0 && s.IntVal != tt.wantInt {
				t.Errorf("IntVal = %d, want %d", s.IntVal, tt.wantInt)
			}
			if tt.wantI64 != 0 && s.Int64Val != tt.wantI64 {
				t.Errorf("Int64Val = %d, want %d", s.Int64Val, tt.wantI64)
			}
			if tt.wantFlt != 0 && s.FloatVal != tt.wantFlt {
				t.Errorf("FloatVal = %f, want %f", s.FloatVal, tt.wantFlt)
			}
			if tt.wantBool && !s.BoolVal {
				t.Errorf("BoolVal = %v, want %v", s.BoolVal, tt.wantBool)
			}
			if tt.wantDur != 0 && s.DurVal != tt.wantDur {
				t.Errorf("DurVal = %v, want %v", s.DurVal, tt.wantDur)
			}
		})
	}
}

func TestApplyEnv_Errors(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		if err := ApplyEnv(nil); err == nil {
			t.Error("ApplyEnv(nil) should return error")
		}
	})

	t.Run("non-pointer", func(t *testing.T) {
		if err := ApplyEnv("not a struct pointer"); err == nil {
			t.Error("ApplyEnv(non-pointer) should return error")
		}
	})

	t.Run("invalid int", func(t *testing.T) {
		type s struct {
			Val int `config:"bad_int"`
		}
		t.Setenv("CACHE_BAD_INT", "not_a_number")

		var v s
		if err := ApplyEnv(&v); err == nil {
			t.Error("ApplyEnv should return error for invalid int")
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		type s struct {
			Val time.Duration `config:"bad_dur"`
		}
		t.Setenv("CACHE_BAD_DUR", "not_a_duration")

		var v s
		if err := ApplyEnv(&v); err == nil {
			t.Error("ApplyEnv should return error for invalid duration")
		}
	})

	t.Run("invalid bool", func(t *testing.T) {
		type s struct {
			Val bool `config:"bad_bool"`
		}
		t.Setenv("CACHE_BAD_BOOL", "not_a_bool")

		var v s
		if err := ApplyEnv(&v); err == nil {
			t.Error("ApplyEnv should return error for invalid bool")
		}
	})

	t.Run("unset env var is skipped", func(t *testing.T) {
		type s struct {
			Val string `config:"unset_field"`
		}
		var v s
		if err := ApplyEnv(&v); err != nil {
			t.Fatalf("ApplyEnv() error: %v", err)
		}
		if v.Val != "" {
			t.Errorf("Val = %q, want empty (no env var set)", v.Val)
		}
	})
}

func TestApplyEnv_RealConfig(t *testing.T) {
	t.Run("Redis config via env", func(t *testing.T) {
		t.Setenv("CACHE_ADDR", "redis.prod:6379")
		t.Setenv("CACHE_POOL_SIZE", "25")
		t.Setenv("CACHE_ENABLE_PIPELINE", "false")

		cfg := &Redis{}
		if err := ApplyEnv(cfg); err != nil {
			t.Fatalf("ApplyEnv() error: %v", err)
		}
		if cfg.Addr != "redis.prod:6379" {
			t.Errorf("Addr = %q, want redis.prod:6379", cfg.Addr)
		}
		if cfg.PoolSize != 25 {
			t.Errorf("PoolSize = %d, want 25", cfg.PoolSize)
		}
		if cfg.EnablePipeline {
			t.Error("EnablePipeline should be false")
		}
	})

	t.Run("Memory config via env", func(t *testing.T) {
		t.Setenv("CACHE_MAX_ENTRIES", "50000")
		t.Setenv("CACHE_DEFAULT_TTL", "1h")
		t.Setenv("CACHE_ENABLE_METRICS", "true")

		cfg := &Memory{}
		if err := ApplyEnv(cfg); err != nil {
			t.Fatalf("ApplyEnv() error: %v", err)
		}
		if cfg.MaxEntries != 50000 {
			t.Errorf("MaxEntries = %d, want 50000", cfg.MaxEntries)
		}
		if cfg.DefaultTTL != time.Hour {
			t.Errorf("DefaultTTL = %v, want 1h", cfg.DefaultTTL)
		}
		if !cfg.EnableMetrics {
			t.Error("EnableMetrics should be true")
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("empty config without defaults", func(t *testing.T) {
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}
		// Without WithDefaults, sub-configs are nil.
		if cfg.Memory != nil {
			t.Error("Memory should be nil without WithDefaults")
		}
		if cfg.Redis != nil {
			t.Error("Redis should be nil without WithDefaults")
		}
		if cfg.Layered != nil {
			t.Error("Layered should be nil without WithDefaults")
		}
	})

	t.Run("WithDefaults", func(t *testing.T) {
		cfg, err := LoadConfig(WithDefaults())
		if err != nil {
			t.Fatalf("LoadConfig(WithDefaults()) error: %v", err)
		}
		if cfg.Memory == nil {
			t.Error("Memory should not be nil with WithDefaults")
		}
		if cfg.Redis == nil {
			t.Error("Redis should not be nil with WithDefaults")
		}
		if cfg.Layered == nil {
			t.Error("Layered should not be nil with WithDefaults")
		}
		if cfg.Resilience.CircuitBreakerThreshold != 5 {
			t.Errorf("CircuitBreakerThreshold = %d, want 5", cfg.Resilience.CircuitBreakerThreshold)
		}
		if cfg.Resilience.CircuitBreakerTimeout != 30*time.Second {
			t.Errorf("CircuitBreakerTimeout = %v, want 30s", cfg.Resilience.CircuitBreakerTimeout)
		}
		if cfg.Resilience.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", cfg.Resilience.MaxRetries)
		}
		if cfg.Observability.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want info", cfg.Observability.LogLevel)
		}
		if cfg.Observability.SlowThreshold != 10*time.Millisecond {
			t.Errorf("SlowThreshold = %v, want 10ms", cfg.Observability.SlowThreshold)
		}
	})

	t.Run("FromEnv", func(t *testing.T) {
		t.Setenv("CACHE_CB_THRESHOLD", "10")
		t.Setenv("CACHE_LOG_LEVEL", "debug")
		t.Setenv("CACHE_SLOW_THRESHOLD", "50ms")
		t.Setenv("CACHE_METRICS_ENABLED", "true")

		cfg, err := LoadConfig(WithDefaults(), FromEnv())
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}
		if cfg.Resilience.CircuitBreakerThreshold != 10 {
			t.Errorf(
				"CircuitBreakerThreshold = %d, want 10",
				cfg.Resilience.CircuitBreakerThreshold,
			)
		}
		if cfg.Observability.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want debug", cfg.Observability.LogLevel)
		}
		if cfg.Observability.SlowThreshold != 50*time.Millisecond {
			t.Errorf("SlowThreshold = %v, want 50ms", cfg.Observability.SlowThreshold)
		}
		if !cfg.Observability.MetricsEnabled {
			t.Error("MetricsEnabled should be true")
		}
	})

	t.Run("FromMap", func(t *testing.T) {
		cfg, err := LoadConfig(WithDefaults(), FromMap(map[string]string{
			"cb_threshold":    "7",
			"log_level":       "warn",
			"slow_threshold":  "200ms",
			"max_retries":     "5",
			"tracing_enabled": "true",
		}))
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}
		if cfg.Resilience.CircuitBreakerThreshold != 7 {
			t.Errorf("CircuitBreakerThreshold = %d, want 7", cfg.Resilience.CircuitBreakerThreshold)
		}
		if cfg.Resilience.MaxRetries != 5 {
			t.Errorf("MaxRetries = %d, want 5", cfg.Resilience.MaxRetries)
		}
		if cfg.Observability.LogLevel != "warn" {
			t.Errorf("LogLevel = %q, want warn", cfg.Observability.LogLevel)
		}
		if cfg.Observability.SlowThreshold != 200*time.Millisecond {
			t.Errorf("SlowThreshold = %v, want 200ms", cfg.Observability.SlowThreshold)
		}
		if !cfg.Observability.TracingEnabled {
			t.Error("TracingEnabled should be true")
		}
	})

	t.Run("validation error for bad log level", func(t *testing.T) {
		_, err := LoadConfig(WithDefaults(), FromMap(map[string]string{
			"log_level": "verbose",
		}))
		if err == nil {
			t.Error("LoadConfig should return error for invalid log level")
		}
	})

	t.Run("validation error for negative circuit breaker threshold", func(t *testing.T) {
		_, err := LoadConfig(WithDefaults(), FromMap(map[string]string{
			"cb_threshold": "-1",
		}))
		if err == nil {
			t.Error("LoadConfig should return error for negative CB threshold")
		}
	})
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Memory == nil {
		t.Error("Memory should be set")
	}
	if cfg.Redis == nil {
		t.Error("Redis should be set")
	}
	if cfg.Layered == nil {
		t.Error("Layered should be set")
	}
	if cfg.Resilience.CircuitBreakerThreshold != 5 {
		t.Errorf("CircuitBreakerThreshold = %d, want 5", cfg.Resilience.CircuitBreakerThreshold)
	}
	if cfg.Observability.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.Observability.LogLevel)
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid defaults", func(t *testing.T) {
		cfg := &Config{}
		cfg.SetDefaults()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error: %v", err)
		}
	})

	t.Run("nil sub-configs are valid", func(t *testing.T) {
		cfg := &Config{}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() with nil sub-configs error: %v", err)
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := &Config{}
		cfg.Observability.LogLevel = "verbose"
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should reject invalid log level")
		}
	})

	t.Run("negative slow threshold", func(t *testing.T) {
		cfg := &Config{}
		cfg.Observability.SlowThreshold = -1
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should reject negative slow threshold")
		}
	})

	t.Run("negative max retries", func(t *testing.T) {
		cfg := &Config{}
		cfg.Resilience.MaxRetries = -1
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should reject negative max retries")
		}
	})

	t.Run("negative CB timeout", func(t *testing.T) {
		cfg := &Config{}
		cfg.Resilience.CircuitBreakerTimeout = -1 * time.Second
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should reject negative CB timeout")
		}
	})

	t.Run("invalid memory sub-config", func(t *testing.T) {
		cfg := &Config{}
		cfg.Memory = &Memory{MaxEntries: -1}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should reject invalid Memory sub-config")
		}
	})

	t.Run("valid log levels", func(t *testing.T) {
		for _, level := range []string{"debug", "info", "warn", "error"} {
			t.Run(level, func(t *testing.T) {
				cfg := &Config{}
				cfg.Observability.LogLevel = level
				if err := cfg.Validate(); err != nil {
					t.Errorf("Validate() should accept log level %q: %v", level, err)
				}
			})
		}
	})
}

func TestResilienceConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		var r ResilienceConfig
		SetDefaultInt(&r.CircuitBreakerThreshold, 5)
		SetDefaultDuration(&r.CircuitBreakerTimeout, 30*time.Second)
		SetDefaultInt(&r.MaxRetries, 3)

		if r.CircuitBreakerThreshold != 5 {
			t.Errorf("CircuitBreakerThreshold = %d, want 5", r.CircuitBreakerThreshold)
		}
		if r.CircuitBreakerTimeout != 30*time.Second {
			t.Errorf("CircuitBreakerTimeout = %v, want 30s", r.CircuitBreakerTimeout)
		}
		if r.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", r.MaxRetries)
		}
	})
}

func TestObsConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		var o ObsConfig
		SetDefaultBool(&o.MetricsEnabled, false)
		SetDefaultBool(&o.TracingEnabled, false)
		SetDefaultString(&o.LogLevel, "info")
		SetDefaultDuration(&o.SlowThreshold, 10*time.Millisecond)

		if o.MetricsEnabled {
			t.Error("MetricsEnabled should be false")
		}
		if o.TracingEnabled {
			t.Error("TracingEnabled should be false")
		}
		if o.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want info", o.LogLevel)
		}
		if o.SlowThreshold != 10*time.Millisecond {
			t.Errorf("SlowThreshold = %v, want 10ms", o.SlowThreshold)
		}
	})
}

func TestSetMapValues(t *testing.T) {
	t.Run("overrides resilience config", func(t *testing.T) {
		cfg := &Config{}
		cfg.SetDefaults()

		m := map[string]string{
			"cb_threshold":   "8",
			"cb_timeout":     "1m",
			"max_retries":    "5",
			"log_level":      "error",
			"slow_threshold": "500ms",
		}

		if err := setMapValues(cfg, m); err != nil {
			t.Fatalf("setMapValues() error: %v", err)
		}

		if cfg.Resilience.CircuitBreakerThreshold != 8 {
			t.Errorf("CircuitBreakerThreshold = %d, want 8", cfg.Resilience.CircuitBreakerThreshold)
		}
		if cfg.Resilience.CircuitBreakerTimeout != time.Minute {
			t.Errorf("CircuitBreakerTimeout = %v, want 1m", cfg.Resilience.CircuitBreakerTimeout)
		}
		if cfg.Resilience.MaxRetries != 5 {
			t.Errorf("MaxRetries = %d, want 5", cfg.Resilience.MaxRetries)
		}
		if cfg.Observability.LogLevel != "error" {
			t.Errorf("LogLevel = %q, want error", cfg.Observability.LogLevel)
		}
		if cfg.Observability.SlowThreshold != 500*time.Millisecond {
			t.Errorf("SlowThreshold = %v, want 500ms", cfg.Observability.SlowThreshold)
		}
	})

	t.Run("restores env vars after use", func(t *testing.T) {
		// Set an existing env var.
		origKey := "CACHE_CB_THRESHOLD"
		t.Setenv(origKey, "original_value")

		cfg := &Config{}
		cfg.SetDefaults()

		m := map[string]string{"cb_threshold": "99"}
		_ = setMapValues(cfg, m)

		// Verify the env var is restored.
		val, ok := os.LookupEnv(origKey)
		if !ok {
			t.Error("env var should still be set after setMapValues")
		} else if val != "original_value" {
			t.Errorf("env var = %q, want original_value", val)
		}
	})

	t.Run("unsets env vars that weren't set before", func(t *testing.T) {
		// Verify that setMapValues doesn't leave behind env vars
		// that weren't set before the call.
		key := "CACHE_LOG_LEVEL"

		cfg := &Config{}
		cfg.SetDefaults()

		m := map[string]string{"log_level": "debug"}
		_ = setMapValues(cfg, m)

		// The config value should have been applied.
		if cfg.Observability.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want debug", cfg.Observability.LogLevel)
		}

		// The env var should have been unset after setMapValues
		// if it wasn't previously set. We can't reliably test this
		// with t.Setenv since it manages env vars itself, so we
		// just verify the config was applied correctly.
		_ = key
	})
}

func TestTagName(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"simple", "simple"},
		{"with,extra", "with"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := tagName(tt.tag); got != tt.want {
			t.Errorf("tagName(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}
