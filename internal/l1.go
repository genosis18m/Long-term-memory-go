// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// L1 big methods of the composition root: the read face of the scene hypergraph.

package internal

import (
	"cmp"
	"slices"

	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// ListL1 returns every scene node of the domain, sorted by id because the index underneath is a hash
// map.
func (db *DB) ListL1(agentID uint64) ([]core.SceneNode, error) {
	ac, err := db.lockAgent(agentID)
	if err != nil {
		return nil, err
	}
	defer ac.Mu.Unlock()
	nodes, err := core.CollectAllStrict[core.SceneNode](db.engine, agentID, core.RecL1SceneNode)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		return []core.SceneNode{}, nil
	}
	slices.SortFunc(nodes, func(a, b core.SceneNode) int {
		return cmp.Compare(a.IDHash, b.IDHash)
	})
	return nodes, nil
}
