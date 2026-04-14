package stats

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestStats_HitMiss(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Hit()
	s.Miss()
	if s.Hits() != 2 {
		t.Errorf("Hits = %d, want 2", s.Hits())
	}
	if s.Misses() != 1 {
		t.Errorf("Misses = %d, want 1", s.Misses())
	}
}

func TestStats_HitRate(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Hit()
	s.Miss()
	if rate := s.HitRate(); rate != 66.66666666666666 {
		t.Errorf("HitRate = %v, want ~66.67", rate)
	}
}

func TestStats_Reset(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.SetOp()
	s.Reset()
	if s.Hits() != 0 {
		t.Error("Hits should be 0 after Reset")
	}
	if s.Sets() != 0 {
		t.Error("Sets should be 0 after Reset")
	}
}

func TestStats_Snapshot(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.SetOp()
	snap := s.TakeSnapshot()
	if snap.Hits != 1 {
		t.Errorf("Snapshot Hits = %d, want 1", snap.Hits)
	}
	if snap.Sets != 1 {
		t.Errorf("Snapshot Sets = %d, want 1", snap.Sets)
	}
}

func TestStats_ImplementsRecorder(t *testing.T) {
	var _ Recorder = NewStats()
}

func BenchmarkStats_ParallelHitMiss(b *testing.B) {
	s := NewStats()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				s.Hit()
			} else {
				s.Miss()
			}
			i++
		}
	})
}

func TestStats_ConcurrentOperations(t *testing.T) {
	s := NewStats()
	var wg sync.WaitGroup
	const goroutines = 100
	const opsPerGoroutine = 1000

	var totalHits atomic.Int64
	var totalMisses atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if j%2 == 0 {
					s.Hit()
					totalHits.Add(1)
				} else {
					s.Miss()
					totalMisses.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if s.Hits() != totalHits.Load() {
		t.Errorf("Hits = %d, want %d", s.Hits(), totalHits.Load())
	}
	if s.Misses() != totalMisses.Load() {
		t.Errorf("Misses = %d, want %d", s.Misses(), totalMisses.Load())
	}
}
