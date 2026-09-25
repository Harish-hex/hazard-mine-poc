// Package wsserver hosts the two live WebSocket surfaces: inbound vehicle
// telemetry ingest (telemetry.go) and outbound dashboard state fan-out
// (hub.go, dashboard.go).
package wsserver

import (
	"encoding/json"
	"sync"

	"hazard-mine-poc/internal/state"
)

// subscriberBufferSize is the per-subscriber channel depth. Small on
// purpose: at ~10Hz full-snapshot pushes, a subscriber that's behind by more
// than this is already stale and should just get dropped frames, not build
// up a growing backlog.
const subscriberBufferSize = 16

// Hub is a pub/sub broadcaster: every registered subscriber receives every
// broadcast frame. It satisfies state.Notifier, so a *Hub can be passed
// directly as the Notifier a state.Store broadcasts to. Store always
// releases its own lock before calling Broadcast, so a slow/blocked
// dashboard client can never stall telemetry ingest — Broadcast itself is
// non-blocking per subscriber (drop-oldest-on-full).
type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan []byte]struct{})}
}

// Subscribe registers a new subscriber and returns its receive channel plus
// a cancel function the caller must invoke exactly once when done reading
// (this both unregisters and closes the channel).
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, subscriberBufferSize)

	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, ch)
			h.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

// Broadcast marshals d to JSON and fans it out to every current subscriber.
// Non-blocking: a subscriber whose buffer is full has its oldest queued
// frame dropped to make room for the newest one, so a stalled dashboard
// client never backpressures the broadcaster.
func (h *Hub) Broadcast(d state.DashboardState) {
	b, err := json.Marshal(d)
	if err != nil {
		return
	}
	h.BroadcastRaw(b)
}

// BroadcastRaw fans out a pre-marshaled JSON frame, applying the same
// non-blocking drop-oldest-on-full policy as Broadcast.
func (h *Hub) BroadcastRaw(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs {
		select {
		case ch <- b:
		default:
			// Full: drop the oldest queued frame, then push the newest.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- b:
			default:
			}
		}
	}
}

// SubscriberCount returns the current number of registered subscribers.
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
