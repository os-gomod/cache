// Package stats_test provides tests for atomic, lock-free cache statistics.
package stats_test

import (
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/internal/stats"
)

func TestNewStats(t *testing.T) {
	s := stats.NewStats()
	if s == nil {
		t.Fatal("NewStats returned nil")
	}

	// Verify start time is set
	if s.Uptime() <= 0 {
		t.Error("Uptime should be > 0")
	}

	// All counters should be zero
	verifyZeroStats(t, s)
}

func verifyZeroStats(t *testing.T, s *stats.Stats) {
	t.Helper()

	checks := []struct {
		name string
		got  int64
	}{
		{"Hits", s.Hits()},
		{"Misses", s.Misses()},
		{"Gets", s.Gets()},
		{"Sets", s.Sets()},
		{"Deletes", s.Deletes()},
		{"Evictions", s.Evictions()},
		{"Errors", s.Errors()},
		{"Items", s.Items()},
		{"Memory", s.Memory()},
		{"L1Hits", s.L1Hits()},
		{"L1Misses", s.L1Misses()},
		{"L1Errors", s.L1Errors()},
		{"L2Hits", s.L2Hits()},
		{"L2Misses", s.L2Misses()},
		{"L2Errors", s.L2Errors()},
		{"L2Promotions", s.L2Promotions()},
		{"WriteBackEnqueued", s.WriteBackEnqueued()},
		{"WriteBackFlushed", s.WriteBackFlushed()},
		{"WriteBackDropped", s.WriteBackDropped()},
	}

	for _, check := range checks {
		if check.got != 0 {
			t.Errorf("%s = %d, want 0", check.name, check.got)
		}
	}

	if s.HitRate() != 0 {
		t.Errorf("HitRate = %f, want 0", s.HitRate())
	}
	if s.L1HitRate() != 0 {
		t.Errorf("L1HitRate = %f, want 0", s.L1HitRate())
	}
	if s.L2HitRate() != 0 {
		t.Errorf("L2HitRate = %f, want 0", s.L2HitRate())
	}
}

func TestStats_BasicOperations(t *testing.T) {
	s := stats.NewStats()

	// Test individual operations
	s.Hit()
	s.Miss()
	s.GetOp()
	s.SetOp()
	s.DeleteOp()
	s.EvictionOp()
	s.ErrorOp()
	s.AddItems(5)
	s.AddMemory(1024)

	if got := s.Hits(); got != 1 {
		t.Errorf("Hits = %d, want 1", got)
	}
	if got := s.Misses(); got != 1 {
		t.Errorf("Misses = %d, want 1", got)
	}
	if got := s.Gets(); got != 1 {
		t.Errorf("Gets = %d, want 1", got)
	}
	if got := s.Sets(); got != 1 {
		t.Errorf("Sets = %d, want 1", got)
	}
	if got := s.Deletes(); got != 1 {
		t.Errorf("Deletes = %d, want 1", got)
	}
	if got := s.Evictions(); got != 1 {
		t.Errorf("Evictions = %d, want 1", got)
	}
	if got := s.Errors(); got != 1 {
		t.Errorf("Errors = %d, want 1", got)
	}
	if got := s.Items(); got != 5 {
		t.Errorf("Items = %d, want 5", got)
	}
	if got := s.Memory(); got != 1024 {
		t.Errorf("Memory = %d, want 1024", got)
	}
}

func TestStats_LayeredOperations(t *testing.T) {
	s := stats.NewStats()

	// L1 operations
	s.L1Hit()
	s.L1Miss()
	s.L1Error()

	// L2 operations
	s.L2Hit()
	s.L2Miss()
	s.L2Error()
	s.L2Promotion()

	if got := s.L1Hits(); got != 1 {
		t.Errorf("L1Hits = %d, want 1", got)
	}
	if got := s.L1Misses(); got != 1 {
		t.Errorf("L1Misses = %d, want 1", got)
	}
	if got := s.L1Errors(); got != 1 {
		t.Errorf("L1Errors = %d, want 1", got)
	}
	if got := s.L2Hits(); got != 1 {
		t.Errorf("L2Hits = %d, want 1", got)
	}
	if got := s.L2Misses(); got != 1 {
		t.Errorf("L2Misses = %d, want 1", got)
	}
	if got := s.L2Errors(); got != 1 {
		t.Errorf("L2Errors = %d, want 1", got)
	}
	if got := s.L2Promotions(); got != 1 {
		t.Errorf("L2Promotions = %d, want 1", got)
	}

	// Verify that L1Hit and L2Hit also increment global hits
	if got := s.Hits(); got != 1 {
		t.Errorf("Hits = %d, want 1", got)
	}
	if got := s.Errors(); got != 2 {
		t.Errorf("Errors = %d, want 2", got)
	}
}

