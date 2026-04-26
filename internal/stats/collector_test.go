package stats

import (
	"sync"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
)

func TestNewStats(t *testing.T) {
	s := NewStats()
	if s == nil {
		t.Fatal("expected non-nil Stats")
	}
	if s.StartTime().IsZero() {
		t.Error("expected non-zero start time")
	}
}

func TestStatsCounters(t *testing.T) {
	s := NewStats()

	tests := []struct {
		name      string
		increment func()
		reader    func() int64
	}{
		{"Hit", s.Hit, s.Hits},
		{"Miss", s.Miss, s.Misses},
		{"Set", s.Set, s.Sets},
		{"Delete", s.Delete, s.Deletes},
		{"Error", s.Error, s.Errors},
		{"Eviction", s.Eviction, s.Evictions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.reader()
			tt.increment()
			tt.increment()
			tt.increment()
			after := tt.reader()
			if after != before+3 {
				t.Errorf("expected %d increments, got delta=%d (before=%d, after=%d)",
					3, after-before, before, after)
			}
		})
	}
}

func TestStatsMemory(t *testing.T) {
	s := NewStats()

	s.AddMemory(1024)
	if s.Memory() != 1024 {
		t.Errorf("expected 1024, got %d", s.Memory())
	}

	s.AddMemory(512)
	if s.Memory() != 1536 {
		t.Errorf("expected 1536, got %d", s.Memory())
	}

	s.SubMemory(256)
	if s.Memory() != 1280 {
		t.Errorf("expected 1280, got %d", s.Memory())
	}
}

func TestStatsItems(t *testing.T) {
	s := NewStats()

	s.AddItems(10)
	if s.Items() != 10 {
		t.Errorf("expected 10, got %d", s.Items())
	}

	s.SubItems(3)
	if s.Items() != 7 {
		t.Errorf("expected 7, got %d", s.Items())
	}

	s.AddItems(-2)
	if s.Items() != 5 {
		t.Errorf("expected 5, got %d", s.Items())
	}
}

func TestStatsHitRate(t *testing.T) {
	s := NewStats()

	t.Run("zero operations", func(t *testing.T) {
		if rate := s.HitRate(); rate != 0 {
			t.Errorf("expected 0, got %f", rate)
		}
	})

	t.Run("all hits", func(t *testing.T) {
		s.Reset()
		s.Hit()
		s.Hit()
		s.Hit()
		if rate := s.HitRate(); rate != 1.0 {
			t.Errorf("expected 1.0, got %f", rate)
		}
	})

	t.Run("50/50", func(t *testing.T) {
		s.Reset()
		s.Hit()
		s.Miss()
		if rate := s.HitRate(); rate != 0.5 {
			t.Errorf("expected 0.5, got %f", rate)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		s.Reset()
		for i := 0; i < 75; i++ {
			s.Hit()
		}
		for i := 0; i < 25; i++ {
			s.Miss()
		}
		if rate := s.HitRate(); rate != 0.75 {
			t.Errorf("expected 0.75, got %f", rate)
		}
	})
}

func TestStatsTakeSnapshot(t *testing.T) {
	s := NewStats()

	s.Hit()
	s.Hit()
	s.Miss()
	s.Set()
	s.Delete()
	s.Eviction()
	s.Error()
	s.AddMemory(2048)
	s.AddItems(5)
	s.SetMaxMemory(4096)

	snapshot := s.TakeSnapshot()

	if snapshot.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", snapshot.Hits)
	}
	if snapshot.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", snapshot.Misses)
	}
	if snapshot.Sets != 1 {
		t.Errorf("expected 1 set, got %d", snapshot.Sets)
	}
	if snapshot.Deletes != 1 {
		t.Errorf("expected 1 delete, got %d", snapshot.Deletes)
	}
	if snapshot.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", snapshot.Evictions)
	}
	if snapshot.Errors != 1 {
		t.Errorf("expected 1 error, got %d", snapshot.Errors)
	}
	if snapshot.Items != 5 {
		t.Errorf("expected 5 items, got %d", snapshot.Items)
	}
	if snapshot.MemoryBytes != 2048 {
		t.Errorf("expected 2048 memory bytes, got %d", snapshot.MemoryBytes)
	}
	if snapshot.MaxMemory != 4096 {
		t.Errorf("expected 4096 max memory, got %d", snapshot.MaxMemory)
	}
	if snapshot.StartTime.IsZero() {
		t.Error("expected non-zero start time in snapshot")
	}
}

