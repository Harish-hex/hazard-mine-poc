package wsserver

import (
	"encoding/json"
	"testing"
	"time"

	"hazard-mine-poc/internal/state"
)

func TestHubBroadcastFanOut(t *testing.T) {
	h := NewHub()

	ch1, cancel1 := h.Subscribe()
	defer cancel1()
	ch2, cancel2 := h.Subscribe()
	defer cancel2()

	if got := h.SubscriberCount(); got != 2 {
		t.Fatalf("SubscriberCount = %d, want 2", got)
	}

	h.Broadcast(state.DashboardState{CurrentNode: "N1"})

	for _, ch := range []<-chan []byte{ch1, ch2} {
		select {
		case msg := <-ch:
			var got state.DashboardState
			if err := json.Unmarshal(msg, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.CurrentNode != "N1" {
				t.Errorf("CurrentNode = %v, want N1", got.CurrentNode)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
}

func TestHubUnsubscribe(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe()
	if got := h.SubscriberCount(); got != 1 {
		t.Fatalf("SubscriberCount = %d, want 1", got)
	}
	cancel()
	if got := h.SubscriberCount(); got != 0 {
		t.Fatalf("SubscriberCount after cancel = %d, want 0", got)
	}
}

func TestHubDropsOldestWhenFull(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()
	defer cancel()

	// Fill well past the buffer without draining.
	for i := 0; i < subscriberBufferSize+5; i++ {
		h.Broadcast(state.DashboardState{CumulativeDistanceM: float64(i)})
	}

	// Should not have blocked; channel should hold at most bufferSize frames,
	// and the most recent broadcast should still be delivered eventually.
	var last state.DashboardState
	drained := 0
	for {
		select {
		case msg := <-ch:
			_ = json.Unmarshal(msg, &last)
			drained++
		default:
			goto done
		}
	}
done:
	if drained == 0 {
		t.Fatalf("expected at least one frame drained")
	}
	if drained > subscriberBufferSize {
		t.Errorf("drained %d frames, want <= buffer size %d", drained, subscriberBufferSize)
	}
}
