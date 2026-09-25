package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"hazard-mine-poc/internal/state"
)

// hazardRequest is POST /api/hazard's body. It carries no edge_id — a
// hazard always blocks the cart's *current* edge, never an arbitrary
// caller-picked one.
type hazardRequest struct {
	Type     string `json:"type"`
	Severity string `json:"severity,omitempty"`
}

type hazardBlockedResponse struct {
	BlockedEdge  string `json:"blocked_edge"`
	RouteChanged bool   `json:"route_changed"`
}

type hazardUnblockedResponse struct {
	UnblockedEdge string `json:"unblocked_edge"`
	RouteChanged  bool   `json:"route_changed"`
}

// handlePostHazard serves POST /api/hazard (phone → backend).
func handlePostHazard(store *state.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req hazardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Type == "" {
			writeError(w, http.StatusBadRequest, "type is required")
			return
		}

		edgeID, routeChanged, err := store.ReportHazard(state.SourcePhone, state.HazardEvent{
			Type:     req.Type,
			Severity: req.Severity,
		})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, state.ErrArrived) {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, hazardBlockedResponse{
			BlockedEdge:  string(edgeID),
			RouteChanged: routeChanged,
		})
	}
}

// handleDeleteHazard serves DELETE /api/hazard/{edge_id} — an explicit
// admin/dashboard action to clear a previously blocked edge.
func handleDeleteHazard(store *state.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		edgeIDRaw := r.PathValue("edge_id")
		if edgeIDRaw == "" {
			writeError(w, http.StatusBadRequest, "edge_id is required")
			return
		}

		edgeID, routeChanged, err := store.UnblockEdge(edgeIDRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, hazardUnblockedResponse{
			UnblockedEdge: string(edgeID),
			RouteChanged:  routeChanged,
		})
	}
}
