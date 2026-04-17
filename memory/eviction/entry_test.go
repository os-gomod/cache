package eviction

import (
	"testing"
	"time"
)

func TestNewEntry_BasicFields(t *testing.T) {
	e := NewEntry("key1", []byte("value1"), time.Minute, 0)
	if e.Key != "key1" {
		t.Errorf("Key = %q, want %q", e.Key, "key1")
	}
	if string(e.Value) != "value1" {
		t.Errorf("Value = %q", e.Value)
	}
	if e.Size != int64(len("key1")+len("value1")) {
		t.Errorf("Size = %d", e.Size)
	}
}

func TestNewEntry_WithTTL(t *testing.T) {
	e := NewEntry("key", []byte("val"), 10*time.Minute, 0)
	if e.ExpiresAt == 0 {
		t.Error("ExpiresAt should be set with non-zero TTL")
	}
}

func TestNewEntry_ZeroTTL(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.ExpiresAt != 0 {
		t.Error("ExpiresAt should be 0 for zero TTL")
	}
}

func TestNewEntry_NegativeTTL(t *testing.T) {
	e := NewEntry("key", []byte("val"), -time.Minute, 0)
	if e.ExpiresAt != 0 {
		t.Error("ExpiresAt should be 0 for negative TTL")
	}
}

func TestNewEntry_SoftExpiry(t *testing.T) {
	e := NewEntry("key", []byte("val"), 10*time.Minute, 5*time.Minute)
	if e.SoftExpiresAt == 0 {
		t.Error("SoftExpiresAt should be set")
	}
}

func TestEntry_IsExpired(t *testing.T) {
	e := NewEntry("key", []byte("val"), time.Millisecond, 0)
	time.Sleep(2 * time.Millisecond)
	if !e.IsExpired() {
		t.Error("entry should be expired")
	}
}

func TestEntry_IsExpired_Never(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.IsExpired() {
		t.Error("entry with zero TTL should never expire")
	}
}

func TestEntry_IsExpiredAt(t *testing.T) {
	e := NewEntry("key", []byte("val"), time.Hour, 0)
	now := time.Now().UnixNano()
	if e.IsExpiredAt(now - int64(2*time.Hour)) {
		t.Error("should not be expired at a past time before entry was created")
	}
}

func TestEntry_Touch(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	initialHits := e.GetHits()
	e.Touch()
	if e.GetHits() <= initialHits {
		t.Error("Touch should increment hits")
	}
	if e.GetAccessAt() == 0 {
		t.Error("Touch should update AccessAt")
	}
}

func TestEntry_TTLRemaining(t *testing.T) {
	e := NewEntry("key", []byte("val"), 10*time.Minute, 0)
	remaining := e.TTLRemaining()
	if remaining <= 0 {
		t.Error("TTLRemaining should be positive")
	}
	if remaining > 10*time.Minute {
		t.Error("TTLRemaining should not exceed original TTL")
	}
}

func TestEntry_TTLRemaining_ZeroTTL(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.TTLRemaining() != 0 {
		t.Error("TTLRemaining should be 0 for zero TTL")
	}
}

func TestEntry_Getters(t *testing.T) {
	e := NewEntry("key", []byte("val"), time.Minute, 0)
	if e.GetCreatedAt() == 0 {
		t.Error("CreatedAt should be set")
	}
	if e.GetAccessAt() == 0 {
		t.Error("AccessAt should be set")
	}
	if e.GetHits() < 1 {
		t.Error("Hits should be at least 1 after creation")
	}
}

func TestEntry_Frequency(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.GetFrequency() != 0 {
		t.Error("initial frequency should be 0")
	}
	e.SetFrequency(5)
	if e.GetFrequency() != 5 {
		t.Error("SetFrequency should update frequency")
	}
	e.IncrFrequency()
	if e.GetFrequency() != 6 {
		t.Error("IncrFrequency should increment")
	}
}

func TestEntry_Recency(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	e.SetRecency(10)
	if e.GetRecency() != 10 {
		t.Error("SetRecency should update recency")
	}
}

func TestEntry_RefreshCount(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.GetRefreshCount() != 0 {
		t.Error("initial refresh count should be 0")
	}
	e.IncrRefreshCount()
	if e.GetRefreshCount() != 1 {
		t.Error("IncrRefreshCount should increment")
	}
}

func TestAcquireEntry(t *testing.T) {
	e := AcquireEntry("key", []byte("val"), time.Minute, 0)
	if e == nil {
		t.Fatal("AcquireEntry returned nil")
	}
	if e.Key != "key" {
		t.Errorf("Key = %q", e.Key)
	}
	ReleaseEntry(e)
}

func TestReleaseEntry_Idempotent(t *testing.T) {
	e := AcquireEntry("key", []byte("val"), time.Minute, 0)
	ReleaseEntry(e)
	ReleaseEntry(e) // should not panic
}

func TestEntry_WithNewTTL(t *testing.T) {
	original := NewEntry("key", []byte("val"), time.Minute, 0)
	time.Sleep(time.Millisecond)
	newEntry := original.WithNewTTL(10 * time.Minute)
	if newEntry.Key != original.Key {
		t.Error("WithNewTTL should preserve key")
	}
	if string(newEntry.Value) != string(original.Value) {
		t.Error("WithNewTTL should preserve value")
	}
	if newEntry.ExpiresAt <= original.ExpiresAt {
		t.Error("WithNewTTL should extend expiry")
	}
}

func TestEntry_ShouldEarlyRefresh_ZeroExpiry(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.ShouldEarlyRefresh(1.0) {
		t.Error("zero TTL entries should not trigger early refresh")
	}
}

func TestEntry_HitsCount(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.HitsCount() != e.GetHits() {
		t.Error("HitsCount should match GetHits")
	}
}

func TestEntry_FrequencyCount(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	if e.FrequencyCount() != e.GetFrequency() {
		t.Error("FrequencyCount should match GetFrequency")
	}
}

func TestEntry_IncrHits(t *testing.T) {
	e := NewEntry("key", []byte("val"), 0, 0)
	h := e.GetHits()
	e.IncrHits()
	if e.GetHits() != h+1 {
		t.Error("IncrHits should increment")
	}
}
