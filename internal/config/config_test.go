package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Memory config tests
// ---------------------------------------------------------------------------

func TestMemory_SetDefaults(t *testing.T) {
	c := &Memory{}
	c.SetDefaults()

	assert.Equal(t, 10000, c.MaxEntries)
	assert.Equal(t, int64(104857600), c.MaxMemoryBytes)
	assert.Equal(t, 32, c.ShardCount)
	assert.Equal(t, 5*time.Minute, c.DefaultTTL)
	assert.Equal(t, 1*time.Minute, c.CleanupInterval)
	assert.Equal(t, "lru", c.EvictionPolicy)
}

func TestMemory_Validate_ValidDefaults(t *testing.T) {
	c := DefaultMemory()
	err := c.Validate()
	assert.NoError(t, err)
}

func TestMemory_Validate_InvalidShardCount(t *testing.T) {
	c := DefaultMemory()
	c.ShardCount = 0
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ShardCount")
}

func TestMemory_Validate_NegativeMaxEntries(t *testing.T) {
	c := DefaultMemory()
	c.MaxEntries = -1
	err := c.Validate()
	assert.Error(t, err)
}

func TestMemory_Validate_NegativeMemory(t *testing.T) {
	c := DefaultMemory()
	c.MaxMemoryBytes = -1
	err := c.Validate()
	assert.Error(t, err)
}

func TestMemory_Validate_InvalidEvictionPolicy(t *testing.T) {
	c := DefaultMemory()
	c.EvictionPolicy = "unknown"
	err := c.Validate()
	assert.Error(t, err)
}

func TestMemory_Validate_ValidPolicies(t *testing.T) {
	policies := []string{"lru", "lfu", "fifo", "lifo", "mru", "random", "tinylfu"}
	for _, p := range policies {
		c := DefaultMemory()
		c.EvictionPolicy = p
		err := c.Validate()
		assert.NoError(t, err, "policy=%s should be valid", p)
	}
}

func TestMemory_Clone(t *testing.T) {
	c := DefaultMemory()
	clone := c.Clone()

	// Values should match.
	assert.Equal(t, c.MaxEntries, clone.MaxEntries)
	assert.Equal(t, c.MaxMemoryBytes, clone.MaxMemoryBytes)
	assert.Equal(t, c.ShardCount, clone.ShardCount)
	assert.Equal(t, c.DefaultTTL, clone.DefaultTTL)
	assert.Equal(t, c.CleanupInterval, clone.CleanupInterval)
	assert.Equal(t, c.EvictionPolicy, clone.EvictionPolicy)

	// Modify clone — original should be unaffected.
	clone.MaxEntries = 999
	clone.EvictionPolicy = "lfu"
	assert.Equal(t, 10000, c.MaxEntries)
	assert.Equal(t, "lru", c.EvictionPolicy)
}

func TestMemory_ZeroValues(t *testing.T) {
	c := &Memory{
		MaxEntries:     0,
		MaxMemoryBytes: 0,
		ShardCount:     16,
		EvictionPolicy: "random",
	}
	c.SetDefaults()
	// Zero-value fields should get defaults; non-zero should be preserved.
	assert.Equal(t, 10000, c.MaxEntries)
	assert.Equal(t, int64(104857600), c.MaxMemoryBytes)
	assert.Equal(t, 16, c.ShardCount, "non-zero ShardCount should be preserved")
	assert.Equal(t, "random", c.EvictionPolicy, "non-zero EvictionPolicy should be preserved")
}

func TestMemory_String(t *testing.T) {
	c := DefaultMemory()
	s := c.String()
	assert.Contains(t, s, "Memory{")
	assert.Contains(t, s, "lru")
}

// ---------------------------------------------------------------------------
// Redis config tests
// ---------------------------------------------------------------------------

func TestRedis_SetDefaults(t *testing.T) {
	c := &Redis{}
	c.SetDefaults()

	assert.Equal(t, "localhost:6379", c.Addr)
	assert.Equal(t, 0, c.DB)
	assert.Equal(t, 10, c.PoolSize)
	assert.Equal(t, 2, c.MinIdleConns)
	assert.Equal(t, 3, c.MaxRetries)
	assert.Equal(t, 5*time.Second, c.DialTimeout)
	assert.Equal(t, 3*time.Second, c.ReadTimeout)
	assert.Equal(t, 3*time.Second, c.WriteTimeout)
	assert.Equal(t, 1*time.Hour, c.DefaultTTL)
}

func TestRedis_Validate_ValidDefaults(t *testing.T) {
	c := DefaultRedis()
	err := c.Validate()
	assert.NoError(t, err)
}

func TestRedis_Validate_MissingAddr(t *testing.T) {
	c := &Redis{}
	c.SetDefaults()
	c.Addr = ""
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Addr")
}

func TestRedis_Validate_InvalidDB(t *testing.T) {
	c := DefaultRedis()
	c.DB = 16
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB")
}

