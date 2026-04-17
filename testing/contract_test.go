package testing

import (
	"context"
	"testing"
	"time"

	"github.com/os-gomod/cache/memory"
)

func TestRunBackendContractSuite_Memory(t *testing.T) {
	RunBackendContractSuite(t, func() Backend {
		c, err := memory.New()
		if err != nil {
			t.Fatalf("failed to create memory cache: %v", err)
		}
		return c
	})
}

func TestRunBackendContractSuite_MemoryWithTTL(t *testing.T) {
	RunBackendContractSuite(t, func() Backend {
		c, err := memory.New(
			memory.WithMaxEntries(1000),
			memory.WithTTL(30*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create memory cache: %v", err)
		}
		return c
	})
}

func TestRunBackendContractSuite_MemoryWithShards(t *testing.T) {
	RunBackendContractSuite(t, func() Backend {
		c, err := memory.New(memory.WithShards(16))
		if err != nil {
			t.Fatalf("failed to create memory cache: %v", err)
		}
		return c
	})
}

// Additional integration test: large batch operations
func TestMemory_LargeBatchOperations(t *testing.T) {
	c, err := memory.New(memory.WithMaxEntries(10000))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close(context.Background())

	// Set 5000 keys
	items := make(map[string][]byte, 5000)
	for i := 0; i < 5000; i++ {
		items[string(rune('a'+i%26))+string(rune('0'+i%10))] = []byte("value")
	}
	if err := c.SetMulti(context.Background(), items, time.Minute); err != nil {
		t.Fatalf("SetMulti: %v", err)
	}
}
