package invalidation

import (
	"sync"
	"testing"
	"time"
)

func TestEventKind_Constants(t *testing.T) {
	tests := []struct {
		kind EventKind
		want string
	}{
		{KindExpire, "expire"},
		{KindEvict, "evict"},
		{KindDelete, "delete"},
		{KindInvalidate, "invalidate"},
		{KindClear, "clear"},
	}
	for _, tt := range tests {
		if string(tt.kind) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, string(tt.kind))
		}
	}
}

func TestEvent_ZeroTimestamp(t *testing.T) {
	evt := Event{Kind: KindExpire, Key: "k"}
	if !evt.Timestamp.IsZero() {
		t.Error("expected zero timestamp")
	}
}

func TestHandlerFunc_Called(t *testing.T) {
	var gotEvt Event
	h := HandlerFunc(func(evt Event) {
		gotEvt = evt
	})
	wantEvt := Event{Kind: KindDelete, Key: "mykey"}
	h.OnEvent(wantEvt)
	if gotEvt.Kind != wantEvt.Kind || gotEvt.Key != wantEvt.Key {
		t.Errorf("got %+v, want %+v", gotEvt, wantEvt)
	}
}

func TestNewLocalBus(t *testing.T) {
	bus := NewLocalBus()
	if bus == nil {
		t.Fatal("expected non-nil bus")
	}
}

func TestLocalBus_Publish_Subscribe(t *testing.T) {
	bus := NewLocalBus()

	var gotEvt Event
	var mu sync.Mutex
	h := HandlerFunc(func(evt Event) {
		mu.Lock()
		gotEvt = evt
		mu.Unlock()
	})

	bus.Subscribe(h)

	bus.Publish(Event{Kind: KindEvict, Key: "k1", Backend: "memory"})

	mu.Lock()
	defer mu.Unlock()
	if gotEvt.Kind != KindEvict {
		t.Errorf("expected KindEvict, got %s", gotEvt.Kind)
	}
	if gotEvt.Key != "k1" {
		t.Errorf("expected key k1, got %s", gotEvt.Key)
	}
	if gotEvt.Backend != "memory" {
		t.Errorf("expected backend memory, got %s", gotEvt.Backend)
	}
}

func TestLocalBus_Unsubscribe(t *testing.T) {
	bus := NewLocalBus()

	callCount := 0
	h := HandlerFunc(func(evt Event) {
		callCount++
	})

	unsub := bus.Subscribe(h)

	bus.Publish(Event{Kind: KindExpire, Key: "k"})
	if callCount != 1 {
		t.Fatalf("expected 1 call before unsubscribe, got %d", callCount)
	}

	unsub()

	bus.Publish(Event{Kind: KindExpire, Key: "k"})
	if callCount != 1 {
		t.Fatalf("expected still 1 call after unsubscribe, got %d", callCount)
	}
}

func TestLocalBus_Publish_AutoTimestamp(t *testing.T) {
	bus := NewLocalBus()

	var gotEvt Event
	var mu sync.Mutex
	h := HandlerFunc(func(evt Event) {
		mu.Lock()
		gotEvt = evt
		mu.Unlock()
	})
	bus.Subscribe(h)

	before := time.Now()
	bus.Publish(Event{Kind: KindDelete, Key: "k"})
	after := time.Now()

	mu.Lock()
	defer mu.Unlock()
	if gotEvt.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
	if gotEvt.Timestamp.Before(before) || gotEvt.Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", gotEvt.Timestamp, before, after)
	}
}

func TestLocalBus_Publish_HandlerPanics(t *testing.T) {
	bus := NewLocalBus()

	// First handler panics.
	bus.Subscribe(HandlerFunc(func(evt Event) {
		panic("test panic")
	}))

	// Second handler should still be called.
	var secondCalled bool
	bus.Subscribe(HandlerFunc(func(evt Event) {
		secondCalled = true
	}))

	// Publish should not panic.
	bus.Publish(Event{Kind: KindExpire, Key: "k"})

	if !secondCalled {
		t.Error("expected second handler to be called even after first panicked")
	}
}

func TestLocalBus_Close_NoPanic(t *testing.T) {
	bus := NewLocalBus()
	bus.Subscribe(HandlerFunc(func(evt Event) {}))
	bus.Close()
	bus.Close() // double close should not panic
}

func TestLocalBus_SubscribeAfterClose(t *testing.T) {
	bus := NewLocalBus()
	bus.Close()

	called := false
	unsub := bus.Subscribe(HandlerFunc(func(evt Event) {
		called = true
	}))

	// Unsubscribe should be a no-op and not panic.
	unsub()

	bus.Publish(Event{Kind: KindExpire, Key: "k"})
	if called {
		t.Error("handler should not be called after bus is closed")
	}
}

func TestLocalBus_MultipleHandlers(t *testing.T) {
	bus := NewLocalBus()

	var count int
	var mu sync.Mutex
	h := HandlerFunc(func(evt Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Subscribe(h)
	bus.Subscribe(h)
	bus.Subscribe(h)

	bus.Publish(Event{Kind: KindClear})

	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 handler calls, got %d", count)
	}
}
