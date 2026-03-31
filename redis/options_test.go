package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/os-gomod/cache/config"
)

func TestWithAddress(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithAddress("redis.example.com:6379")
	opt(cfg)
	assert.Equal(t, "redis.example.com:6379", cfg.Addr)
}

func TestWithUsername(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithUsername("testuser")
	opt(cfg)
	assert.Equal(t, "testuser", cfg.Username)
}

func TestWithPassword(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithPassword("secret")
	opt(cfg)
	assert.Equal(t, "secret", cfg.Password)
}

func TestWithDB(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithDB(7)
	opt(cfg)
	assert.Equal(t, 7, cfg.DB)
}

func TestWithPoolSize(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithPoolSize(50)
	opt(cfg)
	assert.Equal(t, 50, cfg.PoolSize)
}

func TestWithMinIdleConns(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithMinIdleConns(10)
	opt(cfg)
	assert.Equal(t, 10, cfg.MinIdleConns)
}

func TestWithMaxRetries(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithMaxRetries(5)
	opt(cfg)
	assert.Equal(t, 5, cfg.MaxRetries)
}

func TestWithRetryBackoff(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithRetryBackoff(200 * time.Millisecond)
	opt(cfg)
	assert.Equal(t, 200*time.Millisecond, cfg.RetryBackoff)
}

func TestWithTTL(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithTTL(30 * time.Minute)
	opt(cfg)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTTL)
}

func TestWithKeyPrefix(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithKeyPrefix("app:")
	opt(cfg)
	assert.Equal(t, "app:", cfg.KeyPrefix)
}

func TestWithEnableMetrics(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithEnableMetrics(true)
	opt(cfg)
	assert.True(t, cfg.EnableMetrics)
}

func TestWithEnablePipeline(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithEnablePipeline(false)
	opt(cfg)
	assert.False(t, cfg.EnablePipeline)
}

func TestWithDistributedStampedeProtection(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithDistributedStampedeProtection(true)
	opt(cfg)
	assert.True(t, cfg.EnableDistributedStampedeProtection)
}

func TestWithStampedeLockTTL(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithStampedeLockTTL(7 * time.Second)
	opt(cfg)
	assert.Equal(t, 7*time.Second, cfg.StampedeLockTTL)
}

func TestWithStampedeWaitTimeout(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithStampedeWaitTimeout(1500 * time.Millisecond)
	opt(cfg)
	assert.Equal(t, 1500*time.Millisecond, cfg.StampedeWaitTimeout)
}

func TestWithStampedeRetryInterval(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithStampedeRetryInterval(40 * time.Millisecond)
	opt(cfg)
	assert.Equal(t, 40*time.Millisecond, cfg.StampedeRetryInterval)
}

func TestWithDialTimeout(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithDialTimeout(10 * time.Second)
	opt(cfg)
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
}

func TestWithReadTimeout(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithReadTimeout(5 * time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.ReadTimeout)
}

func TestWithWriteTimeout(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithWriteTimeout(5 * time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.WriteTimeout)
}

func TestWithTimeouts(t *testing.T) {
	cfg := &config.Redis{}
	opt := WithTimeouts(5*time.Second, 3*time.Second, 2*time.Second)
	opt(cfg)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 2*time.Second, cfg.WriteTimeout)
}

func TestWithConfig(t *testing.T) {
	original := &config.Redis{
		Addr:           "localhost:6379",
		DB:             3,
		PoolSize:       15,
		DefaultTTL:     time.Hour,
		KeyPrefix:      "test:",
		EnablePipeline: true,
	}

	cfg := &config.Redis{}
	opt := WithConfig(original)
	opt(cfg)

	assert.Equal(t, original.Addr, cfg.Addr)
	assert.Equal(t, original.DB, cfg.DB)
	assert.Equal(t, original.PoolSize, cfg.PoolSize)
	assert.Equal(t, original.DefaultTTL, cfg.DefaultTTL)
	assert.Equal(t, original.KeyPrefix, cfg.KeyPrefix)
	assert.Equal(t, original.EnablePipeline, cfg.EnablePipeline)
}

func TestMergeOptions(t *testing.T) {
	opts := []Option{
		WithAddress("localhost:6379"),
		WithDB(5),
		WithPoolSize(20),
		WithTTL(2 * time.Hour),
		WithKeyPrefix("test:"),
	}

	cfg, err := MergeOptions(opts...)
	assert.NoError(t, err)
	assert.Equal(t, "localhost:6379", cfg.Addr)
	assert.Equal(t, 5, cfg.DB)
	assert.Equal(t, 20, cfg.PoolSize)
	assert.Equal(t, 2*time.Hour, cfg.DefaultTTL)
	assert.Equal(t, "test:", cfg.KeyPrefix)
}

func TestMergeOptions_Defaults(t *testing.T) {
	cfg, err := MergeOptions()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:6379", cfg.Addr)
	assert.Greater(t, cfg.PoolSize, 0)
}

func TestOption_Chain(t *testing.T) {
	cfg, err := MergeOptions(
		WithAddress("redis:6379"),
		WithDB(2),
		WithPoolSize(30),
		WithTTL(15*time.Minute),
	)

	assert.NoError(t, err)
	assert.Equal(t, "redis:6379", cfg.Addr)
	assert.Equal(t, 2, cfg.DB)
	assert.Equal(t, 30, cfg.PoolSize)
	assert.Equal(t, 15*time.Minute, cfg.DefaultTTL)
}
