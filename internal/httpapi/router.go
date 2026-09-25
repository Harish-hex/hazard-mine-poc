package httpapi

import (
	"net/http"

	"hazard-mine-poc/internal/graph"
	"hazard-mine-poc/internal/state"
	"hazard-mine-poc/internal/wsserver"
)

// Deps bundles everything NewRouter needs to wire the full HTTP surface.
type Deps struct {
	Store     *state.Store
	Graph     *graph.Graph
	Telemetry *wsserver.TelemetryHandler
	Dashboard *wsserver.DashboardHandler
}

// NewRouter builds the backend's full HTTP mux: REST endpoints, the
// embedded phone page, and the two WebSocket upgrade routes.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/state", handleState(d.Store))
	mux.HandleFunc("GET /api/graph", handleGraph(d.Graph))
	mux.HandleFunc("GET /phone", handlePhone)

	mux.HandleFunc("POST /api/hazard", handlePostHazard(d.Store))
	mux.HandleFunc("DELETE /api/hazard/{edge_id}", handleDeleteHazard(d.Store))
	mux.HandleFunc("POST /api/override", handlePostOverride(d.Store))

	mux.Handle("GET /ws/telemetry", d.Telemetry)
	mux.Handle("GET /ws/dashboard", d.Dashboard)

	return withCORS(mux)
}
