package layered

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/os-gomod/cache/config"
)

func TestMergeOptions(t *testing.T) {
	opts := []Option{
		WithL1MaxEntries(1000),
		WithL1MaxMemoryMB(50),
		WithL1TTL(5 * time.Minute),
		WithL1CleanupInterval(time.Minute),
		WithL1Shards(64),
		WithL2Address("localhost:6379"),
		WithL2DB(5),
		WithL2PoolSize(20),
		WithL2TTL(time.Hour),
		WithL2KeyPrefix("test:"),
		WithPromoteOnHit(true),
		WithWriteBack(false),
		WithNegativeTTL(30 * time.Second),
	}

	cfg, err := MergeOptions(opts...)
	assert.NoError(t, err)

	assert.Equal(t, 1000, cfg.L1Config.MaxEntries)
	assert.Equal(t, 50, cfg.L1Config.MaxMemoryMB)
	assert.Equal(t, 5*time.Minute, cfg.L1Config.DefaultTTL)
	assert.Equal(t, time.Minute, cfg.L1Config.CleanupInterval)
	assert.Equal(t, config.EvictLRU, cfg.L1Config.EvictionPolicy)
	assert.Equal(t, 64, cfg.L1Config.ShardCount)
	assert.Equal(t, "localhost:6379", cfg.L2Config.Addr)
	assert.Equal(t, 5, cfg.L2Config.DB)
	assert.Equal(t, 20, cfg.L2Config.PoolSize)
	assert.Equal(t, time.Hour, cfg.L2Config.DefaultTTL)
	assert.Equal(t, "test:", cfg.L2Config.KeyPrefix)
	assert.True(t, cfg.PromoteOnHit)
	assert.False(t, cfg.WriteBack)
	assert.Equal(t, 30*time.Second, cfg.NegativeTTL)
}

// ----------------------------------------------------------------------------
// L1 Configuration Options Tests
// ----------------------------------------------------------------------------

func TestWithL1Config(t *testing.T) {
	cfg := &config.Layered{}
	l1Cfg := &config.Memory{
		MaxEntries: 5000,
		DefaultTTL: 10 * time.Minute,
	}
	opt := WithL1Config(l1Cfg)
	opt(cfg)
	assert.Equal(t, l1Cfg, cfg.L1Config)
}

func TestWithL1MaxEntries(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1MaxEntries(2000)
	opt(cfg)
	assert.Equal(t, 2000, cfg.L1Config.MaxEntries)
}

func TestWithL1MaxMemoryMB(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1MaxMemoryMB(100)
	opt(cfg)
	assert.Equal(t, 100, cfg.L1Config.MaxMemoryMB)
}

func TestWithL1MaxMemoryBytes(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1MaxMemoryBytes(1024 * 1024 * 200) // 200 MB
	opt(cfg)
	assert.Equal(t, 200, cfg.L1Config.MaxMemoryMB)
}

func TestWithL1TTL(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1TTL(15 * time.Minute)
	opt(cfg)
	assert.Equal(t, 15*time.Minute, cfg.L1Config.DefaultTTL)
}

func TestWithL1CleanupInterval(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1CleanupInterval(2 * time.Minute)
	opt(cfg)
	assert.Equal(t, 2*time.Minute, cfg.L1Config.CleanupInterval)
}

func TestWithL1Shards(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1Shards(128)
	opt(cfg)
	assert.Equal(t, 128, cfg.L1Config.ShardCount)
}

func TestWithL1EvictionPolicy(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1EvictionPolicy(config.EvictLFU)
	opt(cfg)
	assert.Equal(t, config.EvictLFU, cfg.L1Config.EvictionPolicy)
}

func TestWithL1OnEvictionPolicy(t *testing.T) {
	cfg := &config.Layered{}
	called := 0
	fn := func(key, reason string) {
		called++
		_ = key    // Explicitly mark as used
		_ = reason // Explicitly mark as used
	}
	opt := WithL1OnEvictionPolicy(fn)
	opt(cfg)
	assert.NotNil(t, cfg.L1Config.OnEvictionPolicy)
	// Verify the callback hasn't been called yet
	assert.Equal(t, 0, called)
}

func TestWithL1EnableMetrics(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL1EnableMetrics(true)
	opt(cfg)
	assert.True(t, cfg.L1Config.EnableMetrics)
}

// ----------------------------------------------------------------------------
// L2 Configuration Options Tests
// ----------------------------------------------------------------------------

func TestWithL2Config(t *testing.T) {
	cfg := &config.Layered{}
	l2Cfg := &config.Redis{
		Addr:       "redis:6379",
		PoolSize:   50,
		DefaultTTL: time.Hour,
	}
	opt := WithL2Config(l2Cfg)
	opt(cfg)
	assert.Equal(t, l2Cfg, cfg.L2Config)
}

func TestWithL2Addr(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2Address("redis-cluster:6379")
	opt(cfg)
	assert.Equal(t, "redis-cluster:6379", cfg.L2Config.Addr)
}

func TestWithL2Username(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2Username("testuser")
	opt(cfg)
	assert.Equal(t, "testuser", cfg.L2Config.Username)
}

func TestWithL2Password(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2Password("secret")
	opt(cfg)
	assert.Equal(t, "secret", cfg.L2Config.Password)
}

func TestWithL2DB(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2DB(7)
	opt(cfg)
	assert.Equal(t, 7, cfg.L2Config.DB)
}

