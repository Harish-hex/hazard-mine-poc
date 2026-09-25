// Package state owns the single mutex-guarded Store that ties together the
// route graph, odometry integration, stall detection, and the cart's
// position along its active route. All cross-cutting reads/writes for the
// backend happen through Store's methods.
package state

import (
	"errors"
	"fmt"
	"time"

	"hazard-mine-poc/internal/graph"
	"hazard-mine-poc/internal/odometry"

	"sync"
)

// Sentinel errors returned by ReportHazard so HTTP handlers can distinguish
// "cart already arrived" (409 Conflict) from other validation failures
// (400 Bad Request).
var (
	ErrArrived       = errors.New("state: cart has arrived, no current edge to block")
	ErrNoCurrentEdge = errors.New("state: no current edge")
)

// HazardSource identifies who/what reported a hazard.
type HazardSource string

const (
	SourceStall HazardSource = "stall"
	SourcePhone HazardSource = "phone"
)

// HazardEvent is the caller-supplied payload for a reported hazard (phone
// POST body, or synthesized by the stall detector).
type HazardEvent struct {
	Type     string `json:"type"`
	Severity string `json:"severity,omitempty"`
}

// HazardRecord is the backend's record of the most recent hazard.
type HazardRecord struct {
	EdgeID graph.EdgeID
	Type   string
	Source HazardSource
	At     time.Time
}

// CartState is the cart's current position and active route.
type CartState struct {
	CurrentNode       graph.NodeID
	DefaultPath       []graph.NodeID
	ActiveRoute       graph.Route
	ActiveEdgeIndex   int
	DistanceIntoEdgeM float64
	Arrived           bool
}

// Notifier receives a full DashboardState snapshot every time the cart's
// state changes (telemetry tick, hazard, unblock, override).
type Notifier interface {
	Broadcast(DashboardState)
}

// Store is the single mutex-guarded owner of graph + odometry + stall +
// cart state. Writers are the ~10Hz telemetry ingest path plus occasional
// HTTP handlers; critical sections stay tiny (5-node Dijkstra, map
// lookups). The lock is always released before calling notifier.Broadcast,
// so a slow/blocked dashboard subscriber can never stall telemetry ingest.
type Store struct {
	mu sync.RWMutex

	g    *graph.Graph
	dest graph.NodeID // routing destination, DefaultPath's last node

	odo   *odometry.Integrator
	stall odometry.StallDebouncer

	cart       CartState
	lastHazard *HazardRecord

	notifier Notifier
}

// NewStore builds a Store seeded with the given graph and default path. The
// cart starts at defaultPath[0] with its active route computed via Dijkstra
// from there to defaultPath's last node (equal to the default path itself
// when no edges are blocked).
func NewStore(g *graph.Graph, defaultPath []graph.NodeID, odoCfg odometry.Config, stallCfg odometry.StallDebouncer, n Notifier) *Store {
	if len(defaultPath) == 0 {
		panic("state: NewStore requires a non-empty defaultPath")
	}

	s := &Store{
		g:        g,
		dest:     defaultPath[len(defaultPath)-1],
		odo:      odometry.NewIntegrator(odoCfg),
		stall:    stallCfg,
		notifier: n,
	}

	start := defaultPath[0]
	route, err := g.ShortestPath(start, s.dest)
	if err != nil {
		// Seed topology guarantees a path at t=0; a failure here means the
		// graph/defaultPath were misconfigured by the caller.
		panic(fmt.Sprintf("state: NewStore initial route: %v", err))
	}

	s.cart = CartState{
		CurrentNode:       start,
		DefaultPath:       append([]graph.NodeID(nil), defaultPath...),
		ActiveRoute:       route,
		ActiveEdgeIndex:   0,
		DistanceIntoEdgeM: 0,
		Arrived:           len(route.Edges) == 0,
	}

	return s
}

// ApplyTelemetry folds one telemetry sample into odometry + edge-advancement
// + stall detection, and broadcasts the resulting snapshot to the notifier.
func (s *Store) ApplyTelemetry(t odometry.TelemetrySample) {
	var snap DashboardState
	var hazardRose bool

	s.mu.Lock()
	prevDist := s.odo.Snapshot().CumulativeDistanceM
	s.odo.Update(t)
	newDist := s.odo.Snapshot().CumulativeDistanceM
	delta := newDist - prevDist

	if !s.cart.Arrived && delta > 0 {
		s.cart.DistanceIntoEdgeM += delta
		s.advanceEdgesLocked()
	}

	stalled, rose := s.stall.Observe(t)
	_ = stalled
	hazardRose = rose

	if hazardRose && !s.cart.Arrived {
		// Best-effort: stall-triggered hazards never surface an error to a
		// caller (there is none), so any failure (e.g. Arrived race) is
		// simply ignored — the debouncer will re-fire if it recovers and
		// re-triggers.
		_, _, _ = s.reportHazardLocked(SourceStall, HazardEvent{Type: "stall"})
	}

	snap = s.snapshotLocked()
	s.mu.Unlock()

	s.notifier.Broadcast(snap)
}

