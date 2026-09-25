// Package graph implements the hardcoded 5-node mine route graph: node/edge
// types, ID canonicalization, and adjacency bookkeeping. Dijkstra routing
// lives in dijkstra.go.
package graph

import (
	"fmt"
	"sort"
	"sync"
)

// NodeID identifies a graph node (e.g. "N1", "EXIT").
type NodeID string

// EdgeID identifies a graph edge in canonical form "A-B", where A < B
// lexicographically. Always construct via CanonicalEdgeID/NormalizeEdgeID.
type EdgeID string

// Node is a graph vertex. Name/X/Y are presentation metadata only — never
// used in routing math.
type Node struct {
	ID   NodeID
	Name string
	X, Y float64
}

// Edge is an undirected, weighted graph edge that can be blocked/unblocked.
type Edge struct {
	ID      EdgeID
	A, B    NodeID
	Weight  float64
	Blocked bool
}

// Graph is the hardcoded 5-node mine route graph. Safe for concurrent use;
// all exported methods take the internal lock.
type Graph struct {
	mu    sync.RWMutex
	nodes map[NodeID]*Node
	edges map[EdgeID]*Edge
	adj   map[NodeID][]EdgeID
}

// CanonicalEdgeID orders a and b by (length, lexical) and joins them with
// "-", so CanonicalEdgeID("N2","N4") == CanonicalEdgeID("N4","N2") ==
// "N2-N4". Ordering by length first (before falling back to plain lexical
// comparison) keeps short numbered IDs ("N1".."N4") ahead of longer named
// terminal IDs ("EXIT"), matching the topology's documented edge IDs
// ("N3-EXIT", "N4-EXIT") rather than a pure byte-lexical sort, which would
// put "EXIT" first (since 'E' < 'N').
func CanonicalEdgeID(a, b NodeID) EdgeID {
	sa, sb := string(a), string(b)
	if edgeIDLess(sa, sb) {
		return EdgeID(sa + "-" + sb)
	}
	return EdgeID(sb + "-" + sa)
}

// edgeIDLess reports whether sa should sort before sb when building a
// canonical edge ID: shorter strings first, then lexical order.
func edgeIDLess(sa, sb string) bool {
	if len(sa) != len(sb) {
		return len(sa) < len(sb)
	}
	return sa <= sb
}

// NormalizeEdgeID parses a raw "A-B" (or "B-A") string into its canonical
// EdgeID form. It splits on the first "-" only, so node IDs themselves must
// not contain "-".
func NormalizeEdgeID(raw string) (EdgeID, error) {
	idx := -1
	for i := 0; i < len(raw); i++ {
		if raw[i] == '-' {
			idx = i
			break
		}
	}
	if idx <= 0 || idx == len(raw)-1 {
		return "", fmt.Errorf("graph: invalid edge id %q", raw)
	}
	a := NodeID(raw[:idx])
	b := NodeID(raw[idx+1:])
	return CanonicalEdgeID(a, b), nil
}

// newGraph constructs an empty Graph ready for AddNode/AddEdge calls.
func newGraph() *Graph {
	return &Graph{
		nodes: make(map[NodeID]*Node),
		edges: make(map[EdgeID]*Edge),
		adj:   make(map[NodeID][]EdgeID),
	}
}

func (g *Graph) addNode(n Node) {
	nn := n
	g.nodes[n.ID] = &nn
}

func (g *Graph) addEdge(a, b NodeID, weight float64) EdgeID {
	id := CanonicalEdgeID(a, b)
	g.edges[id] = &Edge{ID: id, A: a, B: b, Weight: weight}
	g.adj[a] = append(g.adj[a], id)
	g.adj[b] = append(g.adj[b], id)
	return id
}

// Seed builds the hardcoded 5-node placeholder mine topology described in
// the project spec: N1 Shaft Entrance, N2 Checkpoint Alpha, N3 Ventilation
// Junction, N4 Checkpoint Bravo, EXIT Exit Ramp, and the 7 weighted edges
// connecting them.
func Seed() *Graph {
	g := newGraph()

	g.addNode(Node{ID: "N1", Name: "Shaft Entrance", X: 0, Y: 50})
	g.addNode(Node{ID: "N2", Name: "Checkpoint Alpha", X: 30, Y: 80})
	g.addNode(Node{ID: "N3", Name: "Ventilation Junction", X: 30, Y: 20})
	g.addNode(Node{ID: "N4", Name: "Checkpoint Bravo", X: 60, Y: 60})
	g.addNode(Node{ID: "EXIT", Name: "Exit Ramp", X: 90, Y: 40})

	g.addEdge("N1", "N2", 20)
	g.addEdge("N1", "N3", 35)
	g.addEdge("N2", "N3", 15)
	g.addEdge("N2", "N4", 25)
	g.addEdge("N3", "N4", 12)
	g.addEdge("N3", "EXIT", 30)
	g.addEdge("N4", "EXIT", 10)

	return g
}

// Node returns a copy of the node with the given ID.
func (g *Graph) Node(id NodeID) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	if !ok {
		return Node{}, false
	}
	return *n, true
}

// Nodes returns a copy of all nodes, sorted by ID for deterministic output.
func (g *Graph) Nodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Edge returns a copy of the edge with the given canonical ID.
func (g *Graph) Edge(id EdgeID) (Edge, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.edges[id]
	if !ok {
		return Edge{}, false
	}
	return *e, true
}

// Edges returns a copy of all edges, sorted by ID for deterministic output.
func (g *Graph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Edge, 0, len(g.edges))
	for _, e := range g.edges {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// BlockEdge marks the given edge as blocked (impassable for routing).
func (g *Graph) BlockEdge(id EdgeID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.edges[id]
	if !ok {
		return fmt.Errorf("graph: unknown edge %q", id)
	}
	e.Blocked = true
	return nil
}

// UnblockEdge marks the given edge as passable again.
func (g *Graph) UnblockEdge(id EdgeID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.edges[id]
	if !ok {
		return fmt.Errorf("graph: unknown edge %q", id)
	}
	e.Blocked = false
	return nil
}

// BlockedEdges returns the IDs of all currently blocked edges, sorted.
func (g *Graph) BlockedEdges() []EdgeID {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []EdgeID
	for id, e := range g.edges {
		if e.Blocked {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