func TestStats_WriteBackOperations(t *testing.T) {
	s := stats.NewStats()

	s.WriteBackEnqueue()
	s.WriteBackFlush()
	s.WriteBackDrop()

	if got := s.WriteBackEnqueued(); got != 1 {
		t.Errorf("WriteBackEnqueued = %d, want 1", got)
	}
	if got := s.WriteBackFlushed(); got != 1 {
		t.Errorf("WriteBackFlushed = %d, want 1", got)
	}
	if got := s.WriteBackDropped(); got != 1 {
		t.Errorf("WriteBackDropped = %d, want 1", got)
	}
}

func TestStats_HitRate(t *testing.T) {
	tests := []struct {
		name   string
		hits   int64
		misses int64
		want   float64
	}{
		{"no operations", 0, 0, 0},
		{"all hits", 10, 0, 100},
		{"all misses", 0, 10, 0},
		{"mixed", 7, 3, 70},
		{"equal", 5, 5, 50},
		{"large numbers", 750, 250, 75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := stats.NewStats()
			for i := int64(0); i < tt.hits; i++ {
				s.Hit()
			}
			for i := int64(0); i < tt.misses; i++ {
				s.Miss()
			}
			if got := s.HitRate(); got != tt.want {
				t.Errorf("HitRate() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestStats_L1HitRate(t *testing.T) {
	s := stats.NewStats()

	s.L1Hit()
	s.L1Hit()
	s.L1Miss()

	if got := s.L1HitRate(); got != 66.66666666666666 {
		t.Errorf("L1HitRate() = %f, want 66.66666666666666", got)
	}
}

func TestStats_L2HitRate(t *testing.T) {
	s := stats.NewStats()

	s.L2Hit()
	s.L2Miss()
	s.L2Miss()

	if got := s.L2HitRate(); got != 33.33333333333333 {
		t.Errorf("L2HitRate() = %f, want 33.33333333333333", got)
	}
}

func TestStats_OpsPerSecond(t *testing.T) {
	s := stats.NewStats()

	// Add some operations
	s.GetOp()
	s.GetOp()
	s.SetOp()

	// Give it a small delay to ensure time passes
	time.Sleep(10 * time.Millisecond)

	opsPerSec := s.OpsPerSecond()
	if opsPerSec <= 0 {
		t.Errorf("OpsPerSecond = %f, should be > 0", opsPerSec)
	}

	// Upper bound check - shouldn't be astronomical
	if opsPerSec > 10000 {
		t.Errorf("OpsPerSecond = %f, seems too high", opsPerSec)
	}
}

func TestStats_Uptime(t *testing.T) {
	s := stats.NewStats()

	uptime := s.Uptime()
	if uptime <= 0 {
		t.Errorf("Uptime = %v, should be > 0", uptime)
	}

	time.Sleep(50 * time.Millisecond)

	uptime2 := s.Uptime()
	if uptime2 <= uptime {
		t.Errorf("Uptime did not increase: %v -> %v", uptime, uptime2)
	}
}

func TestStats_TakeSnapshot(t *testing.T) {
	s := stats.NewStats()

	// Record some stats
	s.Hit()
	s.Miss()
	s.GetOp()
	s.SetOp()
	s.DeleteOp()
	s.EvictionOp()
	s.ErrorOp()
	s.AddItems(10)
	s.AddMemory(2048)
	s.L1Hit()
	s.L1Miss()
	s.L2Hit()
	s.L2Miss()
	s.L2Promotion()
	s.WriteBackEnqueue()
	s.WriteBackFlush()

	snapshot := s.TakeSnapshot()

	// Verify snapshot values
	if snapshot.Hits != 2 { // Hit + L1Hit
		t.Errorf("Snapshot.Hits = %d, want 2", snapshot.Hits)
	}
	if snapshot.Misses != 1 {
		t.Errorf("Snapshot.Misses = %d, want 1", snapshot.Misses)
	}
	if snapshot.Gets != 1 {
		t.Errorf("Snapshot.Gets = %d, want 1", snapshot.Gets)
	}
	if snapshot.Sets != 1 {
		t.Errorf("Snapshot.Sets = %d, want 1", snapshot.Sets)
	}
	if snapshot.Deletes != 1 {
		t.Errorf("Snapshot.Deletes = %d, want 1", snapshot.Deletes)
	}
	if snapshot.Evictions != 1 {
		t.Errorf("Snapshot.Evictions = %d, want 1", snapshot.Evictions)
	}
	if snapshot.Errors != 1 {
		t.Errorf("Snapshot.Errors = %d, want 1", snapshot.Errors)
	}
	if snapshot.Items != 10 {
		t.Errorf("Snapshot.Items = %d, want 10", snapshot.Items)
	}
	if snapshot.Memory != 2048 {
		t.Errorf("Snapshot.Memory = %d, want 2048", snapshot.Memory)
	}
	if snapshot.L1Hits != 1 {
		t.Errorf("Snapshot.L1Hits = %d, want 1", snapshot.L1Hits)
	}
	if snapshot.L1Misses != 1 {
		t.Errorf("Snapshot.L1Misses = %d, want 1", snapshot.L1Misses)
	}
	if snapshot.L2Hits != 1 {
		t.Errorf("Snapshot.L2Hits = %d, want 1", snapshot.L2Hits)
	}
	if snapshot.L2Misses != 1 {
		t.Errorf("Snapshot.L2Misses = %d, want 1", snapshot.L2Misses)
	}
	if snapshot.L2Promotions != 1 {
		t.Errorf("Snapshot.L2Promotions = %d, want 1", snapshot.L2Promotions)
	}
	if snapshot.WriteBackEnqueued != 1 {
		t.Errorf("Snapshot.WriteBackEnqueued = %d, want 1", snapshot.WriteBackEnqueued)
	}
	if snapshot.WriteBackFlushed != 1 {
		t.Errorf("Snapshot.WriteBackFlushed = %d, want 1", snapshot.WriteBackFlushed)
	}
}

func TestStats_Reset(t *testing.T) {
	s := stats.NewStats()

	// Add some stats
	s.Hit()
	s.Miss()
	s.GetOp()
	s.SetOp()
	s.AddItems(100)
	s.AddMemory(1024)

	time.Sleep(10 * time.Millisecond)

	// Reset
	s.Reset()

	// All counters should be zero
	verifyZeroStats(t, s)

	// Uptime should be near zero again after reset.
	if uptimeAfter := s.Uptime(); uptimeAfter >= 5*time.Millisecond {
		t.Errorf("Uptime not reset: after=%v", uptimeAfter)
	}
}

func TestStats_ConcurrentOperations(t *testing.T) {
	s := stats.NewStats()
	const numGoroutines = 100
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				s.Hit()
				s.Miss()
				s.GetOp()
				s.SetOp()
				s.L1Hit()
				s.L2Hit()
				s.WriteBackEnqueue()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * opsPerGoroutine)
	if got := s.Hits(); got != expected*2 { // Hit + L1Hit each iteration
		t.Errorf("Hits = %d, want %d", got, expected*2)
	}
	if got := s.Misses(); got != expected {
		t.Errorf("Misses = %d, want %d", got, expected)
	}
	if got := s.Gets(); got != expected {
		t.Errorf("Gets = %d, want %d", got, expected)
	}
	if got := s.Sets(); got != expected {
		t.Errorf("Sets = %d, want %d", got, expected)
	}
	if got := s.L1Hits(); got != expected {
		t.Errorf("L1Hits = %d, want %d", got, expected)
	}
	if got := s.L2Hits(); got != expected {
		t.Errorf("L2Hits = %d, want %d", got, expected)
	}
}

func TestStats_ConcurrentReset(_ *testing.T) {
	s := stats.NewStats()
	const numGoroutines = 50
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Start goroutines that increment stats
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				s.Hit()
				s.Miss()
			}
		}()
	}

	// Start goroutines that reset stats
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.Reset()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// No panic means concurrent access is safe
	// Values may be inconsistent due to reset, but that's expected
	_ = s.Hits()
	_ = s.Misses()
}