// advanceEdgesLocked walks the cart forward along ActiveRoute, carrying
// overshoot distance into subsequent edges (so one large tick can cross more
// than one short edge), and marks the cart Arrived once the route is
// exhausted. Caller must hold s.mu.
func (s *Store) advanceEdgesLocked() {
	for {
		if s.cart.ActiveEdgeIndex >= len(s.cart.ActiveRoute.Edges) {
			s.cart.Arrived = true
			s.cart.DistanceIntoEdgeM = 0
			return
		}

		edgeID := s.cart.ActiveRoute.Edges[s.cart.ActiveEdgeIndex]
		edge, ok := s.g.Edge(edgeID)
		if !ok || s.cart.DistanceIntoEdgeM < edge.Weight {
			return
		}

		overshoot := s.cart.DistanceIntoEdgeM - edge.Weight
		s.cart.ActiveEdgeIndex++
		s.cart.CurrentNode = s.cart.ActiveRoute.Nodes[s.cart.ActiveEdgeIndex]
		s.cart.DistanceIntoEdgeM = overshoot
	}
}

// ReportHazard blocks the cart's current edge (the only target a hazard can
// ever apply to — never an arbitrary caller-picked edge), records it, and
// recomputes the route. Returns the blocked edge and whether the active
// route changed as a result.
func (s *Store) ReportHazard(src HazardSource, ev HazardEvent) (graph.EdgeID, bool, error) {
	s.mu.Lock()
	edgeID, routeChanged, err := s.reportHazardLocked(src, ev)
	var snap DashboardState
	if err == nil {
		snap = s.snapshotLocked()
	}
	s.mu.Unlock()

	if err == nil {
		s.notifier.Broadcast(snap)
	}
	return edgeID, routeChanged, err
}

// reportHazardLocked does the work of ReportHazard. Caller must hold s.mu.
//
// Blocking the edge and recomputing the route are not atomic as far as the
// underlying graph.Graph call is concerned, so on a recompute failure (the
// block would strand the cart with no path to the destination — e.g. both
// edges out of its current node end up blocked) this rolls the edge block
// and lastHazard back to their pre-call values before returning the error.
// Without the rollback, a rejected hazard report would still leave the
// graph's blocked-edge set and last_hazard mutated while ActiveRoute stayed
// stale (still routing the cart across the edge that's now flagged
// blocked) — an inconsistent snapshot that would keep advancing the cart
// across a "blocked" edge.
func (s *Store) reportHazardLocked(src HazardSource, ev HazardEvent) (graph.EdgeID, bool, error) {
	if s.cart.Arrived {
		return "", false, ErrArrived
	}
	if s.cart.ActiveEdgeIndex >= len(s.cart.ActiveRoute.Edges) {
		return "", false, ErrNoCurrentEdge
	}

	edgeID := s.cart.ActiveRoute.Edges[s.cart.ActiveEdgeIndex]
	if err := s.g.BlockEdge(edgeID); err != nil {
		return "", false, err
	}

	prevHazard := s.lastHazard
	s.lastHazard = &HazardRecord{
		EdgeID: edgeID,
		Type:   ev.Type,
		Source: src,
		At:     time.Now(),
	}

	routeChanged, err := s.recomputeRouteLocked()
	if err != nil {
		// Roll back: this hazard report is rejected outright, so it must
		// leave no visible trace (no newly-blocked edge, no lastHazard
		// update).
		_ = s.g.UnblockEdge(edgeID)
		s.lastHazard = prevHazard
		return edgeID, false, err
	}
	return edgeID, routeChanged, nil
}

// UnblockEdge clears a previously blocked edge (explicit admin/dashboard
// action) and recomputes the route.
func (s *Store) UnblockEdge(raw string) (graph.EdgeID, bool, error) {
	edgeID, err := graph.NormalizeEdgeID(raw)
	if err != nil {
		return "", false, err
	}

	s.mu.Lock()
	if err := s.g.UnblockEdge(edgeID); err != nil {
		s.mu.Unlock()
		return "", false, err
	}
	routeChanged, err := s.recomputeRouteLocked()
	var snap DashboardState
	if err == nil {
		snap = s.snapshotLocked()
	}
	s.mu.Unlock()

	if err == nil {
		s.notifier.Broadcast(snap)
	}
	return edgeID, routeChanged, err
}

// Override force-sets the cart's current node (dead-reckoning drift
// failsafe), resets distance-into-edge, and recomputes the route.
func (s *Store) Override(nodeID string) error {
	id := graph.NodeID(nodeID)

	s.mu.Lock()
	if _, ok := s.g.Node(id); !ok {
		s.mu.Unlock()
		return fmt.Errorf("state: unknown node %q", nodeID)
	}

	// Snapshot the pre-override cart state so a recompute failure (the
	// override target has no path to the destination given the current
	// blocked set) can be rolled back cleanly, rather than leaving
	// CurrentNode pointing at the override target while ActiveRoute/
	// ActiveEdgeIndex still reference the old (now unrelated) route.
	prevCart := s.cart

	s.cart.CurrentNode = id
	s.cart.DistanceIntoEdgeM = 0
	s.cart.Arrived = false

	_, err := s.recomputeRouteLocked()
	if err != nil {
		s.cart = prevCart
		s.mu.Unlock()
		return err
	}

	snap := s.snapshotLocked()
	s.mu.Unlock()

	s.notifier.Broadcast(snap)
	return nil
}

