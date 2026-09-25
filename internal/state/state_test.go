package state

import (
	"sync"
	"testing"

	"hazard-mine-poc/internal/graph"
	"hazard-mine-poc/internal/odometry"
)

// fakeNotifier records every broadcast snapshot; safe for concurrent use.
type fakeNotifier struct {
	mu    sync.Mutex
	count int
	last  DashboardState
}

func (f *fakeNotifier) Broadcast(d DashboardState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	f.last = d
}

func defaultTestStore(n Notifier) *Store {
	g := graph.Seed()
	defaultPath := []graph.NodeID{"N1", "N2", "N4", "EXIT"}
	odoCfg := odometry.Config{MetersPerTick: 1}
	stallCfg := odometry.StallDebouncer{PWMThreshold: 50, OnSamples: 3, OffSamples: 3, EncoderEpsilon: 1}
	return NewStore(g, defaultPath, odoCfg, stallCfg, n)
}

func TestNewStoreInitialSnapshot(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)
	snap := s.Snapshot()

	if snap.CurrentNode != "N1" {
		t.Errorf("CurrentNode = %v, want N1", snap.CurrentNode)
	}
	if snap.Route.TotalDistanceM != 55 {
		t.Errorf("Route.TotalDistanceM = %v, want 55", snap.Route.TotalDistanceM)
	}
	if snap.Arrived {
		t.Errorf("expected not arrived initially")
	}
	if snap.CurrentEdge == nil || snap.CurrentEdge.ID != "N1-N2" {
		t.Errorf("CurrentEdge = %+v, want N1-N2", snap.CurrentEdge)
	}
}

func TestApplyTelemetryAdvancesEdge(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	// N1-N2 weight is 20m; 15 ticks * 1 m/tick = 15m, not enough to advance.
	s.ApplyTelemetry(odometry.TelemetrySample{EncoderDelta: 15, TS: 100})
	snap := s.Snapshot()
	if snap.CurrentNode != "N1" {
		t.Errorf("CurrentNode = %v, want N1 (not yet advanced)", snap.CurrentNode)
	}
	if snap.CurrentEdge.DistanceIntoEdgeM != 15 {
		t.Errorf("DistanceIntoEdgeM = %v, want 15", snap.CurrentEdge.DistanceIntoEdgeM)
	}

	// 10 more ticks crosses the 20m threshold (25 total), carrying 5m overshoot.
	s.ApplyTelemetry(odometry.TelemetrySample{EncoderDelta: 10, TS: 200})
	snap = s.Snapshot()
	if snap.CurrentNode != "N2" {
		t.Errorf("CurrentNode = %v, want N2 after advancing", snap.CurrentNode)
	}
	if snap.CurrentEdge == nil || snap.CurrentEdge.ID != "N2-N4" {
		t.Fatalf("CurrentEdge = %+v, want N2-N4", snap.CurrentEdge)
	}
	if snap.CurrentEdge.DistanceIntoEdgeM != 5 {
		t.Errorf("DistanceIntoEdgeM overshoot = %v, want 5", snap.CurrentEdge.DistanceIntoEdgeM)
	}

	if n.count == 0 {
		t.Errorf("expected notifier to have been called")
	}
}

func TestApplyTelemetryMultiEdgePerTick(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	// One giant tick (200m) should cross N1-N2 (20), N2-N4 (25), N4-EXIT (10)
	// and land Arrived at EXIT, since total route is 55m.
	s.ApplyTelemetry(odometry.TelemetrySample{EncoderDelta: 200, TS: 100})
	snap := s.Snapshot()
	if !snap.Arrived {
		t.Fatalf("expected Arrived after overshooting entire route")
	}
	if snap.CurrentNode != "EXIT" {
		t.Errorf("CurrentNode = %v, want EXIT", snap.CurrentNode)
	}
	if snap.CurrentEdge != nil {
		t.Errorf("CurrentEdge = %+v, want nil once arrived", snap.CurrentEdge)
	}
}

func TestReportHazardBlocksCurrentEdgeAndReroutes(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	// Cart parked at N1, current edge is N1-N2 (default route's first edge).
	edgeID, routeChanged, err := s.ReportHazard(SourcePhone, HazardEvent{Type: "rockfall", Severity: "high"})
	if err != nil {
		t.Fatalf("ReportHazard: %v", err)
	}
	if edgeID != "N1-N2" {
		t.Errorf("blocked edge = %v, want N1-N2", edgeID)
	}
	if !routeChanged {
		t.Errorf("expected routeChanged=true")
	}

	snap := s.Snapshot()
	if len(snap.BlockedEdges) != 1 || snap.BlockedEdges[0] != "N1-N2" {
		t.Errorf("BlockedEdges = %v, want [N1-N2]", snap.BlockedEdges)
	}
	// Rerouted via N1-N3.
	if snap.CurrentEdge == nil || snap.CurrentEdge.ID != "N1-N3" {
		t.Fatalf("CurrentEdge = %+v, want N1-N3", snap.CurrentEdge)
	}
	if snap.LastHazard == nil || snap.LastHazard.Type != "rockfall" || snap.LastHazard.Source != SourcePhone {
		t.Errorf("LastHazard = %+v, unexpected", snap.LastHazard)
	}
}

