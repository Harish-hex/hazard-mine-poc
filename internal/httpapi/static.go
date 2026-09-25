// Package httpapi builds the backend's HTTP surface: REST endpoints, the
// embedded phone page, and (wired in via router.go) the two WebSocket
// upgrade routes served by internal/wsserver.
package httpapi

import (
	"encoding/json"
	"net/http"

	"hazard-mine-poc/internal/graph"
	"hazard-mine-poc/internal/state"
	"hazard-mine-poc/web"
)

// handleHealthz serves GET /healthz — a trivial liveness check.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleState serves GET /api/state — a non-WS snapshot, same shape as the
// dashboard WS push, useful for curl-based checks.
func handleState(store *state.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.Snapshot())
	}
}

// graphResponse is GET /api/graph's response shape: the static topology
// (nodes with name/coords, edges with weight), fetched once by the
// dashboard on load. Live WS pushes then only reference IDs against this
// already-fetched map.
type graphResponse struct {
	Nodes []nodeResponse `json:"nodes"`
	Edges []edgeResponse `json:"edges"`
}

type nodeResponse struct {
	ID   graph.NodeID `json:"id"`
	Name string       `json:"name"`
	X    float64      `json:"x"`
	Y    float64      `json:"y"`
}

type edgeResponse struct {
	ID      graph.EdgeID `json:"id"`
	A       graph.NodeID `json:"a"`
	B       graph.NodeID `json:"b"`
	WeightM float64      `json:"weight_m"`
}

// handleGraph serves GET /api/graph.
func handleGraph(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes := g.Nodes()
		edges := g.Edges()

		resp := graphResponse{
			Nodes: make([]nodeResponse, len(nodes)),
			Edges: make([]edgeResponse, len(edges)),
		}
		for i, n := range nodes {
			resp.Nodes[i] = nodeResponse{ID: n.ID, Name: n.Name, X: n.X, Y: n.Y}
		}
		for i, e := range edges {
			resp.Edges[i] = edgeResponse{ID: e.ID, A: e.A, B: e.B, WeightM: e.Weight}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// handlePhone serves GET /phone — the embedded one-button hazard report
// page (hazard type + button → fetch() POST to /api/hazard).
func handlePhone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.PhoneHTML)
}

// writeJSON writes v as a JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a small {"error": msg} JSON body with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
