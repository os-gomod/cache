package eviction

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestEntryPool(t *testing.T) {
	// Test acquiring and releasing entries
	e1 := AcquireEntry("key1", []byte("value1"), time.Minute)
	if e1.Key != "key1" {
		t.Errorf("Expected key 'key1', got '%s'", e1.Key)
	}
	if string(e1.Value) != "value1" {
		t.Errorf("Expected value 'value1', got '%s'", string(e1.Value))
	}
	if e1.Size != int64(len("key1")+len("value1")) {
		t.Errorf("Expected size %d, got %d", len("key1")+len("value1"), e1.Size)
	}
	if e1.HitsCount() != 1 {
		t.Errorf("Expected hits 1, got %d", e1.HitsCount())
	}

	ReleaseEntry(e1)

	// Acquire another entry - should reuse the pooled entry
	e2 := AcquireEntry("key2", []byte("value2"), time.Minute)
	if e2.Key != "key2" {
		t.Errorf("Expected key 'key2', got '%s'", e2.Key)
	}
	if string(e2.Value) != "value2" {
		t.Errorf("Expected value 'value2', got '%s'", string(e2.Value))
	}

	ReleaseEntry(e2)
}

func TestNewEntry(t *testing.T) {
	e := NewEntry("test", []byte("data"), time.Hour)

	if e.Key != "test" {
		t.Errorf("Expected key 'test', got '%s'", e.Key)
	}
	if string(e.Value) != "data" {
		t.Errorf("Expected value 'data', got '%s'", string(e.Value))
	}
	if e.Size != int64(len("test")+len("data")) {
		t.Errorf("Expected size %d, got %d", len("test")+len("data"), e.Size)
	}
	if e.HitsCount() != 1 {
		t.Errorf("Expected hits 1, got %d", e.HitsCount())
	}
	if e.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if e.AccessAt == 0 {
		t.Error("AccessAt should be set")
	}
}

func TestEntryExpiration(t *testing.T) {
	// Test with no TTL
	e1 := NewEntry("key1", []byte("value"), 0)
	if e1.IsExpired() {
		t.Error("Entry with no TTL should not be expired")
	}
	if e1.TTLRemaining() != 0 {
		t.Errorf("Expected TTL 0, got %v", e1.TTLRemaining())
	}

	// Test with TTL
	e2 := NewEntry("key2", []byte("value"), time.Millisecond*100)
	if e2.IsExpired() {
		t.Error("New entry should not be expired")
	}

	time.Sleep(time.Millisecond * 150)
	if !e2.IsExpired() {
		t.Error("Entry should be expired after TTL")
	}

	if e2.TTLRemaining() > 0 {
		t.Errorf("TTL should be <= 0, got %v", e2.TTLRemaining())
	}
}

// func TestEntryExpirationAt(t *testing.T) {
// 	e := NewEntry("key", []byte("value"), time.Hour)
// 	now := atomic.LoadInt64(&clock.NowAtomic)

// 	// Should not be expired at current time
// 	if e.IsExpiredAt(now) {
// 		t.Error("Entry should not be expired at current time")
// 	}

// 	// Should be expired in the future
// 	future := now + time.Hour.Nanoseconds() + 1
// 	if !e.IsExpiredAt(future) {
// 		t.Error("Entry should be expired at future time")
// 	}

// 	// No TTL
// 	e2 := NewEntry("key2", []byte("value"), 0)
// 	if e2.IsExpiredAt(now) {
// 		t.Error("Entry with no TTL should never expire")
// 	}
// }

func TestEntryTouch(t *testing.T) {
	e := NewEntry("key", []byte("value"), time.Hour)
	originalAccess := e.GetAccessAt()
	originalHits := e.HitsCount()

	time.Sleep(time.Millisecond * 10)
	e.Touch()

	if e.GetAccessAt() <= originalAccess {
		t.Error("AccessAt should be updated")
	}
	if e.HitsCount() != originalHits+1 {
		t.Errorf("Expected hits %d, got %d", originalHits+1, e.HitsCount())
	}
}

func TestEntryWithNewTTL(t *testing.T) {
	e := NewEntry("key", []byte("value"), time.Hour)
	originalCreated := e.GetCreatedAt()
	originalHits := e.HitsCount()

	// Create copy with new TTL
	e2 := e.WithNewTTL(time.Minute * 30)

	if e2.Key != e.Key {
		t.Error("Key should be copied")
	}
	if !bytes.Equal(e2.Value, e.Value) {
		t.Error("Value should be shared")
	}
	if e2.Size != e.Size {
		t.Error("Size should be copied")
	}
	if e2.CreatedAt != originalCreated {
		t.Error("CreatedAt should be preserved")
	}
	if e2.Hits != originalHits {
		t.Error("Hits should be preserved")
	}
	if e2.ExpiresAt == e.ExpiresAt {
		t.Error("ExpiresAt should be updated")
	}

	// Original should be unchanged
	if e.GetExpiresAt() == e2.ExpiresAt {
		t.Error("Original entry should not be modified")
	}
}

