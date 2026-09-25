package graph

import (
	"container/heap"
	"fmt"
)

// Route is a computed shortest path: the node sequence, the edge sequence
// between consecutive nodes, and the total weight.
type Route struct {
	Nodes       []NodeID
	Edges       []EdgeID
	TotalWeight float64
}

// pqItem is one entry in the Dijkstra priority queue.
type pqItem struct {
	node NodeID
	dist float64
}

// priorityQueue is a min-heap of pqItem ordered by dist.
type priorityQueue []pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].dist < pq[j].dist }
func (pq priorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x interface{}) { *pq = append(*pq, x.(pqItem)) }
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

// ShortestPath computes the minimum-weight path from `from` to `to` using a
// heap-based Dijkstra, skipping blocked edges entirely (not up-weighting
// them). Returns an error if `to` is unreachable given the current blocked
// set, or if `from`/`to` are unknown nodes.
func (g *Graph) ShortestPath(from, to NodeID) (Route, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[from]; !ok {
		return Route{}, fmt.Errorf("graph: unknown start node %q", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return Route{}, fmt.Errorf("graph: unknown destination node %q", to)
	}

	dist := make(map[NodeID]float64, len(g.nodes))
	prevNode := make(map[NodeID]NodeID, len(g.nodes))
	prevEdge := make(map[NodeID]EdgeID, len(g.nodes))
	visited := make(map[NodeID]bool, len(g.nodes))

	for id := range g.nodes {
		dist[id] = -1 // -1 == not yet reached (avoids importing math for +Inf noise)
	}
	dist[from] = 0

	pq := &priorityQueue{{node: from, dist: 0}}
	heap.Init(pq)

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(pqItem)
		if visited[cur.node] {
			continue
		}
		visited[cur.node] = true

		if cur.node == to {
			break
		}

		for _, eid := range g.adj[cur.node] {
			e := g.edges[eid]
			if e.Blocked {
				continue
			}
			var neighbor NodeID
			if e.A == cur.node {
				neighbor = e.B
			} else {
				neighbor = e.A
			}
			if visited[neighbor] {
				continue
			}
			nd := dist[cur.node] + e.Weight
			if dist[neighbor] == -1 || nd < dist[neighbor] {
				dist[neighbor] = nd
				prevNode[neighbor] = cur.node
				prevEdge[neighbor] = eid
				heap.Push(pq, pqItem{node: neighbor, dist: nd})
			}
		}
	}

	if dist[to] == -1 {
		return Route{}, fmt.Errorf("graph: no path from %q to %q", from, to)
	}

	// Walk back from `to` to `from` via prevNode/prevEdge, then reverse.
	var nodesRev []NodeID
	var edgesRev []EdgeID
	n := to
	nodesRev = append(nodesRev, n)
	for n != from {
		edgesRev = append(edgesRev, prevEdge[n])
		n = prevNode[n]
		nodesRev = append(nodesRev, n)
	}

	nodes := make([]NodeID, len(nodesRev))
	for i, v := range nodesRev {
		nodes[len(nodesRev)-1-i] = v
	}
	edges := make([]EdgeID, len(edgesRev))
	for i, v := range edgesRev {
		edges[len(edgesRev)-1-i] = v
	}

	return Route{Nodes: nodes, Edges: edges, TotalWeight: dist[to]}, nil
}
