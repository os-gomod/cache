// Package clock_test provides tests for shared monotonic nanosecond clock.
package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/clock"
)

func TestNow_ReturnsValue(t *testing.T) {
	now := clock.Now()
	if now <= 0 {
		t.Errorf("Now() = %d, should be > 0", now)
	}
}

func TestNow_MonotonicIncreasing(t *testing.T) {
	// Get multiple values and ensure they're non-decreasing
	first := clock.Now()
	time.Sleep(2 * time.Millisecond)
	second := clock.Now()
	time.Sleep(2 * time.Millisecond)
	third := clock.Now()

	if second < first {
		t.Errorf("clock went backwards: %d -> %d", first, second)
	}
	if third < second {
		t.Errorf("clock went backwards: %d -> %d", second, third)
	}
}

func TestNow_ReturnsRecentValue(t *testing.T) {
	// Since the clock updates every millisecond, the value should be
	// at most a few milliseconds old
	before := time.Now().UnixNano()
	time.Sleep(1 * time.Millisecond)
	clockNow := clock.Now()
	after := time.Now().UnixNano()

	if clockNow < before {
		t.Errorf("clock.Now() = %d is before time.Now() = %d", clockNow, before)
	}
	if clockNow > after+int64(2*time.Millisecond) {
		t.Errorf("clock.Now() = %d is too far in future (%d)", clockNow, after)
	}
}

func TestStartClock_OnlyOnce(t *testing.T) {
	// Reset the clock for testing by creating a new package state
	// This is a bit tricky since we can't easily reset package-level variables
	// Instead we verify that multiple calls don't cause issues

	// Record the current time
	firstNow := clock.Now()
	time.Sleep(2 * time.Millisecond)

	// Call StartClock again (should be no-op after init)
	clock.StartClock()
	time.Sleep(2 * time.Millisecond)

	secondNow := clock.Now()
	if secondNow <= firstNow {
		t.Errorf("clock did not advance: %d -> %d", firstNow, secondNow)
	}
}

func TestNow_ConcurrentReads(t *testing.T) {
	var wg sync.WaitGroup
	const numGoroutines = 100
	const readsPerGoroutine = 1000

	values := make([]int64, numGoroutines*readsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				values[offset*readsPerGoroutine+j] = clock.Now()
			}
		}(i)
	}

	wg.Wait()

	// Verify all values are positive
	for _, v := range values {
		if v <= 0 {
			t.Errorf("got non-positive value: %d", v)
		}
	}
}

func TestNow_Accuracy(t *testing.T) {
	// Test that clock.Now() is reasonably close to time.Now()
	const iterations = 10
	const tolerance = 5 * time.Millisecond

	for i := 0; i < iterations; i++ {
		clockNow := clock.Now()
		timeNow := time.Now().UnixNano()

		diff := timeNow - clockNow
		if diff < 0 {
			diff = -diff
		}

		if time.Duration(diff) > tolerance {
			t.Errorf("iteration %d: clock.Now() diff = %v, tolerance %v",
				i, time.Duration(diff), tolerance)
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func TestNow_StaleValue(t *testing.T) {
	// Verify that values are updated by the background ticker
	first := clock.Now()
	time.Sleep(3 * time.Millisecond)
	second := clock.Now()

	if second <= first {
		t.Errorf("value not updated after 3ms: %d -> %d", first, second)
	}
}

func TestNow_ConcurrentWithTicker(_ *testing.T) {
	// Test that concurrent reads don't interfere with the background ticker
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Start background reader
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_ = clock.Now()
					time.Sleep(100 * time.Microsecond)
				}
			}
		}()
	}

	// Let it run for a while
	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()
}

func TestNow_AfterLongDelay(t *testing.T) {
	// Test that the clock continues to advance correctly after a longer delay
	first := clock.Now()
	time.Sleep(10 * time.Millisecond)
	second := clock.Now()

	diff := second - first
	if diff <= 0 {
		t.Errorf("clock did not advance: %d -> %d", first, second)
	}

	// Allow for some timing jitter (should be at least 10ms, but could be slightly more)
	if time.Duration(diff) < 10*time.Millisecond {
		t.Errorf("clock advanced only %v, expected at least 10ms", time.Duration(diff))
	}
}