func TestRedis_Validate_InvalidPoolSize(t *testing.T) {
	c := DefaultRedis()
	c.PoolSize = 0
	err := c.Validate()
	assert.Error(t, err)
}

func TestRedis_Validate_InvalidPoolSizeTooLarge(t *testing.T) {
	c := DefaultRedis()
	c.PoolSize = 1001
	err := c.Validate()
	assert.Error(t, err)
}

func TestRedis_Validate_InvalidRetries(t *testing.T) {
	c := DefaultRedis()
	c.MaxRetries = 11
	err := c.Validate()
	assert.Error(t, err)
}

func TestRedis_Clone(t *testing.T) {
	c := DefaultRedis()
	c.Password = "secret"
	clone := c.Clone()

	assert.Equal(t, c.Addr, clone.Addr)
	assert.Equal(t, c.Password, clone.Password)
	assert.Equal(t, c.DB, clone.DB)
	assert.Equal(t, c.PoolSize, clone.PoolSize)

	// Modify clone — original should be unaffected.
	clone.PoolSize = 50
	clone.Password = "different"
	assert.Equal(t, 10, c.PoolSize)
	assert.Equal(t, "secret", c.Password)
}

func TestRedis_ZeroValues(t *testing.T) {
	c := &Redis{Addr: "custom:6380"}
	c.SetDefaults()
	// Addr should be preserved (non-empty), everything else gets defaults.
	assert.Equal(t, "custom:6380", c.Addr)
	assert.Equal(t, 10, c.PoolSize)
}

func TestRedis_String(t *testing.T) {
	c := DefaultRedis()
	s := c.String()
	assert.Contains(t, s, "Redis{")
	assert.Contains(t, s, "localhost:6379")
}

// ---------------------------------------------------------------------------
// Layered config tests
// ---------------------------------------------------------------------------

func TestLayered_SetDefaults(t *testing.T) {
	c := &Layered{}
	c.SetDefaults()

	require.NotNil(t, c.L1Config)
	require.NotNil(t, c.L2Config)
	assert.Equal(t, true, c.PromoteOnHit)
	assert.Equal(t, false, c.WriteBack)
	assert.Equal(t, 512, c.WriteBackQueueSize)
	assert.Equal(t, 4, c.WriteBackWorkers)
	assert.Equal(t, 30*time.Second, c.NegativeTTL)
}

func TestLayered_Validate_ValidDefaults(t *testing.T) {
	c := DefaultLayered()
	err := c.Validate()
	assert.NoError(t, err)
}

func TestLayered_Validate_NilL1(t *testing.T) {
	c := &Layered{
		L2Config: DefaultRedis(),
	}
	c.SetDefaults()
	c.L1Config = nil
	err := c.Validate()
	assert.Error(t, err)
}

func TestLayered_Validate_NilL2(t *testing.T) {
	c := &Layered{
		L1Config: DefaultMemory(),
	}
	c.SetDefaults()
	c.L2Config = nil
	err := c.Validate()
	assert.Error(t, err)
}

func TestLayered_Validate_InvalidL1(t *testing.T) {
	c := DefaultLayered()
	c.L1Config.ShardCount = 0
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "L1")
}

func TestLayered_Validate_InvalidL2(t *testing.T) {
	c := DefaultLayered()
	c.L2Config.Addr = ""
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "L2")
}

func TestLayered_Validate_WriteBackQueueSize(t *testing.T) {
	c := DefaultLayered()
	c.WriteBackQueueSize = 0
	err := c.Validate()
	assert.Error(t, err)
}

func TestLayered_Clone(t *testing.T) {
	c := DefaultLayered()
	clone := c.Clone()

	require.NotNil(t, clone.L1Config)
	require.NotNil(t, clone.L2Config)
	assert.Equal(t, c.WriteBack, clone.WriteBack)
	assert.Equal(t, c.PromoteOnHit, clone.PromoteOnHit)

	// Deep modification should not affect original.
	clone.L1Config.MaxEntries = 999
	clone.L2Config.PoolSize = 50
	assert.Equal(t, 10000, c.L1Config.MaxEntries)
	assert.Equal(t, 10, c.L2Config.PoolSize)
}

func TestLayered_Clone_NilSubConfigs(t *testing.T) {
	c := &Layered{}
	clone := c.Clone()
	assert.Nil(t, clone.L1Config)
	assert.Nil(t, clone.L2Config)
}

func TestLayered_String(t *testing.T) {
	c := DefaultLayered()
	s := c.String()
	assert.Contains(t, s, "Layered{")
}

// ---------------------------------------------------------------------------
// translateErrors tests
// ---------------------------------------------------------------------------

func TestTranslateErrors_ValidationError(t *testing.T) {
	c := &Memory{ShardCount: 0}
	err := c.Validate()
	require.Error(t, err)
	// Error message should be human-readable.
	assert.Contains(t, err.Error(), "ShardCount")
}
