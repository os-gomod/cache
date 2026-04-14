package layer

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/memory"
)

func TestLayered_Options(t *testing.T) {
	_ = WithL1MaxEntries(100)
	_ = WithL1TTL(time.Minute)
	_ = WithL1LRU()
	_ = WithL1LFU()
	_ = WithL1FIFO()
	_ = WithL1LIFO()
	_ = WithL1MRU()
	_ = WithL1Random()
	_ = WithL1TinyLFU()
	_ = WithL1CleanupInterval(30 * time.Second)
	_ = WithL1Shards(32)
	_ = WithPromoteOnHit(true)
	_ = WithNegativeTTL(30 * time.Second)
	_ = WithWriteBack(false)
}

func TestLayered_L1Direct(t *testing.T) {
	opts := []Option{
		WithL1MaxEntries(100),
		WithL1TTL(time.Minute),
	}
	_ = opts
}

func TestMergeOptions(t *testing.T) {
	cfg, err := MergeOptions(
		WithL1MaxEntries(500),
		WithL1TTL(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("MergeOptions failed: %v", err)
	}
	if cfg.L1Config.MaxEntries != 500 {
		t.Errorf("MaxEntries = %d, want 500", cfg.L1Config.MaxEntries)
	}
	if cfg.L1Config.DefaultTTL != 5*time.Minute {
		t.Errorf("DefaultTTL = %v, want 5m", cfg.L1Config.DefaultTTL)
	}
}

func TestCache_CheckClosed(t *testing.T) {
	var c Cache
	err := c.checkClosed("test.op")
	if err != nil {
		t.Errorf("open cache should not return error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewWithBackends (DI constructor)
// ---------------------------------------------------------------------------

func TestNewWithBackends(t *testing.T) {
	l1, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New for L1 failed: %v", err)
	}
	defer func() { _ = l1.Close(context.Background()) }()

	l2, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New for L2 failed: %v", err)
	}
	defer func() { _ = l2.Close(context.Background()) }()

	lc, err := NewWithBackends(context.Background(), l1, l2)
	if err != nil {
		t.Fatalf("NewWithBackends failed: %v", err)
	}
	defer func() { _ = lc.Close(context.Background()) }()

	if err := lc.Set(context.Background(), "di-key", []byte("di-val"), 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	val, err := lc.Get(context.Background(), "di-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "di-val" {
		t.Errorf("Get = %q, want %q", val, "di-val")
	}
}

// // ---------------------------------------------------------------------------
// // Backend Contract
// // ---------------------------------------------------------------------------

// func TestBackendContract(t *testing.T) {
// 	cachetesting.RunBackendContractSuite(t, func() BackendBackend {
// 		l1, err := memory.New()
// 		if err != nil {
// 			t.Fatalf("memory.New for L1 failed: %v", err)
// 		}
// 		l2, err := memory.New()
// 		if err != nil {
// 			t.Fatalf("memory.New for L2 failed: %v", err)
// 		}
// 		lc, err := NewWithBackends(context.Background(), l1, l2)
// 		if err != nil {
// 			t.Fatalf("NewWithBackends failed: %v", err)
// 		}
// 		return lc
// 	})
// }

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	l1, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New for L1 failed: %v", err)
	}
	defer func() { _ = l1.Close(context.Background()) }()

	l2, err := memory.New()
	if err != nil {
		t.Fatalf("memory.New for L2 failed: %v", err)
	}
	defer func() { _ = l2.Close(context.Background()) }()

	lc, err := NewWithBackends(context.Background(), l1, l2)
	if err != nil {
		t.Fatalf("NewWithBackends failed: %v", err)
	}
	defer func() { _ = lc.Close(context.Background()) }()

	l1Err, l2Err := lc.HealthCheck(context.Background())
	if l1Err != nil {
		t.Errorf("L1 health check failed: %v", l1Err)
	}
	if l2Err != nil {
		t.Errorf("L2 health check failed: %v", l2Err)
	}
}