func TestStats_SnapshotConsistency(t *testing.T) {
	s := stats.NewStats()

	// Add some stats
	s.Hit()
	s.Miss()
	s.GetOp()

	snapshot := s.TakeSnapshot()

	// Modify stats after snapshot
	s.Hit()
	s.Miss()

	// Snapshot should not change
	if snapshot.Hits != 1 {
		t.Errorf("Snapshot.Hits = %d, want 1", snapshot.Hits)
	}
	if snapshot.Misses != 1 {
		t.Errorf("Snapshot.Misses = %d, want 1", snapshot.Misses)
	}
	if snapshot.Gets != 1 {
		t.Errorf("Snapshot.Gets = %d, want 1", snapshot.Gets)
	}
}

func TestStats_DeprecatedSnapshot(t *testing.T) {
	s := stats.NewStats()
	s.Hit()

	// Test deprecated method still works
	snapshot := s.Snapshot()
	if snapshot.Hits != 1 {
		t.Errorf("Snapshot.Hits = %d, want 1", snapshot.Hits)
	}
}

func TestStats_HitRateZeroTotal(t *testing.T) {
	s := stats.NewStats()

	// No hits or misses
	if got := s.HitRate(); got != 0 {
		t.Errorf("HitRate() = %f, want 0", got)
	}
	if got := s.L1HitRate(); got != 0 {
		t.Errorf("L1HitRate() = %f, want 0", got)
	}
	if got := s.L2HitRate(); got != 0 {
		t.Errorf("L2HitRate() = %f, want 0", got)
	}
}

