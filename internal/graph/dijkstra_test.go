package graph

import (
	"reflect"
	"testing"
)

func TestShortestPathDefault(t *testing.T) {
	g := Seed()
	route, err := g.ShortestPath("N1", "EXIT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.TotalWeight != 55 {
		t.Errorf("TotalWeight = %v, want 55", route.TotalWeight)
	}
	wantNodes := []NodeID{"N1", "N2", "N4", "EXIT"}
	if !reflect.DeepEqual(route.Nodes, wantNodes) {
		t.Errorf("Nodes = %v, want %v", route.Nodes, wantNodes)
	}
	wantEdges := []EdgeID{"N1-N2", "N2-N4", "N4-EXIT"}
	if !reflect.DeepEqual(route.Edges, wantEdges) {
		t.Errorf("Edges = %v, want %v", route.Edges, wantEdges)
	}
}

func TestShortestPathRerouteOnBlock(t *testing.T) {
	g := Seed()
	if err := g.BlockEdge("N2-N4"); err != nil {
		t.Fatalf("BlockEdge: %v", err)
	}
	route, err := g.ShortestPath("N2", "EXIT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.TotalWeight != 37 {
		t.Errorf("TotalWeight = %v, want 37", route.TotalWeight)
	}
	wantNodes := []NodeID{"N2", "N3", "N4", "EXIT"}
	if !reflect.DeepEqual(route.Nodes, wantNodes) {
		t.Errorf("Nodes = %v, want %v", route.Nodes, wantNodes)
	}
}

func TestShortestPathUnreachable(t *testing.T) {
	g := Seed()
	// Block every edge touching N1 to isolate it.
	for _, id := range []EdgeID{"N1-N2", "N1-N3"} {
		if err := g.BlockEdge(id); err != nil {
			t.Fatalf("BlockEdge(%v): %v", id, err)
		}
	}
	if _, err := g.ShortestPath("N1", "EXIT"); err == nil {
		t.Errorf("expected error for unreachable destination, got nil")
	}
}

func TestShortestPathUnknownNode(t *testing.T) {
	g := Seed()
	if _, err := g.ShortestPath("N1", "BOGUS"); err == nil {
		t.Errorf("expected error for unknown destination node")
	}
	if _, err := g.ShortestPath("BOGUS", "EXIT"); err == nil {
		t.Errorf("expected error for unknown start node")
	}
}

func TestShortestPathNormalizedEdgeIDsMatch(t *testing.T) {
	g1 := Seed()
	g2 := Seed()
	if err := g1.BlockEdge("N2-N4"); err != nil {
		t.Fatal(err)
	}
	id, err := NormalizeEdgeID("N4-N2")
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.BlockEdge(id); err != nil {
		t.Fatal(err)
	}
	r1, err1 := g1.ShortestPath("N2", "EXIT")
	r2, err2 := g2.ShortestPath("N2", "EXIT")
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("routes differ: %+v vs %+v", r1, r2)
	}
}

func TestUnblockEdgeRestoresDefault(t *testing.T) {
	g := Seed()
	if err := g.BlockEdge("N2-N4"); err != nil {
		t.Fatal(err)
	}
	if err := g.UnblockEdge("N2-N4"); err != nil {
		t.Fatal(err)
	}
	route, err := g.ShortestPath("N1", "EXIT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.TotalWeight != 55 {
		t.Errorf("TotalWeight = %v, want 55", route.TotalWeight)
	}
}

func TestBlockedEdges(t *testing.T) {
	g := Seed()
	if len(g.BlockedEdges()) != 0 {
		t.Errorf("expected no blocked edges initially")
	}
	if err := g.BlockEdge("N2-N4"); err != nil {
		t.Fatal(err)
	}
	blocked := g.BlockedEdges()
	if len(blocked) != 1 || blocked[0] != "N2-N4" {
		t.Errorf("BlockedEdges = %v, want [N2-N4]", blocked)
	}
}
