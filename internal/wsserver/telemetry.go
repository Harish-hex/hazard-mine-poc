package wsserver

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"hazard-mine-poc/internal/odometry"
)

var telemetryUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// TelemetryApplier is the subset of *state.Store the telemetry handler
// needs — kept as a narrow interface so this package doesn't have to depend
// on the concrete Store type for its read path.
type TelemetryApplier interface {
	ApplyTelemetry(odometry.TelemetrySample)
}

// TelemetryHandler serves GET /ws/telemetry. Single-cart reconnect
// tolerance: it keeps at most one active connection; a new upgrade evicts
// the prior one. Any read/decode error closes just that connection — the
// server keeps accepting new upgrades and never crashes.
type TelemetryHandler struct {
	store TelemetryApplier

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewTelemetryHandler builds a TelemetryHandler that feeds decoded samples
// into store.
func NewTelemetryHandler(store TelemetryApplier) *TelemetryHandler {
	return &TelemetryHandler{store: store}
}

func (h *TelemetryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := telemetryUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("wsserver: telemetry upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	prev := h.conn
	h.conn = conn
	h.mu.Unlock()

	if prev != nil {
		log.Printf("wsserver: telemetry: evicting prior connection for new client %s", r.RemoteAddr)
		_ = prev.Close()
	}

	log.Printf("wsserver: telemetry client connected from %s", r.RemoteAddr)

	for {
		var sample odometry.TelemetrySample
		if err := conn.ReadJSON(&sample); err != nil {
			log.Printf("wsserver: telemetry read error, closing connection: %v", err)
			break
		}
		h.store.ApplyTelemetry(sample)
	}

	_ = conn.Close()

	h.mu.Lock()
	if h.conn == conn {
		h.conn = nil
	}
	h.mu.Unlock()
}
