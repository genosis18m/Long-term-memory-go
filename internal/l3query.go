// L3 query big methods of the composition root: node lookup and BFS subgraph.

package internal

import (
	"maps"
	"slices"
	"strings"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/graph"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// QueryL3Nodes reads one graph's nodes through every condition the query names; the conditions AND
// together, and an unset condition does not filter.
func (db *DB) QueryL3Nodes(agentID uint64, q L3NodeQuery) ([]core.HypergraphNode, error) {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	if q.GraphID == "" {
		return nil, common.NewError(common.ErrInvalidQuery, "graph_id is required")
	}
	slotID, err := parseID("l3", q.GraphID)
	if err != nil {
		return nil, err
	}
	slot, err := repo.ReadSharedGraphL3(db.engine, slotID)
	if err != nil {
		return nil, err
	}
	graphHash := slot.IDHash
	filter, err := nodeFilter(q)
	if err != nil {
		return nil, err
	}
	nodes, err := repo.ListNodeL3(db.engine, core.SharedPoolAgentID, graphHash)
	if err != nil {
		return nil, err
	}
	out := make([]core.HypergraphNode, 0)
	for _, n := range nodes {
		if filter.Matches(n) {
			out = append(out, n)
		}
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// nodeFilter builds the graph filter from the query; a node id that does not parse is refused rather
// than dropped.
func nodeFilter(q L3NodeQuery) (graph.NodeFilter, error) {
	f := graph.NodeFilter{Keyword: strings.ToLower(q.Keyword), NodeType: q.NodeType}
	if len(q.IDs) > 0 {
		f.IDs = make(map[uint64]struct{}, len(q.IDs))
		for _, id := range q.IDs {
			idHash, err := common.ParseID(id)
			if err != nil {
				return graph.NodeFilter{}, common.NewError(common.ErrInvalidQuery, "parse node id", err)
			}
			f.IDs[idHash] = struct{}{}
		}
	}
	return f, nil
}

// QueryL3Subgraph BFS from startNodeID up to maxDepth hops; edgeKinds restricts reachable edges.
func (db *DB) QueryL3Subgraph(agentID uint64, graphID, startNodeID string, maxDepth int, edgeKinds []core.GraphEdgeKind) (*L3Subgraph, error) {
	for _, kind := range edgeKinds {
		if !kind.Valid() {
			return nil, common.NewError(common.ErrInvalidQuery, "unknown edge kind")
		}
	}
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	graphHash, err := parseID("graph", graphID)
	if err != nil {
		return nil, err
	}
	startHash, err := parseID("start node", startNodeID)
	if err != nil {
		return nil, err
	}
	if err := graph.CheckSubgraphStart(db.engine, core.SharedPoolAgentID, graphHash, startHash); err != nil {
		return nil, err
	}

	// Adjacency: all graph edges (filtered by edgeKinds), hyperedge nodeIDs fully connected.
	adj, edges, err := graph.SubgraphAdjacency(db.engine, core.SharedPoolAgentID, graphHash, edgeKinds)
	if err != nil {
		return nil, err
	}

	visited := graph.BfsWithinDepth(startHash, adj, maxDepth)

	nodes := make([]core.HypergraphNode, 0, len(visited))
	for _, h := range slices.Sorted(maps.Keys(visited)) {
		// A visited id is one an edge named, and an edge is only written over nodes the import that created it
		// had in hand.
		n, err := core.ReadHypergraphNode(db.engine, core.SharedPoolAgentID, h)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	subEdges := make([]core.HypergraphEdge, 0, len(edges))
	for _, e := range edges {
		if graph.AllNodesVisited(e.NodeIDs, visited) {
			subEdges = append(subEdges, e)
		}
	}
	return &L3Subgraph{Nodes: nodes, Edges: subEdges}, nil
}
