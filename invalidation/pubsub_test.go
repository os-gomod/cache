package invalidation

import "testing"

func TestEventKinds(t *testing.T) {
	kinds := []EventKind{
		KindExpire, KindEvict, KindDelete,
		KindInvalidate, KindClear,
	}
	for _, k := range kinds {
		if string(k) == "" {
			t.Errorf("event kind %v has empty string", k)
		}
	}
}

func TestHandlerFunc(t *testing.T) {
	var received Event
	h := HandlerFunc(func(evt Event) {
		received = evt
	})
	h.OnEvent(Event{Kind: KindDelete, Key: "k1"})
	if received.Kind != KindDelete {
		t.Errorf("expected KindDelete, got %v", received.Kind)
	}
	if received.Key != "k1" {
		t.Errorf("expected key k1, got %s", received.Key)
	}
}

func TestLocalBus_PublishSubscribe(t *testing.T) {
	bus := NewLocalBus()
	defer bus.Close()

	var received Event
	unsub := bus.Subscribe(HandlerFunc(func(evt Event) {
		received = evt
	}))

	bus.Publish(Event{Kind: KindExpire, Key: "test-key"})
	if received.Kind != KindExpire {
		t.Errorf("expected KindExpire, got %v", received.Kind)
	}
	if received.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
	unsub()

	// After unsubscribe, should not receive
	received = Event{}
	bus.Publish(Event{Kind: KindDelete, Key: "other"})
	if received.Kind != "" {
		t.Error("should not receive after unsubscribe")
	}
}

func TestLocalBus_MultipleSubscribers(t *testing.T) {
	bus := NewLocalBus()
	defer bus.Close()

	count := 0
	for range 5 {
		bus.Subscribe(HandlerFunc(func(evt Event) {
			count++
		}))
	}
	bus.Publish(Event{Kind: KindClear})
	if count != 5 {
		t.Errorf("expected 5 handlers called, got %d", count)
	}
}

func TestLocalBus_Close(t *testing.T) {
	bus := NewLocalBus()
	bus.Close()

	// Subscribe after close should return noop
	unsub := bus.Subscribe(HandlerFunc(func(evt Event) {}))
	unsub() // should not panic

	bus.Publish(Event{Kind: KindDelete}) // should not panic
}

func TestLocalBus_PublishPanicsHandled(t *testing.T) {
	bus := NewLocalBus()
	defer bus.Close()

	bus.Subscribe(HandlerFunc(func(evt Event) {
		panic("test panic")
	}))

	// Should not panic - handler panic is recovered
	bus.Publish(Event{Kind: KindDelete})
}