func TestStatsProviderInterface(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Set()

	var provider contracts.StatsProvider = s
	snapshot := provider.Stats()

	if snapshot.Hits != 1 {
		t.Errorf("expected 1 hit via StatsProvider, got %d", snapshot.Hits)
	}
	if snapshot.Sets != 1 {
		t.Errorf("expected 1 set via StatsProvider, got %d", snapshot.Sets)
	}
}

func TestStatsReset(t *testing.T) {
	s := NewStats()

	// Populate some stats
	s.Hit()
	s.Hit()
	s.Miss()
	s.Set()
	s.AddMemory(1024)
	s.AddItems(5)

	// Reset
	s.Reset()

	// Verify all counters are zero
	if s.Hits() != 0 {
		t.Errorf("expected 0 hits after reset, got %d", s.Hits())
	}
	if s.Misses() != 0 {
		t.Errorf("expected 0 misses after reset, got %d", s.Misses())
	}
	if s.Sets() != 0 {
		t.Errorf("expected 0 sets after reset, got %d", s.Sets())
	}
	if s.Memory() != 0 {
		t.Errorf("expected 0 memory after reset, got %d", s.Memory())
	}
	if s.Items() != 0 {
		t.Errorf("expected 0 items after reset, got %d", s.Items())
	}

	// Start time should be updated
	if s.StartTime().IsZero() {
		t.Error("expected non-zero start time after reset")
	}
}

func TestStatsConcurrent(t *testing.T) {
	s := NewStats()
	var wg sync.WaitGroup
	const goroutines = 50
	const opsPer = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPer; j++ {
				s.Hit()
				s.Miss()
				s.Set()
				s.Delete()
				s.Error()
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * opsPer)
	if got := s.Hits(); got != expected {
		t.Errorf("expected %d hits, got %d", expected, got)
	}
	if got := s.Misses(); got != expected {
		t.Errorf("expected %d misses, got %d", expected, got)
	}
	if got := s.Sets(); got != expected {
		t.Errorf("expected %d sets, got %d", expected, got)
	}
}

func TestStatsMaxMemory(t *testing.T) {
	s := NewStats()

	if s.MaxMemory() != 0 {
		t.Errorf("expected 0 max memory by default, got %d", s.MaxMemory())
	}

	s.SetMaxMemory(8192)
	if s.MaxMemory() != 8192 {
		t.Errorf("expected 8192, got %d", s.MaxMemory())
	}
}

func TestStatsUptime(t *testing.T) {
	s := NewStats()

	// Sleep briefly
	time.Sleep(10 * time.Millisecond)

	uptime := s.Uptime()
	if uptime <= 0 {
		t.Error("expected positive uptime")
	}
	if uptime > time.Second {
		t.Errorf("uptime too large: %v", uptime)
	}
}

func TestStatsSnapshotIsImmutable(t *testing.T) {
	s := NewStats()
	s.Hit()

	snapshot := s.TakeSnapshot()

	// Modify stats after snapshot
	s.Hit()
	s.Hit()
	s.Miss()
	s.Set()

	// Snapshot should be unchanged
	if snapshot.Hits != 1 {
		t.Errorf("snapshot should be immutable, expected 1 hit, got %d", snapshot.Hits)
	}
	if snapshot.Sets != 0 {
		t.Errorf("snapshot should be immutable, expected 0 sets, got %d", snapshot.Sets)
	}
}

func TestStatsAtomicIncrementFromConcurrent(t *testing.T) {
	s := NewStats()
	var wg sync.WaitGroup

	// Concurrent increments of different types
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AddMemory(10)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AddItems(1)
		}()
	}

	wg.Wait()

	if s.Memory() != 1000 {
		t.Errorf("expected 1000 memory, got %d", s.Memory())
	}
	if s.Items() != 100 {
		t.Errorf("expected 100 items, got %d", s.Items())
	}

	// Now concurrent decrements
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SubMemory(5)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SubItems(1)
		}()
	}

	wg.Wait()

	if s.Memory() != 750 {
		t.Errorf("expected 750 memory, got %d", s.Memory())
	}
	if s.Items() != 50 {
		t.Errorf("expected 50 items, got %d", s.Items())
	}
}

// Test that the no-op stats from the runtime package would work.
func TestNoOpStats(t *testing.T) {
	// This tests the interface compatibility
	type collector interface {
		Hit()
		Miss()
		Set()
		Delete()
		Error()
		Eviction()
	}

	s := NewStats()
	var _ collector = s
	_ = s
}