// recomputeRouteLocked always recomputes Dijkstra from CurrentNode to the
// store's destination, rather than first checking whether the change
// intersects ActiveRoute — simpler, costs microseconds at 5 nodes, and is
// externally identical (the route only visibly changes when the shortest
// path actually changed). If the new current edge differs from the old one,
// DistanceIntoEdgeM resets to 0 (no attempt to model backtrack distance, an
// explicit simplification given only 1D cumulative distance is tracked).
// Caller must hold s.mu.
func (s *Store) recomputeRouteLocked() (routeChanged bool, err error) {
	oldEdge, hadOldEdge := s.currentEdgeIDLocked()

	newRoute, err := s.g.ShortestPath(s.cart.CurrentNode, s.dest)
	if err != nil {
		return false, err
	}

	routeChanged = !routesEqual(s.cart.ActiveRoute, newRoute)

	s.cart.ActiveRoute = newRoute
	s.cart.ActiveEdgeIndex = 0
	s.cart.Arrived = len(newRoute.Edges) == 0

	newEdge, hasNewEdge := s.currentEdgeIDLocked()
	if !hasNewEdge || !hadOldEdge || newEdge != oldEdge {
		s.cart.DistanceIntoEdgeM = 0
	}

	return routeChanged, nil
}

// currentEdgeIDLocked returns the edge the cart is currently on, if any.
// Caller must hold s.mu.
func (s *Store) currentEdgeIDLocked() (graph.EdgeID, bool) {
	if s.cart.ActiveEdgeIndex >= len(s.cart.ActiveRoute.Edges) {
		return "", false
	}
	return s.cart.ActiveRoute.Edges[s.cart.ActiveEdgeIndex], true
}

func routesEqual(a, b graph.Route) bool {
	if len(a.Nodes) != len(b.Nodes) || len(a.Edges) != len(b.Edges) {
		return false
	}
	for i := range a.Nodes {
		if a.Nodes[i] != b.Nodes[i] {
			return false
		}
	}
	for i := range a.Edges {
		if a.Edges[i] != b.Edges[i] {
			return false
		}
	}
	return a.TotalWeight == b.TotalWeight
}

// Snapshot returns the current DashboardState (same shape pushed over the
// dashboard WS and served by GET /api/state).
func (s *Store) Snapshot() DashboardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

// snapshotLocked builds a DashboardState from current fields. Caller must
// hold s.mu (read or write lock).
func (s *Store) snapshotLocked() DashboardState {
	odo := s.odo.Snapshot()

	var currentEdge *CurrentEdgeInfo
	if edgeID, ok := s.currentEdgeIDLocked(); ok {
		if e, ok := s.g.Edge(edgeID); ok {
			progress := 0.0
			if e.Weight > 0 {
				progress = (s.cart.DistanceIntoEdgeM / e.Weight) * 100
			}
			currentEdge = &CurrentEdgeInfo{
				ID:                edgeID,
				EdgeLengthM:       e.Weight,
				DistanceIntoEdgeM: s.cart.DistanceIntoEdgeM,
				ProgressPct:       progress,
			}
		}
	}

	blocked := s.g.BlockedEdges()
	if blocked == nil {
		blocked = []graph.EdgeID{}
	}

	var lastHazard *HazardInfo
	if s.lastHazard != nil {
		lastHazard = &HazardInfo{
			EdgeID: s.lastHazard.EdgeID,
			Type:   s.lastHazard.Type,
			Source: s.lastHazard.Source,
			At:     s.lastHazard.At,
		}
	}

	return DashboardState{
		Timestamp:           time.Now().UTC(),
		CurrentNode:         s.cart.CurrentNode,
		HeadingDeg:          odo.HeadingDeg,
		CumulativeDistanceM: odo.CumulativeDistanceM,
		CurrentEdge:         currentEdge,
		Route: RouteInfo{
			Nodes:          s.cart.ActiveRoute.Nodes,
			Edges:          s.cart.ActiveRoute.Edges,
			TotalDistanceM: s.cart.ActiveRoute.TotalWeight,
		},
		BlockedEdges: blocked,
		Stall: StallInfo{
			IsStalled:               s.stall.IsStalled,
			ConsecutiveStallSamples: s.stall.ConsecutiveSamples(),
		},
		Telemetry: TelemetryInfo{
			UltrasonicCM: odo.LastUltrasonicCM,
			MotorPWM:     odo.LastMotorPWM,
			LastUpdateTS: odo.LastTS,
		},
		LastHazard: lastHazard,
		Arrived:    s.cart.Arrived,
	}
}
