// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package dream holds the consolidation pipeline's stage small methods.

package dream

import (
	"github.com/genosis18m/Long-term-memory-go/internal/repo"
	"github.com/genosis18m/Long-term-memory-go/internal/repo/core"
)

// SceneSet resolves the target scenes of one pass: a non-zero scene id, or every scene of the domain.
func SceneSet(engine *core.StorageEngine, agentID uint64, sceneID uint64) ([]uint64, error) {
	if sceneID != 0 {
		// Existence is part of the answer: a scene the caller names but that is gone
		// must fail the pass, not report a clean no-op.
		if _, err := core.ReadSceneSlot(engine, agentID, sceneID); err != nil {
			return nil, err
		}
		return []uint64{sceneID}, nil
	}
	scenes, err := repo.CollectAllScenesL2(engine, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(scenes))
	for _, s := range scenes {
		out = append(out, s.SceneID)
	}
	return out, nil
}