func TestWithL2PoolSize(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2PoolSize(30)
	opt(cfg)
	assert.Equal(t, 30, cfg.L2Config.PoolSize)
}

func TestWithL2MinIdleConns(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2MinIdleConns(10)
	opt(cfg)
	assert.Equal(t, 10, cfg.L2Config.MinIdleConns)
}

func TestWithL2MaxRetries(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2MaxRetries(5)
	opt(cfg)
	assert.Equal(t, 5, cfg.L2Config.MaxRetries)
}

func TestWithL2RetryBackoff(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2RetryBackoff(200 * time.Millisecond)
	opt(cfg)
	assert.Equal(t, 200*time.Millisecond, cfg.L2Config.RetryBackoff)
}

func TestWithL2DialTimeout(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2DialTimeout(10 * time.Second)
	opt(cfg)
	assert.Equal(t, 10*time.Second, cfg.L2Config.DialTimeout)
}

func TestWithL2ReadTimeout(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2ReadTimeout(5 * time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.L2Config.ReadTimeout)
}

func TestWithL2WriteTimeout(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2WriteTimeout(5 * time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.L2Config.WriteTimeout)
}

func TestWithL2Timeouts(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2Timeouts(5*time.Second, 3*time.Second, 2*time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.L2Config.DialTimeout)
	assert.Equal(t, 3*time.Second, cfg.L2Config.ReadTimeout)
	assert.Equal(t, 2*time.Second, cfg.L2Config.WriteTimeout)
}

func TestWithL2TTL(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2TTL(2 * time.Hour)
	opt(cfg)
	assert.Equal(t, 2*time.Hour, cfg.L2Config.DefaultTTL)
}

func TestWithL2KeyPrefix(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2KeyPrefix("app:")
	opt(cfg)
	assert.Equal(t, "app:", cfg.L2Config.KeyPrefix)
}

func TestWithL2EnablePipeline(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2EnablePipeline(false)
	opt(cfg)
	assert.False(t, cfg.L2Config.EnablePipeline)
}

func TestWithL2EnableMetrics(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithL2EnableMetrics(true)
	opt(cfg)
	assert.True(t, cfg.L2Config.EnableMetrics)
}

// ----------------------------------------------------------------------------
// Layered Behavior Options Tests
// ----------------------------------------------------------------------------

func TestWithPromoteOnHit(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithPromoteOnHit(false)
	opt(cfg)
	assert.False(t, cfg.PromoteOnHit)
}

func TestWithWriteBack(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithWriteBack(true)
	opt(cfg)
	assert.True(t, cfg.WriteBack)
}

func TestWithNegativeTTL(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithNegativeTTL(1 * time.Minute)
	opt(cfg)
	assert.Equal(t, 1*time.Minute, cfg.NegativeTTL)
}

func TestWithSyncEnabled(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithSyncEnabled(true)
	opt(cfg)
	assert.True(t, cfg.SyncEnabled)
}

func TestWithSyncChannel(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithSyncChannel("custom:channel")
	opt(cfg)
	assert.Equal(t, "custom:channel", cfg.SyncChannel)
}

func TestWithSyncBufferSize(t *testing.T) {
	cfg := &config.Layered{}
	opt := WithSyncBufferSize(5000)
	opt(cfg)
	assert.Equal(t, 5000, cfg.SyncBufferSize)
}

// ----------------------------------------------------------------------------
// Combined Options Tests
// ----------------------------------------------------------------------------

func TestWithConfig(t *testing.T) {
	original := &config.Layered{
		L1Config: &config.Memory{
			MaxEntries: 1000,
			DefaultTTL: time.Minute,
		},
		L2Config: &config.Redis{
			Addr:       "localhost:6379",
			DefaultTTL: time.Hour,
		},
		PromoteOnHit: true,
		WriteBack:    false,
		NegativeTTL:  30 * time.Second,
	}

	cfg := &config.Layered{}
	opt := WithConfig(original)
	opt(cfg)

	assert.Equal(t, original.L1Config.MaxEntries, cfg.L1Config.MaxEntries)
	assert.Equal(t, original.L1Config.DefaultTTL, cfg.L1Config.DefaultTTL)
	assert.Equal(t, original.L2Config.Addr, cfg.L2Config.Addr)
	assert.Equal(t, original.L2Config.DefaultTTL, cfg.L2Config.DefaultTTL)
	assert.Equal(t, original.PromoteOnHit, cfg.PromoteOnHit)
	assert.Equal(t, original.WriteBack, cfg.WriteBack)
	assert.Equal(t, original.NegativeTTL, cfg.NegativeTTL)
}

func TestMergeOptions_Defaults(t *testing.T) {
	cfg, err := MergeOptions()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.L1Config)
	assert.NotNil(t, cfg.L2Config)
	assert.Equal(t, 10000, cfg.L1Config.MaxEntries)
	assert.Equal(t, 100, cfg.L1Config.MaxMemoryMB)
	assert.Equal(t, 30*time.Minute, cfg.L1Config.DefaultTTL)
	assert.Equal(t, 5*time.Minute, cfg.L1Config.CleanupInterval)
	assert.Equal(t, 32, cfg.L1Config.ShardCount)
	assert.Equal(t, "localhost:6379", cfg.L2Config.Addr)
	assert.Equal(t, time.Hour, cfg.L2Config.DefaultTTL)
}
