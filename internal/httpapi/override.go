package httpapi

import (
	"encoding/json"
	"net/http"

	"hazard-mine-poc/internal/state"
)

// overrideRequest is POST /api/override's body — the dashboard's manual
// drift-correction failsafe.
type overrideRequest struct {
	NodeID string `json:"node_id"`
}

// handlePostOverride serves POST /api/override: force-sets the cart's
// current node, resets distance-into-edge, and triggers a Dijkstra
// recompute.
func handlePostOverride(store *state.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req overrideRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.NodeID == "" {
			writeError(w, http.StatusBadRequest, "node_id is required")
			return
		}

		if err := store.Override(req.NodeID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, store.Snapshot())
	}
}
