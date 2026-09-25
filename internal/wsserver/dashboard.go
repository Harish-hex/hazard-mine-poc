package wsserver

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var dashboardUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const writeDeadline = 5 * time.Second

// DashboardHandler serves GET /ws/dashboard: each connection subscribes to
// the Hub and receives a full DashboardState JSON frame on every broadcast
// (full snapshot per push, no throttling/diffing).
type DashboardHandler struct {
	hub *Hub
}

// NewDashboardHandler builds a DashboardHandler fed by hub.
func NewDashboardHandler(hub *Hub) *DashboardHandler {
	return &DashboardHandler{hub: hub}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := dashboardUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("wsserver: dashboard upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	frames, cancel := h.hub.Subscribe()
	defer cancel()

	log.Printf("wsserver: dashboard client connected from %s", r.RemoteAddr)

	// The dashboard WS is outbound-only (no inbound message schema), but we
	// still need to drain reads so close/ping control frames are handled
	// and a client disconnect is noticed.
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	for frame := range frames {
		_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
		if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			log.Printf("wsserver: dashboard write error, closing connection: %v", err)
			return
		}
	}
}