func TestReportHazardMidRouteRerouteScenario(t *testing.T) {
	// This is the plan's canonical demo reroute scenario: advance the cart
	// to N2, block N2-N4 (on the active route), confirm reroute recomputes
	// from N2 landing on N2-N3-N4-EXIT (37m).
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	// Advance exactly to N2 (20m).
	s.ApplyTelemetry(odometry.TelemetrySample{EncoderDelta: 20, TS: 100})
	snap := s.Snapshot()
	if snap.CurrentNode != "N2" {
		t.Fatalf("expected CurrentNode N2 after 20m, got %v", snap.CurrentNode)
	}
	if snap.CurrentEdge == nil || snap.CurrentEdge.ID != "N2-N4" {
		t.Fatalf("expected current edge N2-N4, got %+v", snap.CurrentEdge)
	}

	edgeID, routeChanged, err := s.ReportHazard(SourcePhone, HazardEvent{Type: "rockfall"})
	if err != nil {
		t.Fatalf("ReportHazard: %v", err)
	}
	if edgeID != "N2-N4" {
		t.Errorf("blocked edge = %v, want N2-N4", edgeID)
	}
	if !routeChanged {
		t.Errorf("expected routeChanged=true")
	}

	snap = s.Snapshot()
	if snap.Route.TotalDistanceM != 37 {
		t.Errorf("Route.TotalDistanceM = %v, want 37", snap.Route.TotalDistanceM)
	}
	wantNodes := []graph.NodeID{"N2", "N3", "N4", "EXIT"}
	if len(snap.Route.Nodes) != len(wantNodes) {
		t.Fatalf("Route.Nodes = %v, want %v", snap.Route.Nodes, wantNodes)
	}
	for i, n := range wantNodes {
		if snap.Route.Nodes[i] != n {
			t.Errorf("Route.Nodes[%d] = %v, want %v", i, snap.Route.Nodes[i], n)
		}
	}
	if snap.CurrentEdge.DistanceIntoEdgeM != 0 {
		t.Errorf("DistanceIntoEdgeM after reroute = %v, want 0 (no backtrack modeled)", snap.CurrentEdge.DistanceIntoEdgeM)
	}
}

func TestUnblockEdgeClearsAndReroutes(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	if _, _, err := s.ReportHazard(SourcePhone, HazardEvent{Type: "rockfall"}); err != nil {
		t.Fatalf("ReportHazard: %v", err)
	}

	edgeID, _, err := s.UnblockEdge("N1-N2")
	if err != nil {
		t.Fatalf("UnblockEdge: %v", err)
	}
	if edgeID != "N1-N2" {
		t.Errorf("unblocked edge = %v, want N1-N2", edgeID)
	}

	snap := s.Snapshot()
	if len(snap.BlockedEdges) != 0 {
		t.Errorf("BlockedEdges = %v, want empty after unblock", snap.BlockedEdges)
	}
	if snap.CurrentEdge == nil || snap.CurrentEdge.ID != "N1-N2" {
		t.Errorf("expected route restored via N1-N2, got %+v", snap.CurrentEdge)
	}
}

func TestReportHazardWhenArrivedErrors(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)
	s.ApplyTelemetry(odometry.TelemetrySample{EncoderDelta: 1000, TS: 100})

	snap := s.Snapshot()
	if !snap.Arrived {
		t.Fatalf("expected arrived")
	}

	if _, _, err := s.ReportHazard(SourcePhone, HazardEvent{Type: "rockfall"}); err == nil {
		t.Errorf("expected error reporting hazard after arrival")
	}
}

func TestOverride(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	if err := s.Override("N3"); err != nil {
		t.Fatalf("Override: %v", err)
	}
	snap := s.Snapshot()
	if snap.CurrentNode != "N3" {
		t.Errorf("CurrentNode = %v, want N3", snap.CurrentNode)
	}
	if snap.CurrentEdge == nil || snap.CurrentEdge.DistanceIntoEdgeM != 0 {
		t.Errorf("expected DistanceIntoEdgeM reset to 0 after override")
	}
}

func TestOverrideUnknownNodeErrors(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)
	if err := s.Override("BOGUS"); err == nil {
		t.Errorf("expected error for unknown node")
	}
}

func TestStallTriggersHazardEndToEnd(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	for i := 0; i < 3; i++ {
		s.ApplyTelemetry(odometry.TelemetrySample{MotorPWM: 200, EncoderDelta: 0, TS: int64(100 * (i + 1))})
	}

	snap := s.Snapshot()
	if !snap.Stall.IsStalled {
		t.Fatalf("expected IsStalled true after sustained stall samples")
	}
	if snap.LastHazard == nil || snap.LastHazard.Source != SourceStall {
		t.Errorf("expected stall-sourced hazard, got %+v", snap.LastHazard)
	}
	if len(snap.BlockedEdges) != 1 || snap.BlockedEdges[0] != "N1-N2" {
		t.Errorf("BlockedEdges = %v, want [N1-N2]", snap.BlockedEdges)
	}
}

// TestConcurrentAccessRace hammers ApplyTelemetry/ReportHazard/Snapshot from
// multiple goroutines concurrently with a fake Notifier. Run with -race.
func TestConcurrentAccessRace(t *testing.T) {
	n := &fakeNotifier{}
	s := defaultTestStore(n)

	var wg sync.WaitGroup

	// Telemetry ingest goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s.ApplyTelemetry(odometry.TelemetrySample{
				EncoderDelta: int64(i % 3),
				GyroZ:        0.1,
				MotorPWM:     10,
				TS:           int64(i * 10),
			})
		}
	}()

	// Hazard-reporting goroutine (phone POSTs).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _, _ = s.ReportHazard(SourcePhone, HazardEvent{Type: "rockfall"})
		}
	}()

	// Unblock goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _, _ = s.UnblockEdge("N1-N2")
		}
	}()

	// Reader goroutines.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = s.Snapshot()
			}
		}()
	}

	wg.Wait()
}
