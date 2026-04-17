package stats

import "testing"

func TestNewStats(t *testing.T) {
	s := NewStats()
	if s == nil {
		t.Fatal("NewStats returned nil")
	}
	if s.Hits() != 0 {
		t.Errorf("new stats should have 0 hits, got %d", s.Hits())
	}
	if s.Misses() != 0 {
		t.Errorf("new stats should have 0 misses, got %d", s.Misses())
	}
}

func TestStats_HitMiss(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Hit()
	s.Hit()
	s.Miss()
	s.Miss()
	if s.Hits() != 3 {
		t.Errorf("expected 3 hits, got %d", s.Hits())
	}
	if s.Misses() != 2 {
		t.Errorf("expected 2 misses, got %d", s.Misses())
	}
	rate := s.HitRate()
	if rate < 59 || rate > 61 {
		t.Errorf("expected ~60%% hit rate, got %.2f%%", rate)
	}
}

func TestStats_L1L2(t *testing.T) {
	s := NewStats()
	s.L1Hit()
	s.L1Hit()
	s.L1Miss()
	s.L2Hit()
	s.L2Miss()
	s.L2Error()
	s.L2Promotion()
	if s.L1Hits() != 2 {
		t.Errorf("expected 2 L1 hits, got %d", s.L1Hits())
	}
	// L1Hit also increments overall hits
	if s.Hits() != 2 {
		t.Errorf("expected 2 total hits, got %d", s.Hits())
	}
	if s.L1Misses() != 1 {
		t.Errorf("expected 1 L1 miss, got %d", s.L1Misses())
	}
	if s.L1Errors() != 0 {
		t.Errorf("expected 0 L1 errors, got %d", s.L1Errors())
	}
	if s.L2Errors() != 1 {
		t.Errorf("expected 1 L2 error, got %d", s.L2Errors())
	}
	if s.L2Errors() != 1 {
		t.Errorf("expected 1 total error, got %d", s.Errors())
	}
}

func TestStats_SetOpDeleteOp(t *testing.T) {
	s := NewStats()
	s.SetOp()
	s.SetOp()
	s.DeleteOp()
	if s.Sets() != 2 {
		t.Errorf("expected 2 sets, got %d", s.Sets())
	}
	if s.Deletes() != 1 {
		t.Errorf("expected 1 delete, got %d", s.Deletes())
	}
}

func TestStats_Eviction(t *testing.T) {
	s := NewStats()
	s.EvictionOp()
	s.EvictionOp()
	if s.Evictions() != 2 {
		t.Errorf("expected 2 evictions, got %d", s.Evictions())
	}
}

func TestStats_WriteBack(t *testing.T) {
	s := NewStats()
	s.WriteBackEnqueue()
	s.WriteBackEnqueue()
	s.WriteBackFlush()
	s.WriteBackDrop()
	if s.WriteBackEnqueued() != 2 {
		t.Errorf("expected 2 enqueued, got %d", s.WriteBackEnqueued())
	}
	if s.WriteBackFlushed() != 1 {
		t.Errorf("expected 1 flushed, got %d", s.WriteBackFlushed())
	}
	if s.WriteBackDropped() != 1 {
		t.Errorf("expected 1 dropped, got %d", s.WriteBackDropped())
	}
}

func TestStats_TakeSnapshot(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Miss()
	s.SetOp()
	snap := s.TakeSnapshot()
	if snap.Hits != 1 {
		t.Errorf("snapshot hits = %d, want 1", snap.Hits)
	}
	if snap.Misses != 1 {
		t.Errorf("snapshot misses = %d, want 1", snap.Misses)
	}
	if snap.Sets != 1 {
		t.Errorf("snapshot sets = %d, want 1", snap.Sets)
	}
}

func TestStats_Snapshot_Alias(t *testing.T) {
	s := NewStats()
	s.Hit()
	snap := s.Snapshot()
	if snap.Hits != 1 {
		t.Errorf("Snapshot() hits = %d, want 1", snap.Hits)
	}
}

func TestStats_Reset(t *testing.T) {
	s := NewStats()
	s.Hit()
	s.Hit()
	s.Miss()
	s.SetOp()
	s.Reset()
	if s.Hits() != 0 {
		t.Errorf("after reset, hits = %d, want 0", s.Hits())
	}
	if s.Misses() != 0 {
		t.Errorf("after reset, misses = %d, want 0", s.Misses())
	}
	if s.Sets() != 0 {
		t.Errorf("after reset, sets = %d, want 0", s.Sets())
	}
}

func TestStats_HitRate_NoOps(t *testing.T) {
	s := NewStats()
	if rate := s.HitRate(); rate != 0 {
		t.Errorf("expected 0%% hit rate with no ops, got %.2f%%", rate)
	}
}

func TestStats_OpsPerSecond(t *testing.T) {
	s := NewStats()
	s.RecordGet()
	s.RecordGet()
	ops := s.OpsPerSecond()
	if ops <= 0 {
		t.Errorf("expected positive ops/sec, got %.2f", ops)
	}
}

func TestStats_Uptime(t *testing.T) {
	s := NewStats()
	up := s.Uptime()
	if up <= 0 {
		t.Error("uptime should be positive")
	}
}

func TestStats_AddItemsMemory(t *testing.T) {
	s := NewStats()
	s.AddItems(100)
	s.AddMemory(4096)
	if s.Items() != 100 {
		t.Errorf("expected 100 items, got %d", s.Items())
	}
	if s.Memory() != 4096 {
		t.Errorf("expected 4096 memory, got %d", s.Memory())
	}
}

func TestHitRate(t *testing.T) {
	tests := []struct {
		hits, misses int64
		want         float64
	}{
		{100, 0, 100},
		{0, 100, 0},
		{50, 50, 50},
		{1, 3, 25},
		{0, 0, 0},
	}
	for _, tt := range tests {
		got := hitRate(tt.hits, tt.misses)
		if got != tt.want {
			t.Errorf("hitRate(%d, %d) = %.2f, want %.2f", tt.hits, tt.misses, got, tt.want)
		}
	}
}
