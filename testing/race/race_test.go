// Package race provides race-condition tests for the cache library. All
// tests exercise concurrent access to shared state and must be run with
// the -race flag:
//
//	go test -race -count=100 -timeout=120s ./testing/race/
package race

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/memory"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/resilience"
	"github.com/os-gomod/cache/stampede"
)

// ---------------------------------------------------------------------------
// TestRace_ConcurrentGetSet — 500 goroutines read/write same key
// ---------------------------------------------------------------------------

func TestRace_ConcurrentGetSet(t *testing.T) {
	c, err := memory.New(memory.WithShards(4))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	_ = c.Set(context.Background(), "hot-key", []byte("initial"), 0)

	const goroutines = 500
	const opsPerGoroutine = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if j%2 == 0 {
					_, _ = c.Get(context.Background(), "hot-key")
				} else {
					val := []byte{byte(id % 256)}
					_ = c.Set(context.Background(), "hot-key", val, 0)
				}
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// TestRace_ConcurrentClose — 10 goroutines call Close() simultaneously
// ---------------------------------------------------------------------------

func TestRace_ConcurrentClose(t *testing.T) {
	const attempts = 50 // run multiple times for higher confidence
	for n := 0; n < attempts; n++ {
		c, err := memory.New()
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		const goroutines = 10
		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = c.Close(context.Background())
			}()
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// TestRace_CircuitBreaker — 200 goroutines call Allow/Success/Failure
// ---------------------------------------------------------------------------

func TestRace_CircuitBreaker(t *testing.T) {
	cb := resilience.NewCircuitBreaker(5, 100*time.Millisecond)

	const goroutines = 200
	const opsPerGoroutine = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if cb.Allow() {
					if id%3 == 0 {
						cb.Success()
					} else {
						cb.Failure()
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// TestRace_ResiliencePolicy — 100 goroutines Execute() concurrently
// ---------------------------------------------------------------------------

func TestRace_ResiliencePolicy(t *testing.T) {
	c, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	policy := resilience.Policy{
		CircuitBreaker: resilience.NewCircuitBreaker(10, 200*time.Millisecond),
		Retry: resilience.RetryConfig{
			MaxAttempts:  2,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     5 * time.Millisecond,
			Multiplier:   2.0,
		},
	}

	rc := resilience.NewCacheWithPolicy(c, policy)
	defer func() { _ = rc.Close(context.Background()) }()

	_ = rc.Set(context.Background(), "race-resilience", []byte("v"), 0)

	const goroutines = 100
	const opsPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_, _ = rc.Get(context.Background(), "race-resilience")
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// TestRace_StampedeDetector — 50 goroutines GetOrSet same key simultaneously
// ---------------------------------------------------------------------------

func TestRace_StampedeDetector(t *testing.T) {
	c, err := memory.New(memory.WithShards(4))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = c.Close(context.Background()) }()

	// Pre-populate the key so GetOrSet can hit and trigger stampede checks.
	_ = c.Set(context.Background(), "stampede-key", []byte("val"), 5*time.Minute)

	const goroutines = 50
	const opsPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				val, err := c.GetOrSet(context.Background(), "stampede-key",
					func() ([]byte, error) {
						return []byte("computed"), nil
					}, 5*time.Minute)
				if err != nil {
					t.Logf("GetOrSet error: %v", err)
				}
				_ = val
			}
		}()
	}
	wg.Wait()
	_ = observability.NopChain() // ensure import is used
}

// ---------------------------------------------------------------------------
// TestRace_StampedeDetectorDirect — direct detector.Do calls
// ---------------------------------------------------------------------------

func TestRace_StampedeDetectorDirect(t *testing.T) {
	d := stampede.NewDetector(1.0, observability.NopChain())
	defer d.Close()

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := d.Do(context.Background(), "sg-key", nil, nil,
				func(ctx context.Context) ([]byte, error) {
					return []byte("result"), nil
				},
				nil,
			)
			if err != nil {
				t.Logf("detector.Do error: %v", err)
			}
			_ = val
		}()
	}
	wg.Wait()
}
