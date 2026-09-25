package state

import (
	"time"

	"hazard-mine-poc/internal/graph"
)

// DashboardState is the full-snapshot payload pushed over the outbound
// dashboard WebSocket on every state change, and served (same shape) by
// GET /api/state.
type DashboardState struct {
	Timestamp           time.Time        `json:"timestamp"`
	CurrentNode         graph.NodeID     `json:"current_node"`
	HeadingDeg          float64          `json:"heading_deg"`
	CumulativeDistanceM float64          `json:"cumulative_distance_m"`
	CurrentEdge         *CurrentEdgeInfo `json:"current_edge"`
	Route               RouteInfo        `json:"route"`
	BlockedEdges        []graph.EdgeID   `json:"blocked_edges"`
	Stall               StallInfo        `json:"stall"`
	Telemetry           TelemetryInfo    `json:"telemetry"`
	LastHazard          *HazardInfo      `json:"last_hazard"`
	Arrived             bool             `json:"arrived"`
}

// CurrentEdgeInfo describes the edge the cart is currently traversing, or is
// nil once the cart has arrived.
type CurrentEdgeInfo struct {
	ID                graph.EdgeID `json:"id"`
	EdgeLengthM       float64      `json:"edge_length_m"`
	DistanceIntoEdgeM float64      `json:"distance_into_edge_m"`
	ProgressPct       float64      `json:"progress_pct"`
}

// RouteInfo describes the cart's active route.
type RouteInfo struct {
	Nodes          []graph.NodeID `json:"nodes"`
	Edges          []graph.EdgeID `json:"edges"`
	TotalDistanceM float64        `json:"total_distance_m"`
}

// StallInfo mirrors the debounced stall detector's current state.
type StallInfo struct {
	IsStalled               bool `json:"is_stalled"`
	ConsecutiveStallSamples int  `json:"consecutive_stall_samples"`
}

// TelemetryInfo carries the most recent raw telemetry fields not otherwise
// derived (ultrasonic reading, motor PWM, last update timestamp).
type TelemetryInfo struct {
	UltrasonicCM float64 `json:"ultrasonic_cm"`
	MotorPWM     int     `json:"motor_pwm"`
	LastUpdateTS int64   `json:"last_update_ts"`
}

// HazardInfo is the JSON-serializable view of the most recent HazardRecord.
type HazardInfo struct {
	EdgeID graph.EdgeID `json:"edge_id"`
	Type   string       `json:"type"`
	Source HazardSource `json:"source"`
	At     time.Time    `json:"at"`
}