func TestEntryAtomicOperations(t *testing.T) {
	e := NewEntry("key", []byte("value"), time.Hour)

	// Test increment operations
	initialFreq := e.FrequencyCount()
	newFreq := e.IncrFrequency()
	if newFreq != initialFreq+1 {
		t.Errorf("Expected frequency %d, got %d", initialFreq+1, newFreq)
	}

	// Test concurrent increments
	var wg sync.WaitGroup
	concurrency := 100
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.IncrFrequency()
		}()
	}
	wg.Wait()

	expected := initialFreq + 1 + int64(concurrency)
	if e.FrequencyCount() != expected {
		t.Errorf("Expected frequency %d, got %d", expected, e.FrequencyCount())
	}
}

func TestEntryReset(t *testing.T) {
	e := AcquireEntry("key", []byte("value"), time.Hour)
	e.Touch()
	e.IncrFrequency()

	ReleaseEntry(e)

	// Reacquire and verify fields are reset
	e2 := AcquireEntry("newkey", []byte("newvalue"), time.Minute)
	if e2.Key != "newkey" {
		t.Errorf("Expected key 'newkey', got '%s'", e2.Key)
	}
	if string(e2.Value) != "newvalue" {
		t.Errorf("Expected value 'newvalue', got '%s'", string(e2.Value))
	}
	if e2.HitsCount() != 1 {
		t.Errorf("Expected hits 1, got %d", e2.HitsCount())
	}
	if e2.FrequencyCount() != 0 {
		t.Errorf("Expected frequency 0, got %d", e2.FrequencyCount())
	}

	ReleaseEntry(e2)
}

func TestEntryTTLRemaining(t *testing.T) {
	// Test with TTL
	e1 := NewEntry("key1", []byte("value"), 200*time.Millisecond)
	remaining := e1.TTLRemaining()
	if remaining <= 0 || remaining > 200*time.Millisecond {
		t.Errorf("TTL remaining should be between 0 and 200ms, got %v", remaining)
	}

	time.Sleep(120 * time.Millisecond)
	remaining = e1.TTLRemaining()
	if remaining > 100*time.Millisecond {
		t.Errorf("TTL remaining should be less than 100ms, got %v", remaining)
	}

	// Test without TTL
	e2 := NewEntry("key2", []byte("value"), 0)
	if e2.TTLRemaining() != 0 {
		t.Errorf("Expected TTL 0, got %v", e2.TTLRemaining())
	}
}

func TestEntryGetAccessors(t *testing.T) {
	e := NewEntry("key", []byte("value"), time.Hour)

	// Test all getters
	accessAt := e.GetAccessAt()
	createdAt := e.GetCreatedAt()
	expiresAt := e.GetExpiresAt()
	hits := e.GetHits()
	freq := e.GetFrequency()
	recency := e.GetRecency()

	if accessAt == 0 {
		t.Error("GetAccessAt should return non-zero")
	}
	if createdAt == 0 {
		t.Error("GetCreatedAt should return non-zero")
	}
	if expiresAt == 0 {
		t.Error("GetExpiresAt should return non-zero")
	}
	if hits != 1 {
		t.Errorf("Expected hits 1, got %d", hits)
	}
	if freq != 0 {
		t.Errorf("Expected frequency 0, got %d", freq)
	}
	if recency != 0 {
		t.Errorf("Expected recency 0, got %d", recency)
	}
}

func TestEntrySetAccessors(t *testing.T) {
	e := NewEntry("key", []byte("value"), time.Hour)

	// Test setters
	e.SetFrequency(42)
	if e.FrequencyCount() != 42 {
		t.Errorf("Expected frequency 42, got %d", e.FrequencyCount())
	}

	e.SetRecency(99)
	if e.GetRecency() != 99 {
		t.Errorf("Expected recency 99, got %d", e.GetRecency())
	}

	e.IncrHits()
	if e.HitsCount() != 2 {
		t.Errorf("Expected hits 2, got %d", e.HitsCount())
	}
}

func BenchmarkEntryCreation(b *testing.B) {
	b.Run("NewEntry", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := NewEntry("key", []byte("value"), time.Minute)
			_ = e
		}
	})

	b.Run("AcquireEntry", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := AcquireEntry("key", []byte("value"), time.Minute)
			ReleaseEntry(e)
		}
	})
}

func BenchmarkEntryOperations(b *testing.B) {
	e := NewEntry("key", []byte("value"), time.Hour)

	b.Run("Touch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e.Touch()
		}
	})

	b.Run("IsExpired", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.IsExpired()
		}
	})

	b.Run("IncrFrequency", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e.IncrFrequency()
		}
	})

	b.Run("TTLRemaining", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.TTLRemaining()
		}
	})
}
