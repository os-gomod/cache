package memory

import (
	"testing"
	"time"

	"github.com/os-gomod/cache/config"
)

// ---------------------------------------------------------------------------
// WithMaxMemoryMB
// ---------------------------------------------------------------------------

func TestWithMaxMemoryMB(t *testing.T) {
	const mb = 50
	cfg := &config.Memory{}
	opt := WithMaxMemoryMB(mb)
	opt(cfg)

	if cfg.MaxMemoryMB != mb {
		t.Errorf("MaxMemoryMB = %d, want %d", cfg.MaxMemoryMB, mb)
	}
	const bytesPerMB = int64(1024 * 1024)
	expectedBytes := int64(mb) * bytesPerMB
	if cfg.MaxMemoryBytes != expectedBytes {
		t.Errorf("MaxMemoryBytes = %d, want %d", cfg.MaxMemoryBytes, expectedBytes)
	}
}

// ---------------------------------------------------------------------------
// WithMaxMemoryBytes
// ---------------------------------------------------------------------------

func TestWithMaxMemoryBytes(t *testing.T) {
	bytes := int64(64 * 1024 * 1024) // 64 MB
	cfg := &config.Memory{}
	opt := WithMaxMemoryBytes(bytes)
	opt(cfg)

	if cfg.MaxMemoryBytes != bytes {
		t.Errorf("MaxMemoryBytes = %d, want %d", cfg.MaxMemoryBytes, bytes)
	}
	if cfg.MaxMemoryMB != 64 {
		t.Errorf("MaxMemoryMB = %d, want 64", cfg.MaxMemoryMB)
	}
}

// ---------------------------------------------------------------------------
// WithConfig: nil config should not panic
// ---------------------------------------------------------------------------

func TestWithConfig_Nil(t *testing.T) {
	// This should not panic.
	opt := WithConfig(nil)
	if opt == nil {
		t.Fatal("WithConfig(nil) returned nil option func")
	}

	cfg := &config.Memory{MaxEntries: 42}
	opt(cfg)
	// The nil option is a no-op, so MaxEntries should be preserved.
	if cfg.MaxEntries != 42 {
		t.Errorf("MaxEntries = %d, want 42 after nil WithConfig", cfg.MaxEntries)
	}
}

// ---------------------------------------------------------------------------
// WithConfig: clones the provided config
// ---------------------------------------------------------------------------

func TestWithConfig_Clones(t *testing.T) {
	original := &config.Memory{
		MaxEntries:      777,
		MaxMemoryMB:     10,
		MaxMemoryBytes:  10 * 1024 * 1024,
		DefaultTTL:      2 * time.Hour,
		CleanupInterval: 30 * time.Second,
		ShardCount:      8,
		EvictionPolicy:  config.EvictLFU,
	}

	cfg := &config.Memory{MaxEntries: 0}
	opt := WithConfig(original)
	opt(cfg)

	if cfg.MaxEntries != 777 {
		t.Errorf("MaxEntries = %d, want 777", cfg.MaxEntries)
	}
	if cfg.DefaultTTL != 2*time.Hour {
		t.Errorf("DefaultTTL = %v, want %v", cfg.DefaultTTL, 2*time.Hour)
	}
	if cfg.EvictionPolicy != config.EvictLFU {
		t.Errorf("EvictionPolicy = %d, want %d", cfg.EvictionPolicy, config.EvictLFU)
	}

	// Mutate the original to prove it was cloned.
	original.MaxEntries = 9999
	if cfg.MaxEntries != 777 {
		t.Errorf("MaxEntries after original mutation = %d, want 777 (cloned)", cfg.MaxEntries)
	}
}

// ---------------------------------------------------------------------------
// MergeOptions: applies defaults
// ---------------------------------------------------------------------------

