package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/os-gomod/cache/config"
)

func TestMergeOptions(t *testing.T) {
	opts := []Option{
		WithMaxEntries(5000),
		WithMaxMemoryMB(200),
		WithTTL(30 * time.Minute),
		WithCleanupInterval(5 * time.Minute),
		WithLRU(),
		WithShards(64),
	}

	cfg, err := MergeOptions(opts...)
	assert.NoError(t, err)

	assert.Equal(t, 5000, cfg.MaxEntries)
	assert.Equal(t, 200, cfg.MaxMemoryMB)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTTL)
	assert.Equal(t, 5*time.Minute, cfg.CleanupInterval)
	assert.Equal(t, config.EvictLRU, cfg.EvictionPolicy)
	assert.Equal(t, 64, cfg.ShardCount)
}

func TestWithMaxEntries(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithMaxEntries(10000)
	opt(cfg)
	assert.Equal(t, 10000, cfg.MaxEntries)
}

func TestWithMaxMemoryMB(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithMaxMemoryMB(512)
	opt(cfg)
	assert.Equal(t, 512, cfg.MaxMemoryMB)
}

func TestWithMaxMemoryBytes(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithMaxMemoryBytes(1024 * 1024 * 100) // 100 MB
	opt(cfg)
	assert.Equal(t, 100, cfg.MaxMemoryMB)
}

func TestWithTTL(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithTTL(10 * time.Minute)
	opt(cfg)
	assert.Equal(t, 10*time.Minute, cfg.DefaultTTL)
}

func TestWithCleanupInterval(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithCleanupInterval(2 * time.Minute)
	opt(cfg)
	assert.Equal(t, 2*time.Minute, cfg.CleanupInterval)
}

func TestWithEvictionPolicy(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithEvictionPolicy(config.EvictLFU)
	opt(cfg)
	assert.Equal(t, config.EvictLFU, cfg.EvictionPolicy)
}

func TestWithLRU(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithLRU()
	opt(cfg)
	assert.Equal(t, config.EvictLRU, cfg.EvictionPolicy)
}

func TestWithLFU(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithLFU()
	opt(cfg)
	assert.Equal(t, config.EvictLFU, cfg.EvictionPolicy)
}

func TestWithFIFO(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithFIFO()
	opt(cfg)
	assert.Equal(t, config.EvictFIFO, cfg.EvictionPolicy)
}

func TestWithLIFO(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithLIFO()
	opt(cfg)
	assert.Equal(t, config.EvictLIFO, cfg.EvictionPolicy)
}

func TestWithMRU(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithMRU()
	opt(cfg)
	assert.Equal(t, config.EvictMRU, cfg.EvictionPolicy)
}

func TestWithRandom(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithRandom()
	opt(cfg)
	assert.Equal(t, config.EvictRR, cfg.EvictionPolicy)
}

func TestWithTinyLFU(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithTinyLFU()
	opt(cfg)
	assert.Equal(t, config.EvictTinyLFU, cfg.EvictionPolicy)
}

func TestWithShards(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithShards(128)
	opt(cfg)
	assert.Equal(t, 128, cfg.ShardCount)
}

func TestWithEnableMetrics(t *testing.T) {
	cfg := &config.Memory{}
	opt := WithEnableMetrics(true)
	opt(cfg)
	assert.True(t, cfg.EnableMetrics)
}

func TestWithOnEvictionPolicy(t *testing.T) {
	cfg := &config.Memory{}
	called := 0
	fn := func(key, reason string) {
		called++
		_ = key    // Explicitly mark as used to avoid lint warning
		_ = reason // Explicitly mark as used to avoid lint warning
	}
	opt := WithOnEvictionPolicy(fn)
	opt(cfg)
	assert.NotNil(t, cfg.OnEvictionPolicy)
}

func TestWithConfig(t *testing.T) {
	original := &config.Memory{
		MaxEntries:      1000,
		MaxMemoryMB:     50,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: time.Minute,
		EvictionPolicy:  config.EvictLRU,
		ShardCount:      64,
		EnableMetrics:   true,
	}

	cfg := &config.Memory{}
	opt := WithConfig(original)
	opt(cfg)

	assert.Equal(t, original.MaxEntries, cfg.MaxEntries)
	assert.Equal(t, original.MaxMemoryMB, cfg.MaxMemoryMB)
	assert.Equal(t, original.DefaultTTL, cfg.DefaultTTL)
	assert.Equal(t, original.CleanupInterval, cfg.CleanupInterval)
	assert.Equal(t, original.EvictionPolicy, cfg.EvictionPolicy)
	assert.Equal(t, original.ShardCount, cfg.ShardCount)
	assert.Equal(t, original.EnableMetrics, cfg.EnableMetrics)
}

func TestMergeOptions_Defaults(t *testing.T) {
	cfg, err := MergeOptions()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 10000, cfg.MaxEntries)
	assert.Equal(t, 100, cfg.MaxMemoryMB)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTTL)
	assert.Equal(t, 5*time.Minute, cfg.CleanupInterval)
	assert.Equal(t, 32, cfg.ShardCount)
	assert.Equal(t, config.EvictLRU, cfg.EvictionPolicy)
}