func TestNow_WithinBounds(t *testing.T) {
	// Test that clock.Now() is always within a reasonable range
	now := clock.Now()
	minTime := int64(1609459200000000000) // 2021-01-01 UTC
	maxTime := int64(4102444800000000000) // 2100-01-01 UTC

	if now < minTime {
		t.Errorf("clock.Now() = %d is before year 2021", now)
	}
	if now > maxTime {
		t.Errorf("clock.Now() = %d is after year 2100", now)
	}
}

func TestNow_Performance(t *testing.T) {
	// Verify that Now() is fast (no syscalls)
	start := time.Now()
	for i := 0; i < 1000000; i++ {
		_ = clock.Now()
	}
	elapsed := time.Since(start)

	// Should be very fast (under 100ms on modern hardware)
	if elapsed > 200*time.Millisecond {
		t.Logf("Warning: 1M calls took %v", elapsed)
	}
}

func TestStartClock_Idempotent(t *testing.T) {
	// StartClock should be idempotent
	firstNow := clock.Now()
	time.Sleep(2 * time.Millisecond)

	// Call StartClock multiple times
	clock.StartClock()
	clock.StartClock()
	clock.StartClock()

	time.Sleep(2 * time.Millisecond)
	secondNow := clock.Now()

	if secondNow <= firstNow {
		t.Errorf("clock did not advance after multiple StartClock calls")
	}
}

func TestNow_ValuesInRange(t *testing.T) {
	// Test that values are always within expected range
	values := make([]int64, 1000)
	for i := 0; i < len(values); i++ {
		values[i] = clock.Now()
		time.Sleep(100 * time.Microsecond)
	}

	// Verify monotonicity
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			t.Errorf("value decreased at index %d: %d -> %d", i, values[i-1], values[i])
		}
	}

	// Verify all values are positive
	for i, v := range values {
		if v <= 0 {
			t.Errorf("value at index %d is non-positive: %d", i, v)
		}
	}
}

func TestNow_ComparisonWithTimeNow(t *testing.T) {
	// Test that clock.Now() and time.Now() are reasonably close
	const iterations = 100
	const maxDiff = 10 * time.Millisecond

	for i := 0; i < iterations; i++ {
		clockNow := clock.Now()
		timeNow := time.Now().UnixNano()

		diff := timeNow - clockNow
		if diff < 0 {
			diff = -diff
		}

		if time.Duration(diff) > maxDiff {
			t.Errorf("iteration %d: diff %v exceeds %v",
				i, time.Duration(diff), maxDiff)
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func TestNow_ConcurrentReadsWithSleep(t *testing.T) {
	var wg sync.WaitGroup
	const numGoroutines = 50
	const numIterations = 100

	results := make([][]int64, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = make([]int64, numIterations)
			for j := 0; j < numIterations; j++ {
				results[idx][j] = clock.Now()
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify all values are positive and roughly increasing within each goroutine
	for i, values := range results {
		for j := 1; j < len(values); j++ {
			if values[j] < values[j-1] {
				t.Errorf("goroutine %d: value decreased at index %d: %d -> %d",
					i, j, values[j-1], values[j])
			}
		}
	}
}

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = clock.Now()
	}
}

func BenchmarkTimeNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = time.Now().UnixNano()
	}
}

func BenchmarkNow_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = clock.Now()
		}
	})
}

func BenchmarkTimeNow_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = time.Now().UnixNano()
		}
	})
}

func TestNow_Initialization(t *testing.T) {
	// Verify that the clock is initialized by init()
	now := clock.Now()
	if now == 0 {
		t.Error("clock not initialized")
	}
}

func TestNow_AfterSystemSleep(t *testing.T) {
	// Simulate a delay that might cause the clock to be stale
	first := clock.Now()
	time.Sleep(5 * time.Millisecond)
	second := clock.Now()

	if second <= first {
		t.Errorf("clock not updated after sleep: %d -> %d", first, second)
	}
}

func TestNow_TimeDifference(t *testing.T) {
	// Test that the difference between two calls approximates real time
	start := clock.Now()
	time.Sleep(5 * time.Millisecond)
	end := clock.Now()

	diff := time.Duration(end - start)
	if diff < 5*time.Millisecond {
		t.Errorf("difference too small: %v, expected at least 5ms", diff)
	}
	if diff > 10*time.Millisecond {
		t.Logf("difference larger than expected: %v", diff)
	}
}
