package graph

import "testing"

func TestNormalizeEdgeID(t *testing.T) {
	cases := []struct {
		raw  string
		want EdgeID
	}{
		{"N2-N4", "N2-N4"},
		{"N4-N2", "N2-N4"},
		{"N1-N3", "N1-N3"},
		{"N3-N1", "N1-N3"},
	}
	for _, c := range cases {
		got, err := NormalizeEdgeID(c.raw)
		if err != nil {
			t.Fatalf("NormalizeEdgeID(%q) unexpected error: %v", c.raw, err)
		}
		if got != c.want {
			t.Errorf("NormalizeEdgeID(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestNormalizeEdgeIDInvalid(t *testing.T) {
	invalid := []string{"", "N1", "-N1", "N1-", "N1N2"}
	for _, raw := range invalid {
		if _, err := NormalizeEdgeID(raw); err == nil {
			t.Errorf("NormalizeEdgeID(%q) expected error, got nil", raw)
		}
	}
}

func TestCanonicalEdgeIDSymmetric(t *testing.T) {
	if CanonicalEdgeID("N2", "N4") != CanonicalEdgeID("N4", "N2") {
		t.Errorf("CanonicalEdgeID not symmetric")
	}
}

func TestSeedTopology(t *testing.T) {
	g := Seed()

	nodes := g.Nodes()
	if len(nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(nodes))
	}

	edges := g.Edges()
	if len(edges) != 7 {
		t.Fatalf("expected 7 edges, got %d", len(edges))
	}

	n1, ok := g.Node("N1")
	if !ok || n1.Name != "Shaft Entrance" || n1.X != 0 || n1.Y != 50 {
		t.Errorf("N1 mismatch: %+v ok=%v", n1, ok)
	}

	exit, ok := g.Node("EXIT")
	if !ok || exit.Name != "Exit Ramp" || exit.X != 90 || exit.Y != 40 {
		t.Errorf("EXIT mismatch: %+v ok=%v", exit, ok)
	}

	e, ok := g.Edge("N2-N4")
	if !ok || e.Weight != 25 {
		t.Errorf("N2-N4 mismatch: %+v ok=%v", e, ok)
	}
}
