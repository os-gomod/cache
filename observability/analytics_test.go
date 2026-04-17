package observability

import (
	"testing"
	"time"
)

func TestNewHitRateWindow(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	if w == nil {
		t.Fatal("NewHitRateWindow returned nil")
	}
}

func TestNewHitRateWindow_ZeroDefaults(t *testing.T) {
	w := NewHitRateWindow(0, 0)
	if w == nil {
		t.Fatal("NewHitRateWindow returned nil")
	}
}

func TestHitRate_NoData(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	if w.HitRate() != 0.0 {
		t.Errorf("HitRate() = %f, want 0.0", w.HitRate())
	}
}

func TestRecord_HitAndMiss(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	w.Record(true)
	w.Record(true)
	w.Record(false)
	w.Record(true)
	rate := w.HitRate()
	if rate < 0.7 || rate > 0.76 {
		t.Errorf("HitRate() = %f, want ~0.75", rate)
	}
}

func TestAdvance(t *testing.T) {
	w := NewHitRateWindow(time.Second, 5)
	w.Record(true)
	w.Record(true)
	// Bucket 0 has 2 hits.
	w.Advance()
	// current moves to 1; bucket 1 is reset (cleared).
	// Bucket 0 still has 2 hits.
	w.Record(false)
	w.Record(true)
	// Bucket 1 now has 1 hit + 1 miss. Total: 3 hits, 1 miss = 0.75.
	rate := w.HitRate()
	if rate < 0.7 || rate > 0.8 {
		t.Errorf("HitRate after advance = %f, want ~0.75", rate)
	}
}

func TestP50Latency_NoData(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	if w.P50Latency() != 0 {
		t.Error("P50 should be 0 when no data")
	}
	if w.P99Latency() != 0 {
		t.Error("P99 should be 0 when no data")
	}
}

func TestP50Latency_SingleSample(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	w.RecordLatency(5 * time.Millisecond)
	if w.P50Latency() != 5*time.Millisecond {
		t.Errorf("P50 = %v, want 5ms", w.P50Latency())
	}
}

func TestP99Latency_MultipleSamples(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	for i := 0; i < 100; i++ {
		w.RecordLatency(time.Duration(i) * time.Millisecond)
	}
	p50 := w.P50Latency()
	p99 := w.P99Latency()
	if p50 < 48*time.Millisecond || p50 > 52*time.Millisecond {
		t.Errorf("P50 = %v, want ~50ms", p50)
	}
	if p99 < 97*time.Millisecond {
		t.Errorf("P99 = %v, want ~99ms", p99)
	}
}

func TestRecordLatency_Trimming(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	for i := 0; i < 15000; i++ {
		w.RecordLatency(time.Duration(i) * time.Microsecond)
	}
	// Should trim to maxLat (10000)
	if w.P50Latency() == 0 {
		t.Error("P50 should not be 0 after recording many samples")
	}
}

func TestReset(t *testing.T) {
	w := NewHitRateWindow(time.Second, 10)
	w.Record(true)
	w.Record(false)
	w.RecordLatency(5 * time.Millisecond)
	w.Reset()
	if w.HitRate() != 0.0 {
		t.Error("HitRate should be 0 after reset")
	}
	if w.P50Latency() != 0 {
		t.Error("P50 should be 0 after reset")
	}
}