func TestMergeOptions_AppliesDefaults(t *testing.T) {
	cfg, err := MergeOptions(WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("MergeOptions() error = %v", err)
	}

	if cfg.MaxEntries <= 0 {
		t.Errorf("MaxEntries = %d, want > 0 (default applied)", cfg.MaxEntries)
	}
	if cfg.DefaultTTL <= 0 {
		t.Errorf("DefaultTTL = %v, want > 0 (default applied)", cfg.DefaultTTL)
	}
	if cfg.ShardCount <= 0 {
		t.Errorf("ShardCount = %d, want > 0 (default applied)", cfg.ShardCount)
	}
	if !cfg.EvictionPolicy.IsValid() {
		t.Errorf("EvictionPolicy = %d, want valid", cfg.EvictionPolicy)
	}
}

// ---------------------------------------------------------------------------
// MergeOptions: user options override defaults
// ---------------------------------------------------------------------------

func TestMergeOptions_OverridesDefaults(t *testing.T) {
	cfg, err := MergeOptions(
		WithMaxEntries(42),
		WithTTL(5*time.Second),
		WithCleanupInterval(0),
		WithShards(4),
		WithEvictionPolicy(config.EvictFIFO),
	)
	if err != nil {
		t.Fatalf("MergeOptions() error = %v", err)
	}

	if cfg.MaxEntries != 42 {
		t.Errorf("MaxEntries = %d, want 42", cfg.MaxEntries)
	}
	if cfg.DefaultTTL != 5*time.Second {
		t.Errorf("DefaultTTL = %v, want %v", cfg.DefaultTTL, 5*time.Second)
	}
	if cfg.ShardCount != 4 {
		t.Errorf("ShardCount = %d, want 4", cfg.ShardCount)
	}
	if cfg.EvictionPolicy != config.EvictFIFO {
		t.Errorf("EvictionPolicy = %d, want %d", cfg.EvictionPolicy, config.EvictFIFO)
	}
}

// ---------------------------------------------------------------------------
// WithEnableMetrics option
// ---------------------------------------------------------------------------

func TestWithEnableMetrics(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithEnableMetrics(true)
	opt(cfg)

	if cfg.EnableMetrics != true {
		t.Errorf("EnableMetrics = %v, want true", cfg.EnableMetrics)
	}
}

// ---------------------------------------------------------------------------
// WithOnEvictionPolicy option
// ---------------------------------------------------------------------------

func TestWithOnEvictionPolicy(t *testing.T) {
	cfg := &config.Memory{}
	var called bool
	opt := WithOnEvictionPolicy(func(key, reason string) {
		called = true
	})
	opt(cfg)

	if cfg.OnEvictionPolicy == nil {
		t.Fatal("OnEvictionPolicy should be set")
	}
	cfg.OnEvictionPolicy("k", "test")
	if !called {
		t.Error("callback should have been invoked")
	}
}

// ---------------------------------------------------------------------------
// WithInterceptors option
// ---------------------------------------------------------------------------

func TestWithInterceptors(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithInterceptors()
	opt(cfg)

	if cfg.Interceptors == nil {
		t.Fatal("Interceptors should be set (even if empty slice)")
	}
}

// ---------------------------------------------------------------------------
// WithMaxMemoryBytes zero / negative
// ---------------------------------------------------------------------------

func TestWithMaxMemoryBytes_ZeroAndNegative(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		cfg := &config.Memory{}
		WithMaxMemoryBytes(0)(cfg)
		if cfg.MaxMemoryBytes != 0 {
			t.Errorf("MaxMemoryBytes = %d, want 0", cfg.MaxMemoryBytes)
		}
		if cfg.MaxMemoryMB != 0 {
			t.Errorf("MaxMemoryMB = %d, want 0", cfg.MaxMemoryMB)
		}
	})
	t.Run("negative", func(t *testing.T) {
		cfg := &config.Memory{}
		WithMaxMemoryBytes(-100)(cfg)
		if cfg.MaxMemoryBytes != -100 {
			t.Errorf("MaxMemoryBytes = %d, want -100", cfg.MaxMemoryBytes)
		}
		if cfg.MaxMemoryMB != 0 {
			t.Errorf("MaxMemoryMB = %d, want 0 for negative bytes", cfg.MaxMemoryMB)
		}
	})
}
