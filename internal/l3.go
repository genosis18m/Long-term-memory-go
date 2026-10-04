// L3 hypergraph big methods of the composition root: view / import / update / delete.

package internal

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/graph"
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
	"github.com/genosis18m/Long-term-memory-go/internal/scene"
)

func (db *DB) GetL3(agentID uint64, id string) (*L3Graph, error) {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	return db.getL3Graph(id)
}

// getL3Graph reads one graph slot and renders the full view.
func (db *DB) getL3Graph(id string) (*L3Graph, error) {
	slotID, err := parseID("l3", id)
	if err != nil {
		return nil, err
	}
	slot, err := repo.ReadSharedGraphL3(db.engine, slotID)
	if err != nil {
		return nil, err
	}
	return db.graphView(slot)
}

// graphView assembles the host-facing graph around an already-read slot.
func (db *DB) graphView(slot *core.HypergraphSlot) (*L3Graph, error) {
	nodes, err := repo.ListNodeL3(db.engine, core.SharedPoolAgentID, slot.IDHash)
	if err != nil {
		return nil, err
	}
	edges, err := repo.ListEdgeL3(db.engine, core.SharedPoolAgentID, slot.IDHash)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []core.HypergraphNode{}
	}
	if edges == nil {
		edges = []core.HypergraphEdge{}
	}
	return &L3Graph{Slot: *slot, Nodes: nodes, Edges: edges}, nil
}

// ListL3 lists every graph of the file-wide pool, sorted by id: the scan under it is a hash map.
func (db *DB) ListL3(agentID uint64) ([]core.HypergraphSlot, error) {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	all, err := core.CollectAllGraphSlots(db.engine, core.SharedPoolAgentID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(all, func(a, b core.HypergraphSlot) int {
		return cmp.Compare(a.IDHash, b.IDHash)
	})
	if all == nil {
		return []core.HypergraphSlot{}, nil
	}
	return all, nil
}

// ImportL3 batch-imports knowledge nodes.
func (db *DB) ImportL3(agentID uint64, items []L3ImportItem, mode L3ImportMode) (*L3ImportResult, error) {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	if len(items) == 0 {
		return nil, common.NewError(common.ErrInvalidQuery, "import: no items")
	}
	for i := range items {
		if items[i].Title == "" {
			return nil, common.NewError(common.ErrInvalidQuery,
				fmt.Sprintf("import: item %d has no title", i))
		}
		if items[i].Domain == "" {
			return nil, common.NewError(common.ErrInvalidQuery,
				fmt.Sprintf("import: item %q has no domain", items[i].Title))
		}
	}
	batch, err := graph.NewImportBatch(db.engine, core.SharedPoolAgentID, mode)
	if err != nil {
		return nil, common.NewError(common.CodeOf(err), "import", err)
	}
	for i := range items {
		if err := batch.ImportNode(&items[i]); err != nil {
			batch.Result().Errors = append(batch.Result().Errors, fmt.Sprintf("%s: %v", items[i].Title, err))
			continue
		}
	}
	// Relations for every item, including one whose node was skipped: edges are deduped by their sorted
	// members plus kind, so re-declaring one is a no-op.
	for i := range items {
		batch.ImportRelations(&items[i])
	}
	result := batch.Result()
	result.GraphIDs = batch.GraphIDs()
	if err := batch.StampChanged(); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	return result, nil
}

// UpdateL3 renames a graph.
func (db *DB) UpdateL3(agentID uint64, id, name string) (*L3Graph, error) {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	graphHash, err := common.ParseID(id)
	if err != nil {
		return nil, common.NewError(common.ErrInvalidQuery, "parse l3 id", err)
	}
	if name == "" {
		return nil, common.NewError(common.ErrInvalidQuery,
			"a graph label is how ImportL3 finds the graph, so an empty one names nothing", nil)
	}
	moves, err := graph.CheckRename(db.engine, core.SharedPoolAgentID, graphHash, name)
	if err != nil {
		return nil, err
	}
	if !moves {
		return db.getL3Graph(id)
	}
	slot, err := repo.UpdateGraphL3(db.engine, core.SharedPoolAgentID, graphHash, &name)
	if err != nil {
		return nil, err
	}
	return db.graphView(slot)
}

// DeleteL3 cascades: deletes the graph with all its nodes and edges from the shared L3 domain, then.
func (db *DB) DeleteL3(agentID uint64, id string) error {
	ac, err := db.lockSharedPool(agentID)
	if err != nil {
		return err
	}
	slotID, err := parseID("l3", id)
	if err != nil {
		ac.Mu.Unlock()
		return err
	}
	slot, err := repo.ReadSharedGraphL3(db.engine, slotID)
	if err != nil {
		ac.Mu.Unlock()
		return err
	}
	graphHash := slot.IDHash
	err = repo.DeleteGraphL3(db.engine, core.SharedPoolAgentID, graphHash)
	ac.Mu.Unlock()
	if err != nil {
		return err
	}
	return db.detachGraphAnchors(graphHash)
}

// detachGraphAnchors clears every scene anchor naming graphHash across the default domain and all
// registered tenants.
func (db *DB) detachGraphAnchors(graphHash uint64) error {
	db.agentsMu.Lock()
	targets := make([]uint64, 0, len(db.idToName)+1)
	targets = append(targets, core.DefaultAgentID)
	for id := range db.idToName {
		targets = append(targets, id)
	}
	db.agentsMu.Unlock()
	var errs []error
	for _, id := range targets {
		ac, err := db.lockAgent(id)
		if err != nil {
			// The domain is gone, being deleted or the DB is closing: its
			// records are gone or going, so there is no anchor left to clear.
			continue
		}
		if err := scene.DetachGraph(db.engine, id, graphHash); err != nil {
			errs = append(errs, err)
		}
		ac.Mu.Unlock()
	}
	return errors.Join(errs...)
}