func TestStats_AddNegativeValues(t *testing.T) {
	s := stats.NewStats()

	s.AddItems(-5)
	s.AddMemory(-1024)

	if got := s.Items(); got != -5 {
		t.Errorf("Items = %d, want -5", got)
	}
	if got := s.Memory(); got != -1024 {
		t.Errorf("Memory = %d, want -1024", got)
	}
}

func BenchmarkStats_Hit(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Hit()
		}
	})
}

func BenchmarkStats_Miss(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Miss()
		}
	})
}

func BenchmarkStats_L1Hit(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.L1Hit()
		}
	})
}

func BenchmarkStats_L2Hit(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.L2Hit()
		}
	})
}

func BenchmarkStats_WriteBackEnqueue(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.WriteBackEnqueue()
		}
	})
}

func BenchmarkStats_HitRate(b *testing.B) {
	s := stats.NewStats()
	for i := 0; i < 1000; i++ {
		s.Hit()
		s.Miss()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.HitRate()
	}
}

func BenchmarkStats_TakeSnapshot(b *testing.B) {
	s := stats.NewStats()
	for i := 0; i < 1000; i++ {
		s.Hit()
		s.Miss()
		s.GetOp()
		s.SetOp()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.TakeSnapshot()
	}
}

func BenchmarkStats_Reset(b *testing.B) {
	s := stats.NewStats()
	for i := 0; i < b.N; i++ {
		s.Reset()
	}
}

func BenchmarkStats_OpsPerSecond(b *testing.B) {
	s := stats.NewStats()
	for i := 0; i < 1000; i++ {
		s.GetOp()
		s.SetOp()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.OpsPerSecond()
	}
}

func BenchmarkStats_ConcurrentMixed(b *testing.B) {
	s := stats.NewStats()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Hit()
			s.Miss()
			s.GetOp()
			s.SetOp()
			s.L1Hit()
			s.L2Hit()
			s.WriteBackEnqueue()
		}
	})
}
