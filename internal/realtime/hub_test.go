package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/packages/pkg/events"
)

func TestBroadcastOnlyReachesSameTenantSubscribers(t *testing.T) {
	h := NewHub("test-secret")
	tenantA := uuid.New()
	tenantB := uuid.New()

	chA := h.subscribe(tenantA)
	chB := h.subscribe(tenantB)
	defer h.unsubscribe(tenantA, chA)
	defer h.unsubscribe(tenantB, chB)

	h.Broadcast(events.New("repair.status_changed", tenantA, uuid.New(), map[string]any{"status": "completed"}))

	select {
	case msg := <-chA:
		var decoded struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(msg, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.EventType != "repair.status_changed" {
			t.Fatalf("unexpected event type: %s", decoded.EventType)
		}
	case <-time.After(time.Second):
		t.Fatal("expected tenant A subscriber to receive the event")
	}

	select {
	case <-chB:
		t.Fatal("tenant B subscriber should not receive tenant A's event")
	case <-time.After(50 * time.Millisecond):
		// expected: nothing arrives
	}
}

func TestSlowSubscriberDoesNotBlockBroadcast(t *testing.T) {
	h := NewHub("test-secret")
	tenantID := uuid.New()
	ch := h.subscribe(tenantID)
	defer h.unsubscribe(tenantID, ch)

	// Fill the buffer past capacity; Broadcast must not block even though
	// nobody is draining the channel.
	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberBuffer+10; i++ {
			h.Broadcast(events.New("noop", tenantID, uuid.New(), map[string]any{"i": i}))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on a full subscriber channel")
	}
}

func TestSubscriberCount(t *testing.T) {
	h := NewHub("test-secret")
	tenantID := uuid.New()
	if h.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers initially")
	}
	ch := h.subscribe(tenantID)
	if h.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber after subscribe")
	}
	h.unsubscribe(tenantID, ch)
	if h.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe")
	}
}
